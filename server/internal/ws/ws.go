// Package ws upgrades and pumps room websocket connections, dispatching
// client messages into the room hub.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/pattaradol9/game24/server/internal/game"
	"github.com/pattaradol9/game24/server/internal/progress"
	"github.com/pattaradol9/game24/server/internal/room"
	"github.com/pattaradol9/game24/server/internal/store"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
	sendBuffer = 128
)

type Handler struct {
	hub            *room.Hub
	store          *store.Store
	allowedOrigins []string
}

func NewHandler(hub *room.Hub, st *store.Store, allowedOrigins []string) *Handler {
	return &Handler{hub: hub, store: st, allowedOrigins: allowedOrigins}
}

type envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type conn struct {
	ws   *websocket.Conn
	send chan []byte
	once sync.Once
}

func (c *conn) Deliver(raw []byte) {
	select {
	case c.send <- raw:
	default:
		// slow consumer: kick rather than block the room
		c.once.Do(func() { go c.ws.Close() })
	}
}

func (c *conn) CloseConn() { c.once.Do(func() { go c.ws.Close() }) }

// CheckOrigin reports whether an upgrade request's Origin may connect.
// Same host — the SPA served by this binary — is always allowed; anything
// else must be on the configured CORS origin list.
func CheckOrigin(req *http.Request, allowed []string) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if u, err := url.Parse(origin); err == nil && u.Host == req.Host {
		return true
	}
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	rm, err := h.hub.Get(code)
	if err != nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: func(req *http.Request) bool {
			return CheckOrigin(req, h.allowedOrigins)
		},
	}
	raw, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &conn{ws: raw, send: make(chan []byte, sendBuffer)}

	// writer pump
	go func() {
		ping := time.NewTicker(pingPeriod)
		defer ping.Stop()
		for {
			select {
			case msg, ok := <-c.send:
				rawWs := raw
				rawWs.SetWriteDeadline(time.Now().Add(writeWait))
				if !ok {
					rawWs.WriteMessage(websocket.CloseMessage, nil)
					return
				}
				if err := rawWs.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ping.C:
				raw.SetWriteDeadline(time.Now().Add(writeWait))
				if err := raw.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// reader loop
	var sess string
	defer func() {
		c.CloseConn()
		if sess != "" {
			rm.Leave(sess, c)
		}
	}()
	raw.SetReadDeadline(time.Now().Add(pongWait))
	raw.SetPongHandler(func(string) error { raw.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		var env envelope
		if err := raw.ReadJSON(&env); err != nil {
			return
		}
		raw.SetReadDeadline(time.Now().Add(pongWait))
		switch env.Type {
		case "join":
			if sess != "" {
				h.reply(c, "error", map[string]any{"message": "already joined"})
				continue
			}
			var jd struct {
				Name    string `json:"name"`
				Token   string `json:"token"`
				HostKey string `json:"hostKey"`
				Resume  string `json:"resume"`
			}
			if err := json.Unmarshal(env.Data, &jd); err != nil {
				h.reply(c, "error", map[string]any{"message": "bad join payload"})
				continue
			}
			name := trimName(jd.Name)
			if name == "" {
				h.reply(c, "error", map[string]any{"message": "name required"})
				continue
			}
			info := room.Info{SessionID: newSession(), Name: name, HostKey: jd.HostKey, Resume: jd.Resume}
			if jd.Token != "" {
				if p, err := h.store.PlayerByToken(jd.Token); err == nil && !p.IsGuest {
					info.DBID = p.ID
					info.Level = progress.LevelFromExp(p.TotalExp)
					info.Tier = progress.TierName(p.EffectiveTier())
				}
			}
			// the seat may be a resumed one — report its id, not the
			// fresh session id, so the client's "you" stays consistent
			sess = rm.Join(info, c)
			h.reply(c, "joined", map[string]any{"sessionId": sess})
		case "rename":
			if sess == "" {
				h.reply(c, "error", map[string]any{"message": "join first"})
				continue
			}
			var rd struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(env.Data, &rd); err != nil {
				h.reply(c, "error", map[string]any{"message": "bad rename payload"})
				continue
			}
			name := trimName(rd.Name)
			if name == "" {
				h.reply(c, "error", map[string]any{"message": "name required"})
				continue
			}
			if err := rm.Rename(sess, name); err != nil {
				h.reply(c, "error", map[string]any{"message": err.Error()})
			}
		case "start":
			if sess == "" {
				h.reply(c, "error", map[string]any{"message": "join first"})
				continue
			}
			if err := rm.Start(sess); err != nil {
				h.reply(c, "error", map[string]any{"message": err.Error()})
			}
		case "submit":
			if sess == "" {
				h.reply(c, "error", map[string]any{"message": "join first"})
				continue
			}
			var sd struct {
				Steps []game.Step `json:"steps"`
			}
			if err := json.Unmarshal(env.Data, &sd); err != nil {
				h.reply(c, "error", map[string]any{"message": "bad submit payload"})
				continue
			}
			if err := rm.Submit(sess, sd.Steps); err != nil {
				h.reply(c, "error", map[string]any{"message": err.Error()})
			}
		case "hint":
			if sess == "" {
				h.reply(c, "error", map[string]any{"message": "join first"})
				continue
			}
			if err := rm.Hint(sess); err != nil {
				h.reply(c, "error", map[string]any{"message": err.Error()})
			}
		case "regen":
			if sess == "" {
				h.reply(c, "error", map[string]any{"message": "join first"})
				continue
			}
			if err := rm.Regen(sess); err != nil {
				h.reply(c, "error", map[string]any{"message": err.Error()})
			}
		default:
			h.reply(c, "error", map[string]any{"message": "unknown message type"})
		}
	}
}

func (h *Handler) reply(c *conn, typ string, data any) {
	raw, _ := json.Marshal(map[string]any{"type": typ, "data": data})
	c.Deliver(raw)
}

func trimName(s string) string {
	// strip spaces, cap length
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			continue
		}
		out = append(out, r)
		if len(out) >= 24 {
			break
		}
	}
	return string(out)
}

func newSession() string {
	const hexDigits = "0123456789abcdef"
	b := make([]byte, 16)
	if err := randRead(b); err != nil {
		log.Printf("ws: session entropy: %v", err)
	}
	out := make([]byte, 32)
	for i, v := range b {
		out[i*2] = hexDigits[v>>4]
		out[i*2+1] = hexDigits[v&0x0f]
	}
	return string(out)
}
