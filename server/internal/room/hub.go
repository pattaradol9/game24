// Package room implements in-memory multiplayer rooms: players race to
// solve the same dealt hand, first correct submission wins the round.
package room

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/pattaradol9/game24/server/internal/game"
)

const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newCode() string {
	out := make([]byte, 6)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			panic(err)
		}
		out[i] = codeAlphabet[n.Int64()]
	}
	return string(out)
}

// Config is fixed at room creation by the host.
type Config struct {
	Mode       game.Mode `json:"mode"`
	Rounds     int       `json:"rounds"`     // 12 / 24 / 36 / 48
	HintQuota  int       `json:"hintQuota"`  // base, identical for everyone
	RegenQuota int       `json:"regenQuota"` // base, identical for everyone
}

// AwardEXP is called for signed-in round winners: (dbPlayerID, mode, points).
type AwardEXP func(dbPlayerID string, mode game.Mode, points int64)

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
	award AwardEXP
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
	r.OnEmpty(h.Remove)
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
	h.mu.Unlock()
}
