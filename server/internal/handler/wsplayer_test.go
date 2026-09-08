package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialPlayerWS opens a player socket against the test mux and returns the
// connection. t.Cleanup closes it.
func dialPlayerWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws/player"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial player socket: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readMsg reads one envelope with a timeout so a silent server fails the
// test instead of hanging it.
func readMsg(t *testing.T, conn *websocket.Conn) (string, map[string]any) {
	t.Helper()
	type msg struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var m msg
	if err := conn.ReadJSON(&m); err != nil {
		t.Fatalf("read message: %v", err)
	}
	var body map[string]any
	if len(m.Data) > 0 {
		if err := json.Unmarshal(m.Data, &body); err != nil {
			t.Fatalf("bad payload %s: %v", m.Data, err)
		}
	}
	return m.Type, body
}

// authPlayerWS performs the auth handshake and returns the snapshot.
func authPlayerWS(t *testing.T, conn *websocket.Conn, token string) map[string]any {
	t.Helper()
	if err := conn.WriteJSON(map[string]any{"type": "auth", "data": map[string]string{"token": token}}); err != nil {
		t.Fatalf("auth write: %v", err)
	}
	typ, body := readMsg(t, conn)
	if typ != "player" {
		t.Fatalf("post-auth message = %q, want player", typ)
	}
	return body["player"].(map[string]any)
}

func TestPlayerWSRejectsBadToken(t *testing.T) {
	_, mux := newTestAPI(t)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	conn := dialPlayerWS(t, srv)
	if err := conn.WriteJSON(map[string]any{"type": "auth", "data": map[string]string{"token": "nope"}}); err != nil {
		t.Fatal(err)
	}
	if typ, _ := readMsg(t, conn); typ != "error" {
		t.Fatalf("bad-token message = %q, want error", typ)
	}
	// the server hangs up right after the rejection
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("server should close after a rejected auth")
	}
}

// TestPlayerWSPushesAdminAdjustments walks a real socket through the admin
// paths that must land without a refresh: the opening snapshot, a live coin
// override, the ban/unban events and the deletion notice.
func TestPlayerWSPushesAdminAdjustments(t *testing.T) {
	_, mux, adminToken := newAdminTestAPI(t)

	// the administered player
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Sockety"}, "")
	token := data(res)["token"].(string)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	conn := dialPlayerWS(t, srv)
	player := authPlayerWS(t, conn, token)
	if player["nickname"] != "Sockety" {
		t.Fatalf("snapshot player = %v", player)
	}
	targetID := player["id"].(string)

	// a live coin override arrives as a push, no refresh needed
	if code, _ := post(t, mux, "/api/v1/admin/players/"+targetID+"/coins", map[string]any{"value": 777}, adminToken); code != 200 {
		t.Fatalf("admin coins = %d, want 200", code)
	}
	typ, body := readMsg(t, conn)
	if typ != "player" {
		t.Fatalf("push message = %q, want player", typ)
	}
	if got := body["player"].(map[string]any)["totalCoins"].(float64); got != 777 {
		t.Fatalf("pushed totalCoins = %v, want 777", got)
	}

	// a ban lands as its own terminal event, reason included
	if code, _ := doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+targetID,
		map[string]any{"banned": true, "banReason": "cheating"}, adminToken); code != 200 {
		t.Fatalf("admin ban = %d, want 200", code)
	}
	if typ, body = readMsg(t, conn); typ != "banned" {
		t.Fatalf("push message = %q, want banned", typ)
	}
	if body["reason"] != "cheating" {
		t.Fatalf("banned push = %v, want reason", body)
	}

	// lifting the ban resumes the plain profile pushes
	if code, _ := doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+targetID,
		map[string]any{"banned": false}, adminToken); code != 200 {
		t.Fatalf("admin unban = %d, want 200", code)
	}
	if typ, _ = readMsg(t, conn); typ != "player" {
		t.Fatalf("push message = %q, want player", typ)
	}

	// deletion reaches the socket as its own event
	if code, _ := del(t, mux, "/api/v1/admin/players/"+targetID, nil, adminToken); code != 200 {
		t.Fatalf("admin delete = %d, want 200", code)
	}
	if typ, _ = readMsg(t, conn); typ != "deleted" {
		t.Fatalf("push message = %q, want deleted", typ)
	}
}
