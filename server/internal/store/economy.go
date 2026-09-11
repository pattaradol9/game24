// Coin economy, achievements and card skins.
//
// Every solved hand pays EXP and coins (CoinsForHand). After each award the
// player is re-evaluated against the achievement catalog; newly unlocked
// entries pay their own EXP/coin rewards, so one round can bank both the
// hand payout and a cascade of achievement payouts. Skins are bought with
// coins and are gated to Google-signed-in players.
package store

import (
	"database/sql"
	"errors"

	"github.com/pattaradol9/game24/server/internal/achv"
	"github.com/pattaradol9/game24/server/internal/progress"
	"github.com/pattaradol9/game24/server/internal/skins"
)

// CoinsForHand maps a solved hand's EXP points to its coin payout: one coin
// per full ten points, at least one. Deterministic, so callers can display
// the payout without an extra round trip.
func CoinsForHand(points int64) int64 {
	if points < 10 {
		return 1
	}
	return points / 10
}

// AwardResult is the payout of one finished hand plus the totals it led to.
type AwardResult struct {
	Stat ModeStat
	// ExpEarned and CoinsEarned are what the hand itself paid into the
	// ledgers — base amounts with the boost multipliers already applied:
	// server-wide campaigns plus, for solo hands, the player's own items.
	// A multiplayer round win ignores personal item boosts so every seat
	// is paid on equal footing. Achievement rewards banked in the same
	// call are not part of them.
	ExpEarned   int64
	CoinsEarned int64
	TotalExp    int64
	TotalCoins  int64
	TotalWins   int64
}

// AwardSolve banks a solved hand (EXP + coins), optionally counting a
// multiplayer round win, then evaluates achievements. The returned defs were
// unlocked by this solve; their rewards are already applied to the totals.
func (s *Store) AwardSolve(playerID, mode string, points int64, roomWin bool) (AwardResult, []achv.Def, error) {
	stat, _, payout, err := s.awardEXP(playerID, mode, points, true, roomWin)
	if err != nil {
		return AwardResult{}, nil, err
	}
	if roomWin {
		if _, err := s.db.Exec(`UPDATE players SET total_wins = total_wins + 1 WHERE id = ?`, playerID); err != nil {
			return AwardResult{}, nil, err
		}
	}
	unlocked, err := s.EvaluateAchievements(playerID)
	if err != nil {
		return AwardResult{}, nil, err
	}
	res, err := s.readTotals(playerID, stat, payout.CoinGain)
	res.ExpEarned = payout.ExpGain
	return res, unlocked, err
}

// AwardSkip records a skipped (or timed-out) hand — no payout, but
// skip-counting achievements are still evaluated.
func (s *Store) AwardSkip(playerID, mode string) ([]achv.Def, error) {
	if _, _, err := s.AwardEXP(playerID, mode, 0, false); err != nil {
		return nil, err
	}
	return s.EvaluateAchievements(playerID)
}

func (s *Store) readTotals(playerID string, stat ModeStat, coinsEarned int64) (AwardResult, error) {
	var res AwardResult
	res.Stat = stat
	res.CoinsEarned = coinsEarned
	err := s.db.QueryRow(`SELECT total_exp, total_coins, total_wins FROM players WHERE id = ?`, playerID).
		Scan(&res.TotalExp, &res.TotalCoins, &res.TotalWins)
	if errors.Is(err, sql.ErrNoRows) {
		return res, ErrNotFound
	}
	return res, err
}

// PlayerAchievements returns the player's unlocked achievements as
// achievement id → unlock timestamp (SQLite datetime('now') string, UTC).
func (s *Store) PlayerAchievements(playerID string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT achievement_id, unlocked_at FROM player_achievements WHERE player_id = ?`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	unlocked := map[string]string{}
	for rows.Next() {
		var id, ts string
		if err := rows.Scan(&id, &ts); err != nil {
			return nil, err
		}
		unlocked[id] = ts
	}
	return unlocked, rows.Err()
}

// AchievementState returns the unlock timestamps plus the lifetime snapshot
// used for progress display. The timestamps are stored as SQLite
// datetime('now') strings (UTC).
func (s *Store) AchievementState(playerID string) (map[string]string, achv.Snapshot, error) {
	unlocked, err := s.PlayerAchievements(playerID)
	if err != nil {
		return nil, achv.Snapshot{}, err
	}
	snap, err := s.AchievementSnapshot(playerID)
	if err != nil {
		return nil, achv.Snapshot{}, err
	}
	return unlocked, snap, nil
}

// AchievementSnapshot gathers every lifetime counter the catalog tracks.
func (s *Store) AchievementSnapshot(playerID string) (achv.Snapshot, error) {
	var snap achv.Snapshot
	var totalExp int64
	err := s.db.QueryRow(`SELECT total_exp, total_coins, total_wins FROM players WHERE id = ?`, playerID).
		Scan(&totalExp, &snap.TotalCoins, &snap.Wins)
	if errors.Is(err, sql.ErrNoRows) {
		return snap, ErrNotFound
	}
	if err != nil {
		return snap, err
	}
	snap.TotalExp = totalExp
	snap.Level = progress.LevelFromExp(totalExp)

	stats, err := s.ModeStats(playerID)
	if err != nil {
		return snap, err
	}
	snap.Solved = map[string]int64{"": 0}
	snap.BestStreak = map[string]int64{"": 0}
	for mode, st := range stats {
		snap.Solved[""] += st.HandsSolved
		snap.Solved[mode] = st.HandsSolved
		snap.Skipped += st.HandsSkipped
		if st.BestStreak > snap.BestStreak[""] {
			snap.BestStreak[""] = st.BestStreak
		}
		snap.BestStreak[mode] = st.BestStreak
		if st.HandsSolved > 0 {
			snap.ModesPlayed++
		}
	}

	// round-derived counters: hints ever spent, no-hint solves, fastest
	// solve, distinct play days
	err = s.db.QueryRow(`SELECT
			COALESCE(SUM(hints_used), 0),
			COALESCE(SUM(CASE WHEN status = 'solved' AND hints_used = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(MIN(CASE WHEN status = 'solved' THEN elapsed_ms END), 0),
			COUNT(DISTINCT substr(dealt_at, 1, 10))
		FROM rounds WHERE player_id = ?`, playerID).
		Scan(&snap.HintsUsed, &snap.SolvesNoHint, &snap.BestTimeMs, &snap.PlayDays)
	return snap, err
}

// EvaluateAchievements unlocks every catalog entry the player now qualifies
// for, paying each entry's EXP/coin rewards, and returns what was unlocked.
func (s *Store) EvaluateAchievements(playerID string) ([]achv.Def, error) {
	unlocked, snap, err := s.AchievementState(playerID)
	if err != nil {
		return nil, err
	}
	pending := achv.Evaluate(snap, toBoolSet(unlocked))
	if len(pending) == 0 {
		return nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var expReward, coinReward int64
	for _, d := range pending {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO player_achievements (player_id, achievement_id) VALUES (?,?)`,
			playerID, d.ID); err != nil {
			return nil, err
		}
		expReward += d.ExpReward
		coinReward += d.CoinReward
	}
	if _, err := tx.Exec(`UPDATE players SET total_exp = total_exp + ?, total_coins = total_coins + ? WHERE id = ?`,
		expReward, coinReward, playerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return pending, nil
}

func toBoolSet(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for id := range m {
		out[id] = true
	}
	return out
}

// BuySkin purchases a card skin with the player's coins. Google-signed-in
// players only; the default classic skin is never purchasable.
func (s *Store) BuySkin(playerID, skinID string) error {
	def, ok := skins.ByID(skinID)
	if !ok || skinID == "classic" {
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
	res, err := tx.Exec(`INSERT OR IGNORE INTO player_skins (player_id, skin_id) VALUES (?,?)`, playerID, skinID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAlreadyOwned
	}
	if _, err := tx.Exec(`UPDATE players SET total_coins = total_coins - ? WHERE id = ?`, def.Price, playerID); err != nil {
		return err
	}
	return tx.Commit()
}

// EquipSkin equips an owned skin (or the free classic, always allowed).
func (s *Store) EquipSkin(playerID, skinID string) error {
	if skinID == "classic" {
		skinID = ""
	}
	if skinID != "" {
		var owned int
		err := s.db.QueryRow(`SELECT 1 FROM player_skins WHERE player_id = ? AND skin_id = ?`, playerID, skinID).Scan(&owned)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && owned != 1) {
			return ErrNotOwned
		}
		if err != nil {
			return err
		}
	}
	res, err := s.db.Exec(`UPDATE players SET skin = ? WHERE id = ?`, skinID, playerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// OwnedSkins lists the player's owned skin ids, classic (free default) first.
func (s *Store) OwnedSkins(playerID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT skin_id FROM player_skins WHERE player_id = ? ORDER BY acquired_at`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{"classic"}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
