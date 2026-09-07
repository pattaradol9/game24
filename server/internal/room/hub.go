// Package room implements in-memory multiplayer rooms: players race to
// solve the same dealt hand, first correct submission wins the round.
package room

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/pattaradol9/game24/server/internal/game"
)

const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// newSecret returns a random string of n characters from the code
// alphabet (~5 bits of entropy each); room codes use 6, resume secrets 24.
func newSecret(n int) string {
	out := make([]byte, n)
	for i := range out {
		m, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			panic(err)
		}
		out[i] = codeAlphabet[m.Int64()]
	}
	return string(out)
}

func newCode() string { return newSecret(6) }

// Config is fixed at room creation by the host.
type Config struct {
	Mode       game.Mode `json:"mode"`
	Rounds     int       `json:"rounds"`     // 12 / 24 / 36 / 48
	HintQuota  int       `json:"hintQuota"`  // base, identical for everyone
	RegenQuota int       `json:"regenQuota"` // base, identical for everyone
}

// AwardEXP is called for signed-in round winners: (dbPlayerID, mode, points).
type AwardEXP func(dbPlayerID string, mode game.Mode, points int64)

// removeGrace keeps an emptied room around briefly so a page refresh —
// which drops the websocket and rejoins a moment later — finds it alive.
// hostGrace is how long a room with members survives after the host
// disconnects, in case it is just a refresh; past that the room closes.
// playerGrace is how long a member's seat stays warm (flagged absent)
// after their connection drops, before the seat is reaped.
var (
	removeGrace = 15 * time.Second
	hostGrace   = 30 * time.Second
	playerGrace = 10 * time.Second
)

type Hub struct {
	mu       sync.Mutex
	rooms    map[string]*Room
	removals map[string]*time.Timer
	closes   map[string]*time.Timer
	award    AwardEXP
}

func NewHub(award AwardEXP) *Hub {
	return &Hub{rooms: map[string]*Room{}, award: award}
}

func (h *Hub) Create(cfg Config) (*Room, string, error) {
	if _, err := game.Config(cfg.Mode); err != nil {
		return nil, "", err
	}
	switch cfg.Rounds {
	case 12, 24, 36, 48:
	default:
		return nil, "", fmt.Errorf("room: rounds must be 12, 24, 36 or 48")
	}
	if cfg.HintQuota < 0 || cfg.HintQuota > 5 || cfg.RegenQuota < 0 || cfg.RegenQuota > 5 {
		return nil, "", fmt.Errorf("room: quotas must be within 0..5")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	r := newRoom(newCode(), newCode(), cfg, h.award)
	r.OnEmpty(h.scheduleRemove)
	r.OnRepopulated(h.cancelRemove)
	r.OnHostLost(h.scheduleClose)
	r.OnHostReturned(h.cancelClose)
	h.rooms[r.code] = r
	return r, r.hostKey, nil
}

func (h *Hub) Get(code string) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[code]
	if !ok {
		return nil, fmt.Errorf("room: room %s not found", code)
	}
	return r, nil
}

func (h *Hub) Remove(code string) {
	h.mu.Lock()
	delete(h.rooms, code)
	if h.removals != nil {
		if t := h.removals[code]; t != nil {
			t.Stop()
			delete(h.removals, code)
		}
	}
	if h.closes != nil {
		if t := h.closes[code]; t != nil {
			t.Stop()
			delete(h.closes, code)
		}
	}
	h.mu.Unlock()
}

// scheduleRemove queues an emptied room for deletion after the grace
// period, so a player whose page refreshed can rejoin the same room.
func (h *Hub) scheduleRemove(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[code]; !ok {
		return
	}
	if h.removals == nil {
		h.removals = map[string]*time.Timer{}
	}
	if t := h.removals[code]; t != nil {
		t.Stop()
	}
	h.removals[code] = time.AfterFunc(removeGrace, func() { h.Remove(code) })
}

// cancelRemove aborts a pending removal because someone joined again.
func (h *Hub) cancelRemove(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t := h.removals[code]; t != nil {
		t.Stop()
		delete(h.removals, code)
	}
}

// scheduleClose starts the countdown that closes a room whose host
// disconnected and did not come back within the grace window.
func (h *Hub) scheduleClose(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[code]; !ok {
		return
	}
	if h.closes == nil {
		h.closes = map[string]*time.Timer{}
	}
	if t := h.closes[code]; t != nil {
		t.Stop()
	}
	h.closes[code] = time.AfterFunc(hostGrace, func() { h.closeRoom(code) })
}

// cancelClose aborts a pending close because the host rejoined in time.
func (h *Hub) cancelClose(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t := h.closes[code]; t != nil {
		t.Stop()
		delete(h.closes, code)
	}
}

// closeRoom tears the room down after the host reconnect grace elapses:
// everyone still inside receives room_closed and joins start failing.
func (h *Hub) closeRoom(code string) {
	h.mu.Lock()
	r, ok := h.rooms[code]
	if ok {
		delete(h.rooms, code)
		if h.removals != nil {
			if t := h.removals[code]; t != nil {
				t.Stop()
				delete(h.removals, code)
			}
		}
		if h.closes != nil {
			delete(h.closes, code)
		}
	}
	h.mu.Unlock()
	if ok {
		r.Shutdown()
	}
}
