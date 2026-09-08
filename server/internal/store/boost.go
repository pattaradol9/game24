// Server-wide boosts: one admin-authored campaign per payout kind — "exp"
// multiplies the EXP every solved hand banks, "coins" multiplies its coin
// payout. Each kind is a singleton row in boosts (kind unique) holding the
// multiplier, duration, label and run state. Saving a config never arms it —
// EnableBoost starts the window (now → now + duration) and can be called
// again later to re-arm, DisableBoost stops it early. Every mutation writes
// an event_logs row, like the other admin mutations.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

// BoostKind selects which payout a boost multiplies.
type BoostKind string

const (
	BoostKindExp   BoostKind = "exp"
	BoostKindCoins BoostKind = "coins"
)

// ValidBoostKind reports whether name names a boost row.
func ValidBoostKind(name string) bool {
	return BoostKind(name) == BoostKindExp || BoostKind(name) == BoostKindCoins
}

// Admin-facing bounds: multipliers from 1× (a no-op) to 100×, durations from
// one minute to thirty days.
const (
	BoostMultiplierMax = 100
	BoostDurationMin   = 1
	BoostDurationMax   = 30 * 24 * 60 // thirty days, in minutes
	BoostLabelMax      = 80
)

// ErrBoostRange marks an out-of-range multiplier or duration.
var ErrBoostRange = errors.New("store: boost value out of range")

// ErrBoostKind marks an unknown boost kind.
var ErrBoostKind = errors.New("store: unknown boost kind")

// ValidateBoost checks the admin-authored values.
func ValidateBoost(multiplier float64, durationMin int) error {
	switch {
	case math.IsNaN(multiplier), math.IsInf(multiplier, 0),
		multiplier < 1, multiplier > BoostMultiplierMax:
		return fmt.Errorf("%w: multiplier must be 1..%d", ErrBoostRange, BoostMultiplierMax)
	case durationMin < BoostDurationMin || durationMin > BoostDurationMax:
		return fmt.Errorf("%w: duration must be %d..%d minutes", ErrBoostRange, BoostDurationMin, BoostDurationMax)
	}
	return nil
}

// Boost is one kind's config row plus its run state, as the admin portal
// sees it.
type Boost struct {
	Kind        BoostKind
	Multiplier  float64
	DurationMin int
	Label       string
	Enabled     bool
	StartsAt    time.Time // zero while the boost never ran
	EndsAt      time.Time // zero while the boost never ran
	UpdatedBy   string
	UpdatedAt   time.Time
}

// ActiveBoost is the player-facing view: the multiplier applied to every
// solved hand's payout and when the window closes.
type ActiveBoost struct {
	Kind       BoostKind
	Multiplier float64
	EndsAt     time.Time
}

// BoostAmount maps a base payout through the multiplier, rounded to the
// nearest whole unit. Pure, so the payout can be recomputed and displayed
// without a second read.
func BoostAmount(v int64, multiplier float64) int64 {
	return int64(math.Round(float64(v) * multiplier))
}

// ActiveBoost returns the boost of the given kind to apply right now, if
// any. The window is compared against SQLite's own clock so the check
// matches the starts/ends timestamps written at enable time. Callers must
// not run this inside another transaction — the pool holds a single
// connection.
func (s *Store) ActiveBoost(kind BoostKind) (ActiveBoost, bool) {
	var (
		m       float64
		endsStr string
	)
	err := s.db.QueryRow(`SELECT multiplier, ends_at FROM boosts
		WHERE kind = ? AND enabled = 1 AND ends_at IS NOT NULL AND ends_at > datetime('now')`,
		string(kind)).
		Scan(&m, &endsStr)
	if err != nil {
		return ActiveBoost{}, false
	}
	ends := parseDBTime(endsStr)
	if ends.IsZero() {
		return ActiveBoost{}, false
	}
	return ActiveBoost{Kind: kind, Multiplier: m, EndsAt: ends}, true
}

// BoostState reads one kind's whole config row (the admin portal view).
func (s *Store) BoostState(kind BoostKind) (Boost, error) {
	if !ValidBoostKind(string(kind)) {
		return Boost{}, ErrBoostKind
	}
	var (
		b       Boost
		enabled int
		starts  sql.NullString
		ends    sql.NullString
		updated string
	)
	err := s.db.QueryRow(`SELECT multiplier, duration_min, label, enabled, starts_at, ends_at, updated_by, updated_at
		FROM boosts WHERE kind = ?`, string(kind)).
		Scan(&b.Multiplier, &b.DurationMin, &b.Label, &enabled, &starts, &ends, &b.UpdatedBy, &updated)
	if err != nil {
		return b, err
	}
	b.Kind = kind
	b.Enabled = enabled == 1
	if starts.Valid && starts.String != "" {
		b.StartsAt = parseDBTime(starts.String)
	}
	if ends.Valid && ends.String != "" {
		b.EndsAt = parseDBTime(ends.String)
	}
	b.UpdatedAt = parseDBTime(updated)
	return b, nil
}

// AllBoosts returns every kind's config row, exp first.
func (s *Store) AllBoosts() ([]Boost, error) {
	out := make([]Boost, 0, 2)
	for _, kind := range []BoostKind{BoostKindExp, BoostKindCoins} {
		b, err := s.BoostState(kind)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// SetBoostConfig saves the multiplier, duration and label of one kind
// without touching its run state — a draft can be prepared while the boost
// is live (the multiplier change lands on subsequent hands immediately).
func (s *Store) SetBoostConfig(actor string, kind BoostKind, multiplier float64, durationMin int, label string) (Boost, error) {
	if !ValidBoostKind(string(kind)) {
		return Boost{}, ErrBoostKind
	}
	if err := ValidateBoost(multiplier, durationMin); err != nil {
		return Boost{}, err
	}
	if len(label) > BoostLabelMax {
		label = label[:BoostLabelMax]
	}
	before, err := s.BoostState(kind)
	if err != nil {
		return Boost{}, err
	}
	if _, err := s.db.Exec(`UPDATE boosts SET multiplier = ?, duration_min = ?, label = ?, updated_by = ?, updated_at = datetime('now') WHERE kind = ?`,
		multiplier, durationMin, label, actor, string(kind)); err != nil {
		return Boost{}, err
	}
	s.LogEvent(actor, "admin.boost.set", "", mustJSON(map[string]any{
		"kind":      string(kind),
		"before":    map[string]any{"multiplier": before.Multiplier, "durationMin": before.DurationMin, "label": before.Label},
		"after":     map[string]any{"multiplier": multiplier, "durationMin": durationMin, "label": label},
		"wasActive": before.Enabled && !before.EndsAt.IsZero() && before.EndsAt.After(time.Now()),
	}))
	return s.BoostState(kind)
}

// EnableBoost arms one kind: the window runs from now for the configured
// duration. Enabling an already-active boost restarts the window — a handy
// way to hand players another full stretch.
func (s *Store) EnableBoost(actor string, kind BoostKind) (Boost, error) {
	if !ValidBoostKind(string(kind)) {
		return Boost{}, ErrBoostKind
	}
	b, err := s.BoostState(kind)
	if err != nil {
		return Boost{}, err
	}
	if err := ValidateBoost(b.Multiplier, b.DurationMin); err != nil {
		return Boost{}, err
	}
	starts := time.Now().UTC().Truncate(time.Second)
	ends := starts.Add(time.Duration(b.DurationMin) * time.Minute)
	if _, err := s.db.Exec(`UPDATE boosts SET enabled = 1, starts_at = ?, ends_at = ?, updated_by = ?, updated_at = datetime('now') WHERE kind = ?`,
		sqliteTime(starts), sqliteTime(ends), actor, string(kind)); err != nil {
		return Boost{}, err
	}
	s.LogEvent(actor, "admin.boost.enable", "", mustJSON(map[string]any{
		"kind": string(kind), "multiplier": b.Multiplier, "durationMin": b.DurationMin, "endsAt": sqliteTime(ends),
	}))
	return s.BoostState(kind)
}

// DisableBoost stops one kind immediately; the window timestamps stay on the
// row for the record but stop applying.
func (s *Store) DisableBoost(actor string, kind BoostKind) (Boost, error) {
	if !ValidBoostKind(string(kind)) {
		return Boost{}, ErrBoostKind
	}
	if _, err := s.db.Exec(`UPDATE boosts SET enabled = 0, updated_by = ?, updated_at = datetime('now') WHERE kind = ?`, actor, string(kind)); err != nil {
		return Boost{}, err
	}
	s.LogEvent(actor, "admin.boost.disable", "", mustJSON(map[string]any{"kind": string(kind)}))
	return s.BoostState(kind)
}

// sqliteTime renders a time the way SQLite's datetime('now') writes them
// (UTC, second resolution) — the format parseDBTime reads back.
func sqliteTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}
