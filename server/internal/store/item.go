// Player inventory and personal boosts: consumable items bought with coins
// (player_items, one stacked qty per item) and the boost windows activating
// one arms (player_boosts, one row per payout kind). A personal boost stacks
// additively with the server-wide campaigns in the payout path — the lookups
// happen beside ActiveBoost, before the award transaction opens.
package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pattaradol9/game24/server/internal/items"
)

// ErrNoItems marks using an item the player has none (or not enough) of.
var ErrNoItems = errors.New("store: no items in inventory")

// ErrInvalidCount marks a buy/use whose unit count is below one.
var ErrInvalidCount = errors.New("store: invalid item count")

// ErrBoostDurationCap marks a use that would add no time at all because the
// kind's window already runs to the 24-hour cap; nothing is consumed. Uses
// that only PARTLY fit are clamped at the cap instead (capped=true).
var ErrBoostDurationCap = errors.New("store: boost window would exceed the 24h cap")

// ErrRoundClosed marks a time extension arriving for a round that already
// finished (expired, skipped or solved).
var ErrRoundClosed = errors.New("store: round already finished")

// ErrSessionExtendUsed marks a second time extension inside one play
// session: the item may bail a hand out once per session, not once per hand.
var ErrSessionExtendUsed = errors.New("store: time extension already used this session")

// ErrRoundTimeFull marks a time extension arriving while the hand still has
// its full window ahead: there is no room under the mode's base limit for
// the item to fill, so nothing is added and nothing is consumed.
var ErrRoundTimeFull = errors.New("store: round time already at this mode's limit")

// ErrSessionSkipUsed marks a second item skip inside one play session: like
// the time extension, the Skip Pass folds a hand once per session.
var ErrSessionSkipUsed = errors.New("store: skip already used this session")

// BoostWindowMax caps the combined remaining duration of one personal boost
// kind. Same-multiplier activations stack time on top of each other, but a
// kind can never run longer than 24h from now.
const BoostWindowMax = 24 * time.Hour

// PlayerItem is one inventory stack: how many of an item the player holds.
type PlayerItem struct {
	ID  string
	Qty int64
}

// PlayerItems lists the player's inventory stacks (qty > 0), item id order.
func (s *Store) PlayerItems(playerID string) ([]PlayerItem, error) {
	rows, err := s.db.Query(`SELECT item_id, qty FROM player_items WHERE player_id = ? AND qty > 0 ORDER BY item_id`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlayerItem
	for rows.Next() {
		var it PlayerItem
		if err := rows.Scan(&it.ID, &it.Qty); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// BuyItem exchanges coins for `count` more units of an item — the whole
// batch debits price×count and upserts the stack in one transaction, so a
// player either affords every unit or spends nothing. Google-signed-in
// players only.
func (s *Store) BuyItem(playerID, itemID string, count int) error {
	def, ok := items.ByID(itemID)
	if !ok || def.Price <= 0 {
		return ErrNotFound
	}
	if count < 1 {
		return ErrInvalidCount
	}
	cost := def.Price * int64(count)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var guest bool
	var coins int64
	err = tx.QueryRow(`SELECT is_guest, total_coins FROM players WHERE id = ?`, playerID).Scan(&guest, &coins)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if guest {
		return ErrGoogleRequired
	}
	if coins < cost {
		return ErrInsufficientCoins
	}
	if _, err := tx.Exec(`INSERT INTO player_items (player_id, item_id, qty) VALUES (?,?,?)
		ON CONFLICT(player_id, item_id) DO UPDATE SET qty = qty + ?`, playerID, itemID, count, count); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE players SET total_coins = total_coins - ? WHERE id = ?`, cost, playerID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.LogEvent("player:"+playerID, "item.buy", itemID, mustJSON(map[string]any{"price": def.Price, "count": count, "cost": cost}))
	return nil
}

// UseItem consumes `count` units of an item and arms its personal boost —
// every unit contributes its own duration, so ×2/30min ×3 = ×2 for 90min.
// What happens to the kind's single window depends on what is running:
//
//   - same kind, same multiplier → the durations STACK onto the live window
//     (×2/30min + ×2/60min = ×2 for 90min), clamped at BoostWindowMax (24h)
//     from now — time past the cap is trimmed (capped=true) and the items
//     are still consumed; only a window already sitting on the cap is
//     refused outright with nothing consumed (ErrBoostDurationCap);
//   - same kind, different multiplier → the fresh window REPLACES the live
//     one (now → now + count×duration, the new item's multiplier);
//   - nothing live → a fresh window.
//
// Boosts of different kinds (EXP + coins) are independent and always run
// alongside each other. The consumed qty and the new window commit together;
// `action` reports what happened ("fresh" | "extended" | "replaced") and
// `capped` whether the window end was trimmed to the 24h cap.
func (s *Store) UseItem(playerID, itemID string, count int) (ActiveBoost, string, bool, error) {
	def, ok := items.ByID(itemID)
	if !ok || !ValidBoostKind(def.Kind) {
		return ActiveBoost{}, "", false, ErrNotFound
	}
	if count < 1 {
		return ActiveBoost{}, "", false, ErrInvalidCount
	}
	kind := BoostKind(def.Kind)
	duration := time.Duration(def.DurationMin) * time.Minute
	tx, err := s.db.Begin()
	if err != nil {
		return ActiveBoost{}, "", false, err
	}
	defer tx.Rollback()
	var guest bool
	err = tx.QueryRow(`SELECT is_guest FROM players WHERE id = ?`, playerID).Scan(&guest)
	if errors.Is(err, sql.ErrNoRows) {
		return ActiveBoost{}, "", false, ErrNotFound
	}
	if err != nil {
		return ActiveBoost{}, "", false, err
	}
	if guest {
		return ActiveBoost{}, "", false, ErrGoogleRequired
	}
	res, err := tx.Exec(`UPDATE player_items SET qty = qty - ? WHERE player_id = ? AND item_id = ? AND qty >= ?`, count, playerID, itemID, count)
	if err != nil {
		return ActiveBoost{}, "", false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ActiveBoost{}, "", false, ErrNoItems
	}
	// what is running for this kind right now decides how the window moves
	var (
		curMult float64
		endsStr sql.NullString
	)
	if err := tx.QueryRow(`SELECT multiplier, ends_at FROM player_boosts WHERE player_id = ? AND kind = ?`, playerID, string(kind)).Scan(&curMult, &endsStr); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ActiveBoost{}, "", false, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	capAt := now.Add(BoostWindowMax)
	ends := now.Add(time.Duration(count) * duration)
	capped := false
	if ends.After(capAt) {
		ends = capAt
		capped = true
	}
	action := "fresh"
	if endsStr.Valid {
		if curEnds := parseDBTime(endsStr.String); curEnds.After(now) {
			if curMult == def.Multiplier {
				// same tier: stack the durations onto the live end, trimmed at
				// the cap — a window already sitting on the cap gains nothing,
				// so the use is refused and rolls back (items not consumed)
				if !capAt.After(curEnds) {
					return ActiveBoost{}, "", false, ErrBoostDurationCap
				}
				target := curEnds.Add(time.Duration(count) * duration)
				if target.After(capAt) {
					target = capAt
					capped = true
				}
				ends = target
				action = "extended"
			} else {
				// different tier: the new buff takes over
				action = "replaced"
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO player_boosts (player_id, kind, multiplier, ends_at, item_id) VALUES (?,?,?,?,?)
		ON CONFLICT(player_id, kind) DO UPDATE SET
			multiplier = excluded.multiplier, ends_at = excluded.ends_at, item_id = excluded.item_id`,
		playerID, string(kind), def.Multiplier, sqliteTime(ends), def.ID); err != nil {
		return ActiveBoost{}, "", false, err
	}
	if _, err := tx.Exec(`DELETE FROM player_items WHERE player_id = ? AND qty <= 0`, playerID); err != nil {
		return ActiveBoost{}, "", false, err
	}
	if err := tx.Commit(); err != nil {
		return ActiveBoost{}, "", false, err
	}
	s.LogEvent("player:"+playerID, "item.use", itemID, mustJSON(map[string]any{
		"kind": string(kind), "multiplier": def.Multiplier, "endsAt": sqliteTime(ends),
		"action": action, "count": count, "capped": capped,
	}))
	return ActiveBoost{Kind: kind, Multiplier: def.Multiplier, EndsAt: ends, ItemID: def.ID}, action, capped, nil
}

// ExtendRound is the solo play helper behind the Add-time button (and the
// "time over?" dialog offering the same spend at zero): it spends ONE unit
// of a time item (kind "time") to push an open round's countdown out by the
// item's ExtraSeconds — pressable any time mid-play, but a hand's remaining
// time may never pass the mode's base window, so the extra is clamped to the
// seconds the hand has already played away on the server clock (a press at
// 1:50 of a 2:00 hand tops up to 2:00, never past it) and a press with no
// room at all is refused with ErrRoundTimeFull. The consumption and the
// round update commit together. The once-per-session rule is enforced in the
// same transaction: when any round of the same session id already carries
// extra time, a second extension is refused (ErrSessionExtendUsed); rounds
// played without a session id fall back to a per-round rule. Players without
// the item in the bag get ErrNoItems — guests among them, since only Google
// players can buy (called out explicitly with ErrGoogleRequired).
func (s *Store) ExtendRound(playerID, roundID, itemID string) (Round, error) {
	def, ok := items.ByID(itemID)
	if !ok || def.Kind != "time" || def.ExtraSeconds <= 0 {
		return Round{}, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Round{}, err
	}
	defer tx.Rollback()
	var (
		status, sessionID string
		extendSec         int64
		dealtAtStr        string
		guest             bool
	)
	err = tx.QueryRow(`SELECT status, session_id, extend_sec, dealt_at FROM rounds WHERE id = ? AND player_id = ?`,
		roundID, playerID).Scan(&status, &sessionID, &extendSec, &dealtAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return Round{}, ErrNotFound
	}
	if err != nil {
		return Round{}, err
	}
	if status != "open" {
		return Round{}, ErrRoundClosed
	}
	if sessionID != "" {
		var n int64
		if err := tx.QueryRow(`SELECT COUNT(*) FROM rounds
			WHERE player_id = ? AND session_id = ? AND extend_sec > 0`, playerID, sessionID).Scan(&n); err != nil {
			return Round{}, err
		}
		if n > 0 {
			return Round{}, ErrSessionExtendUsed
		}
	} else if extendSec > 0 {
		return Round{}, ErrSessionExtendUsed
	}
	if err := tx.QueryRow(`SELECT is_guest FROM players WHERE id = ?`, playerID).Scan(&guest); err != nil {
		return Round{}, err
	}
	if guest {
		return Round{}, ErrGoogleRequired
	}
	res, err := tx.Exec(`UPDATE player_items SET qty = qty - 1 WHERE player_id = ? AND item_id = ? AND qty >= 1`,
		playerID, itemID)
	if err != nil {
		return Round{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Round{}, ErrNoItems
	}
	// the countdown may only fill the room the hand has already played away:
	// remaining time after the extension stays inside the mode's base window
	extra := int64(def.ExtraSeconds)
	dealtAt, err := time.Parse("2006-01-02 15:04:05", dealtAtStr)
	if err != nil {
		return Round{}, err
	}
	if played := int64(time.Since(dealtAt).Seconds()); played < extra {
		extra = played
	}
	if extra <= 0 {
		return Round{}, ErrRoundTimeFull
	}
	res, err = tx.Exec(`UPDATE rounds SET extend_sec = ? WHERE id = ? AND status = 'open'`, extra, roundID)
	if err != nil {
		return Round{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Round{}, ErrRoundClosed
	}
	if _, err := tx.Exec(`DELETE FROM player_items WHERE player_id = ? AND qty <= 0`, playerID); err != nil {
		return Round{}, err
	}
	if err := tx.Commit(); err != nil {
		return Round{}, err
	}
	s.LogEvent("player:"+playerID, "item.use", itemID, mustJSON(map[string]any{
		"roundId": roundID, "sessionId": sessionID, "extraSeconds": extra,
	}))
	return s.RoundByID(roundID, playerID)
}

// SkipRound is the play helper behind the Skip button: it spends ONE unit of
// a skip item (kind "skip") to fold an open round — status "skipped", no
// payout — and stamps rounds.skip_used in the same transaction so the
// once-per-session rule can hold: when any round of the same session id
// already carries the flag, a second skip is refused
// (ErrSessionSkipUsed); rounds played without a session id fall back to a
// per-round rule. Players without the item in the bag get ErrNoItems —
// guests among them, since only Google players can buy (called out
// explicitly with ErrGoogleRequired). A hand that merely expires folds for
// free through the plain timeout path; only this item path skips mid-play.
func (s *Store) SkipRound(playerID, roundID string) (Round, error) {
	def, ok := items.SkipItem()
	if !ok {
		return Round{}, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Round{}, err
	}
	defer tx.Rollback()
	var (
		status, sessionID string
		skipUsed          int64
		dealtAtStr        string
		guest             bool
	)
	err = tx.QueryRow(`SELECT status, session_id, skip_used, dealt_at FROM rounds WHERE id = ? AND player_id = ?`,
		roundID, playerID).Scan(&status, &sessionID, &skipUsed, &dealtAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return Round{}, ErrNotFound
	}
	if err != nil {
		return Round{}, err
	}
	if status != "open" {
		return Round{}, ErrRoundClosed
	}
	if sessionID != "" {
		var n int64
		if err := tx.QueryRow(`SELECT COUNT(*) FROM rounds
			WHERE player_id = ? AND session_id = ? AND skip_used > 0`, playerID, sessionID).Scan(&n); err != nil {
			return Round{}, err
		}
		if n > 0 {
			return Round{}, ErrSessionSkipUsed
		}
	} else if skipUsed > 0 {
		return Round{}, ErrSessionSkipUsed
	}
	if err := tx.QueryRow(`SELECT is_guest FROM players WHERE id = ?`, playerID).Scan(&guest); err != nil {
		return Round{}, err
	}
	if guest {
		return Round{}, ErrGoogleRequired
	}
	res, err := tx.Exec(`UPDATE player_items SET qty = qty - 1 WHERE player_id = ? AND item_id = ? AND qty >= 1`,
		playerID, def.ID)
	if err != nil {
		return Round{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Round{}, ErrNoItems
	}
	dealtAt, err := time.Parse("2006-01-02 15:04:05", dealtAtStr)
	if err != nil {
		return Round{}, err
	}
	res, err = tx.Exec(`UPDATE rounds SET status = 'skipped', skip_used = 1, elapsed_ms = ?, finished_at = datetime('now')
		WHERE id = ? AND status = 'open'`,
		time.Since(dealtAt).Milliseconds(), roundID)
	if err != nil {
		return Round{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Round{}, ErrRoundClosed
	}
	if _, err := tx.Exec(`DELETE FROM player_items WHERE player_id = ? AND qty <= 0`, playerID); err != nil {
		return Round{}, err
	}
	if err := tx.Commit(); err != nil {
		return Round{}, err
	}
	s.LogEvent("player:"+playerID, "item.use", def.ID, mustJSON(map[string]any{
		"roundId": roundID, "sessionId": sessionID, "action": "skip",
	}))
	return s.RoundByID(roundID, playerID)
}

// ActivePlayerBoost returns the player's personal boost of the given kind to
// apply right now, if any. The window is compared against SQLite's own clock
// so the check matches the timestamps written at activation time. Callers
// must not run this inside another transaction — the pool holds a single
// connection.
func (s *Store) ActivePlayerBoost(playerID string, kind BoostKind) (ActiveBoost, bool) {
	var (
		m       float64
		endsStr string
		itemID  string
	)
	err := s.db.QueryRow(`SELECT multiplier, ends_at, item_id FROM player_boosts
		WHERE player_id = ? AND kind = ? AND ends_at IS NOT NULL AND ends_at > datetime('now')`,
		playerID, string(kind)).
		Scan(&m, &endsStr, &itemID)
	if err != nil {
		return ActiveBoost{Kind: kind}, false
	}
	ends := parseDBTime(endsStr)
	if ends.IsZero() {
		return ActiveBoost{Kind: kind}, false
	}
	return ActiveBoost{Kind: kind, Multiplier: m, EndsAt: ends, ItemID: itemID}, true
}

// ActivePlayerBoosts returns every live personal boost keyed by kind.
func (s *Store) ActivePlayerBoosts(playerID string) map[BoostKind]ActiveBoost {
	out := map[BoostKind]ActiveBoost{}
	for _, kind := range []BoostKind{BoostKindExp, BoostKindCoins} {
		if b, ok := s.ActivePlayerBoost(playerID, kind); ok {
			out[kind] = b
		}
	}
	return out
}

// payoutMultiplier stacks boost sources of a kind additively on top of the
// base payout: each source contributes its bonus (multiplier − 1), so a ×2
// server boost plus a ×2 personal item pay ×3 total — never the compounded
// ×4. The server-wide campaign always counts; the player's own items only
// enter when personal is true — multiplayer round wins pass false so every
// seat's payout stays on equal footing (the server boost still applies to
// everyone alike). Both lookups happen before the award transaction opens.
func (s *Store) payoutMultiplier(playerID string, kind BoostKind, personal bool) float64 {
	total := 1.0
	if b, ok := s.ActiveBoost(kind); ok {
		total += b.Multiplier - 1
	}
	if personal {
		if b, ok := s.ActivePlayerBoost(playerID, kind); ok {
			total += b.Multiplier - 1
		}
	}
	return total
}
