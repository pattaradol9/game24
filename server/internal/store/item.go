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

// ErrNoItems marks using an item the player has none of.
var ErrNoItems = errors.New("store: no items in inventory")

// ErrBoostDurationCap marks a use that would add no time at all because the
// kind's window already runs to the 24-hour cap; nothing is consumed. Uses
// that only PARTLY fit are clamped at the cap instead (capped=true).
var ErrBoostDurationCap = errors.New("store: boost window would exceed the 24h cap")

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

// BuyItem exchanges coins for one more unit of an item. Google-signed-in
// players only; the debit and the inventory upsert share one transaction.
func (s *Store) BuyItem(playerID, itemID string) error {
	def, ok := items.ByID(itemID)
	if !ok || def.Price <= 0 {
		return ErrNotFound
	}
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
	if coins < def.Price {
		return ErrInsufficientCoins
	}
	if _, err := tx.Exec(`INSERT INTO player_items (player_id, item_id, qty) VALUES (?,?,1)
		ON CONFLICT(player_id, item_id) DO UPDATE SET qty = qty + 1`, playerID, itemID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE players SET total_coins = total_coins - ? WHERE id = ?`, def.Price, playerID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.LogEvent("player:"+playerID, "item.buy", itemID, mustJSON(map[string]any{"price": def.Price}))
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
		return ActiveBoost{}, "", false, ErrNoItems
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

// payoutMultiplier stacks every boost source of a kind additively on top of
// the base payout: each source contributes its bonus (multiplier − 1), so a
// ×2 server boost plus a ×2 personal item pay ×3 total — never the compounded
// ×4. Both lookups happen before the award transaction opens.
func (s *Store) payoutMultiplier(playerID string, kind BoostKind) float64 {
	total := 1.0
	if b, ok := s.ActiveBoost(kind); ok {
		total += b.Multiplier - 1
	}
	if b, ok := s.ActivePlayerBoost(playerID, kind); ok {
		total += b.Multiplier - 1
	}
	return total
}
