package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/auth"
	"github.com/pattaradol9/game24/server/internal/config"
	"github.com/pattaradol9/game24/server/internal/crypto"
	"github.com/pattaradol9/game24/server/internal/game"
	"github.com/pattaradol9/game24/server/internal/presence"
	"github.com/pattaradol9/game24/server/internal/progress"
	"github.com/pattaradol9/game24/server/internal/room"
	"github.com/pattaradol9/game24/server/internal/store"
)

func newTestAPI(t *testing.T) (*API, *chi.Mux) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cr, err := crypto.New(strings.Repeat("ab", 32)) // valid 64-hex-char key
	if err != nil {
		t.Fatal(err)
	}
	db.SetCrypter(cr)
	api := &API{
		Store:    db,
		Hub:      room.NewHub(nil),
		Cfg:      config.Config{Port: "0"},
		Presence: presence.NewBroker(),
	}
	r := chi.NewRouter()
	r.Mount("/api/v1", api.Routes())
	return api, r
}

func post(t *testing.T, mux *chi.Mux, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json %s: %v", res.Body.String(), err)
	}
	return res.Code, out
}

func get(t *testing.T, mux *chi.Mux, path, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	json.Unmarshal(res.Body.Bytes(), &out)
	return res.Code, out
}

func data(m map[string]any) map[string]any {
	d, _ := m["data"].(map[string]any)
	return d
}

func del(t *testing.T, mux *chi.Mux, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("DELETE", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json %s: %v", res.Body.String(), err)
	}
	return res.Code, out
}

func TestDeleteMe(t *testing.T) {
	api, mux := newTestAPI(t)

	// guests cannot self-delete: the account survives the attempt
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Doomed"}, "")
	guestToken := data(res)["token"].(string)
	if code, _ := del(t, mux, "/api/v1/me", map[string]string{"confirm": "DELETE"}, guestToken); code != 403 {
		t.Fatalf("guest delete = %d, want 403", code)
	}
	if code, _ := get(t, mux, "/api/v1/me", guestToken); code != 200 {
		t.Fatal("guest should survive a rejected delete")
	}

	// Google accounts delete through the same confirmation guard
	_, token, err := api.Store.GoogleLogin("sub-doomed", "doomed@example.com", "Doomed", "")
	if err != nil {
		t.Fatalf("google login: %v", err)
	}

	// without the exact confirmation the account survives
	if code, res := del(t, mux, "/api/v1/me", nil, token); code != 400 {
		t.Fatalf("unconfirmed delete = %d %v, want 400", code, res)
	}
	if code, res := del(t, mux, "/api/v1/me", map[string]string{"confirm": "delete"}, token); code != 400 {
		t.Fatalf("lowercase confirmation = %d %v, want 400", code, res)
	}
	if code, _ := get(t, mux, "/api/v1/me", token); code != 200 {
		t.Fatal("player should still exist after rejected deletes")
	}

	if code, res := del(t, mux, "/api/v1/me", map[string]string{"confirm": "DELETE"}, token); code != 200 || data(res)["deleted"] != true {
		t.Fatalf("delete me = %d %v, want 200 deleted", code, res)
	}
	// the session token dies with the account
	if code, _ := get(t, mux, "/api/v1/me", token); code != 401 {
		t.Fatalf("me after delete = %d, want 401", code)
	}
}

func TestGuestRoundFlow(t *testing.T) {
	api, mux := newTestAPI(t)
	_ = api

	// 1. guest signs in with a nickname
	code, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "TestNick"}, "")
	if code != 200 || res["success"] != true {
		t.Fatalf("create player: %d %v", code, res)
	}
	player := data(res)["player"].(map[string]any)
	token := data(res)["token"].(string)
	if player["nickname"] != "TestNick" || player["isGuest"] != true {
		t.Fatalf("player = %v", player)
	}

	// 2. deal a hand
	code, res = post(t, mux, "/api/v1/rounds", map[string]string{"mode": "queen"}, token)
	if code != 200 {
		t.Fatalf("create round: %d %v", code, res)
	}
	roundID := data(res)["roundId"].(string)
	numsRaw := data(res)["numbers"].([]any)
	numbers := make([]int, 4)
	for i, n := range numsRaw {
		numbers[i] = int(n.(float64))
	}
	hintQuota := int(data(res)["hintQuota"].(float64))
	if hintQuota != 1 { // bronze guest: 1 hint per hand
		t.Fatalf("hint quota = %v", hintQuota)
	}

	// 3. solve it
	sols := game.Solve(numbers)
	if len(sols) == 0 {
		t.Fatalf("unsolvable hand dealt: %v", numbers)
	}
	steps := []map[string]any{}
	for _, st := range sols[0].Trace {
		step := map[string]any{"op": st.Op}
		if st.Left.IsStep {
			step["left"] = map[string]any{"step": st.Left.Step, "isStep": true}
		} else {
			step["left"] = map[string]any{"card": st.Left.Card}
		}
		if st.Right.IsStep {
			step["right"] = map[string]any{"step": st.Right.Step, "isStep": true}
		} else {
			step["right"] = map[string]any{"card": st.Right.Card}
		}
		steps = append(steps, step)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/submit", map[string]any{"steps": steps}, token)
	if code != 200 {
		t.Fatalf("submit: %d %v", code, res)
	}
	if pts := data(res)["points"].(float64); pts <= 0 {
		t.Fatalf("points = %v", pts)
	}
	if expr, _ := data(res)["expr"].(string); expr == "" {
		t.Fatal("no expr returned")
	}

	// 4. guest never reaches the leaderboard
	code, res = get(t, mux, "/api/v1/leaderboard?mode=queen", "")
	if code != 200 {
		t.Fatalf("leaderboard: %d", code)
	}
	entries := data(res)["entries"].([]any)
	if len(entries) != 0 {
		t.Fatalf("guest leaked to leaderboard: %v", entries)
	}

	// 5. hint quota enforcement on a fresh round
	code, res = post(t, mux, "/api/v1/rounds", map[string]string{"mode": "jack"}, token)
	roundID = data(res)["roundId"].(string)
	_, res = post(t, mux, "/api/v1/rounds/"+roundID+"/hint", map[string]any{}, token)
	if res["success"] != true {
		t.Fatalf("first hint failed: %v", res)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/hint", map[string]any{}, token)
	if code != 403 {
		t.Fatalf("second hint should be 403, got %d", code)
	}
}

func TestRoomCreation(t *testing.T) {
	_, mux := newTestAPI(t)
	code, res := post(t, mux, "/api/v1/rooms", map[string]any{
		"mode": "queen", "rounds": 24, "hintQuota": 3, "regenQuota": 2,
	}, "")
	if code != 200 {
		t.Fatalf("create room: %d %v", code, res)
	}
	roomCode := data(res)["code"].(string)
	if len(roomCode) != 6 {
		t.Fatalf("code = %q", roomCode)
	}
	if data(res)["hostKey"] == "" {
		t.Fatal("missing host key")
	}

	code, res = get(t, mux, "/api/v1/rooms/"+roomCode, "")
	if code != 200 || data(res)["state"] != "lobby" {
		t.Fatalf("room info: %d %v", code, res)
	}

	if code, _ := get(t, mux, "/api/v1/rooms/ZZZZZZ", ""); code != 404 {
		t.Fatalf("missing room should 404, got %d", code)
	}

	// invalid config rejected
	if code, _ := post(t, mux, "/api/v1/rooms", map[string]any{"mode": "queen", "rounds": 10, "hintQuota": 1, "regenQuota": 1}, ""); code != 400 {
		t.Fatalf("invalid rounds accepted")
	}
}

func TestGoogleAuthDisabled(t *testing.T) {
	api, mux := newTestAPI(t)
	api.Google = auth.NewGoogleVerifier("unused")
	// verification will fail because the credential is garbage — that's fine,
	// we only assert the endpoint exists and rejects.
	code, _ := post(t, mux, "/api/v1/auth/google", map[string]string{"credential": "abc"}, "")
	if code != 401 {
		t.Fatalf("bad credential should 401, got %d", code)
	}
}

// One play session grants the full tier quota, shared across every hand of
// that session; a fresh session starts with the quota refilled.
func TestHintSessionQuota(t *testing.T) {
	api, mux := newTestAPI(t)

	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Hinter"}, "")
	if res["success"] != true {
		t.Fatalf("create player: %v", res)
	}
	token := data(res)["token"].(string)
	p, err := api.Store.PlayerByToken(token)
	if err != nil {
		t.Fatal(err)
	}
	// gold tier (level 40): quota = 1 base + 1 tier bonus = 2 per session
	if _, _, err := api.Store.AwardEXP(p.ID, "queen", progress.ExpForLevel(40), true); err != nil {
		t.Fatal(err)
	}

	// newRound creates a hand in the session and reports the hints remaining.
	newRound := func(session string) (roundID string, hintsLeft int) {
		code, res := post(t, mux, "/api/v1/rounds", map[string]any{"mode": "queen", "sessionId": session}, token)
		if code != 200 {
			t.Fatalf("create round: %d %v", code, res)
		}
		if got := int(data(res)["hintQuota"].(float64)); got != 2 {
			t.Fatalf("gold hint quota = %d, want 2", got)
		}
		return data(res)["roundId"].(string), int(data(res)["hintsLeft"].(float64))
	}
	hint := func(roundID string) int {
		code, _ := post(t, mux, "/api/v1/rounds/"+roundID+"/hint", map[string]any{}, token)
		return code
	}

	// session 1: the full quota is usable, spread over different hands
	r1, left := newRound("sess-1")
	if left != 2 {
		t.Fatalf("fresh session hintsLeft = %d, want 2", left)
	}
	if code := hint(r1); code != 200 {
		t.Fatalf("first hint: %d", code)
	}
	r2, left := newRound("sess-1")
	if left != 1 {
		t.Fatalf("hintsLeft after one use = %d, want 1", left)
	}
	if code := hint(r2); code != 200 {
		t.Fatalf("second hint: %d", code)
	}
	// quota spent: the next hand of the same session gets no hint
	r3, left := newRound("sess-1")
	if left != 0 {
		t.Fatalf("hintsLeft after two uses = %d, want 0", left)
	}
	if code := hint(r3); code != 403 {
		t.Fatalf("hint past quota = %d, want 403", code)
	}

	// session 2: a fresh visit refills the budget
	r4, left := newRound("sess-2")
	if left != 2 {
		t.Fatalf("new session hintsLeft = %d, want 2", left)
	}
	if code := hint(r4); code != 200 {
		t.Fatalf("hint in new session: %d", code)
	}
}
