// Player socket: a per-player websocket (/api/v1/ws/player) that streams
// live profile changes — admin EXP/coin/tier adjustments, achievement
// grants, renames, bans, deletions — to every open tab of that player,
// mirroring the room socket's envelope ({type, data}), timings and origin
// policy. The first client message must be {"type":"auth","data":{"token":
// ...}}; the server answers with a full profile snapshot, then relays
// presence pushes ("player" / "banned" / "deleted") until either side hangs
// up.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/pattaradol9/game24/server/internal/presence"
	"github.com/pattaradol9/game24/server/internal/store"
	"github.com/pattaradol9/game24/server/internal/ws"
)

// socket timings mirror the room socket's (internal/ws).
const (
	playerWriteWait  = 10 * time.Second
	playerPongWait   = 60 * time.Second
	playerPingPeriod = 45 * time.Second
	playerSendBuffer = 16
	// how long the client has to send its auth message after connecting
	playerAuthWait = 10 * time.Second
)

// playerConn is one upgraded socket: the send channel feeds the writer pump;
// a slow consumer is kicked rather than allowed to block the relay.
type playerConn struct {
	ws   *websocket.Conn
	send chan []byte
	once sync.Once
}

func (c *playerConn) deliver(raw []byte) {
	select {
	case c.send <- raw:
	default:
		c.once.Do(func() { go c.ws.Close() })
	}
}

func (c *playerConn) closeConn() { c.once.Do(func() { go c.ws.Close() }) }

func (c *playerConn) writePump() {
	ping := time.NewTicker(playerPingPeriod)
	defer ping.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(playerWriteWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			c.ws.SetWriteDeadline(time.Now().Add(playerWriteWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// playerWS returns the upgrade handler mounted at /api/v1/ws/player.
func (a *API) playerWS() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.Presence == nil {
			http.Error(w, "event stream unavailable", http.StatusServiceUnavailable)
			return
		}
		upgrader := websocket.Upgrader{
			CheckOrigin: func(req *http.Request) bool {
				return ws.CheckOrigin(req, a.Cfg.CORSOrigins)
			},
		}
		raw, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := &playerConn{ws: raw, send: make(chan []byte, playerSendBuffer)}
		go c.writePump()

		var (
			handle     presence.Sub
			subscribed bool
			playerID   string
			closeOnce  sync.Once
		)
		shutdown := func() {
			closeOnce.Do(func() {
				c.closeConn()
				if subscribed {
					a.Presence.Unsubscribe(playerID, handle)
				}
			})
		}
		defer shutdown()

		send := func(typ string, data any) bool {
			raw, err := json.Marshal(map[string]any{"type": typ, "data": data})
			if err != nil {
				return false
			}
			c.deliver(raw)
			return true
		}

		// the client must authenticate before its auth window elapses
		raw.SetReadDeadline(time.Now().Add(playerAuthWait))
		var first struct {
			Type string `json:"type"`
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if err := raw.ReadJSON(&first); err != nil || first.Type != "auth" {
			return
		}
		player, err := a.Store.PlayerByToken(first.Data.Token)
		if err != nil {
			// unknown or dead token (banned included): reject and hang up.
			// Written synchronously — pre-auth nothing else writes, and a
			// pump delivery would race the deferred socket close.
			msg := "sign in required"
			if errors.Is(err, store.ErrBanned) {
				msg = "account banned"
			}
			raw.SetWriteDeadline(time.Now().Add(playerWriteWait))
			if payload, err := json.Marshal(map[string]any{
				"type": "error", "data": map[string]any{"message": msg},
			}); err == nil {
				_ = raw.WriteMessage(websocket.TextMessage, payload)
			}
			_ = raw.WriteMessage(websocket.CloseMessage, nil)
			return
		}
		playerID = player.ID

		// subscribe before snapshotting: a push committed while the
		// snapshot is being rendered is queued and replayed instead of
		// falling into a gap — replayed state is idempotent
		handle = a.Presence.Subscribe(playerID)
		subscribed = true
		go func() {
			for ev := range handle.C() {
				if raw := mustEnvelope(ev.Name, ev.Data); raw != nil {
					c.deliver(raw)
				}
			}
		}()

		raw.SetReadDeadline(time.Now().Add(playerPongWait))
		raw.SetPongHandler(func(string) error {
			raw.SetReadDeadline(time.Now().Add(playerPongWait))
			return nil
		})
		if !send("player", map[string]any{"player": a.toPlayerJSON(player)}) {
			return
		}
		// nothing else to read: drain until the client (or the kick) ends
		// the socket, resetting the deadline on pongs
		for {
			if _, _, err := raw.ReadMessage(); err != nil {
				return
			}
		}
	})
}

// mustEnvelope frames a presence event as a room-style {type, data} message.
func mustEnvelope(name string, data []byte) []byte {
	raw, err := json.Marshal(map[string]any{"type": name, "data": json.RawMessage(data)})
	if err != nil {
		return nil
	}
	return raw
}
