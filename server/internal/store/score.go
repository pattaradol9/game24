// Gameplay score: the leaderboard metric of record.
//
// Every solved hand banks its raw points — (10 + seconds remaining) × mode
// multiplier — into the player_scores ledger, in the same transaction as the
// EXP/coin payout. The leaderboard ranks each player's HIGH SCORE: the best
// single hand per mode, not a running sum. Score is deliberately its own
// ledger — payout boosts (server-wide campaigns and personal items) multiply
// EXP and coins but never touch score — so the board always ranks pure play.
// The timestamped rows are what the weekly window aggregates over.
package store

import (
	"database/sql"
	"time"
)

// sqliteTimeLayout matches SQLite datetime('now') strings (UTC); awarded_at
// comparisons are plain lexicographic TEXT compares against it.
const sqliteTimeLayout = "2006-01-02 15:04:05"

// recordScoreTx banks one solved hand's raw points into the score ledger.
// Must run inside the award transaction so score and stats stay atomic.
func recordScoreTx(tx *sql.Tx, playerID, mode string, points int64) error {
	_, err := tx.Exec(`INSERT INTO player_scores (player_id, mode, points) VALUES (?,?,?)`,
		playerID, mode, points)
	return err
}

// WeekStartUTC truncates t to the start of its ISO week: Monday 00:00:00 UTC.
// Weekly leaderboards count everything scored at or after this instant.
func WeekStartUTC(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday()) // Sunday = 0
	if wd == 0 {
		wd = 7
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -(wd - 1))
}

// ScoreRow is one ranked entry of the gameplay-score leaderboard. Score is
// the player's high score for the mode: their single best hand inside the
// window (the current ISO week when weekly, ever otherwise).
type ScoreRow struct {
	PlayerID     string
	Nickname     string
	Picture      string
	Score        int64
	HandsSolved  int64
	BestStreak   int64
	TotalExp     int64 // lifetime, for the level/tier badge next to the name
	TierOverride string
}

// RecordScore banks one solved hand's raw points into the score ledger — and
// nothing else. It is the anonymous-player path behind the leaderboard:
// guests rank exactly like signed-in players, while EXP/coins, per-mode
// stats and achievements stay gated behind Google sign-in.
func (s *Store) RecordScore(playerID, mode string, points int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO player_scores (player_id, mode, points) VALUES (?,?,?)`,
		playerID, mode, points); err != nil {
		return err
	}
	return tx.Commit()
}

// PlayerHighScores maps each mode to the player's best single hand there —
// the same figure the leaderboard ranks. Only modes with a positive high
// score come back; a mode never played (or never scoring) is absent.
func (s *Store) PlayerHighScores(playerID string) (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT mode, MAX(points) FROM player_scores WHERE player_id = ? GROUP BY mode`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var mode string
		var best int64
		if err := rows.Scan(&mode, &best); err != nil {
			return nil, err
		}
		if best > 0 {
			out[mode] = best
		}
	}
	return out, rows.Err()
}

// ScoreLeaderboard returns the ranked list for one mode, aggregated from the
// score ledger. Each entry's Score is that player's best single hand (high
// score) inside the window: weekly=true ranks hands solved in the current
// ISO week (since Monday 00:00 UTC), false ranks all time. Ties break by
// hands solved, then best streak. Anonymous (guest) players rank like
// everyone else; banned players never appear, and neither does a player
// whose best hand in the window scored 0.
func (s *Store) ScoreLeaderboard(mode string, weekly bool, limit, offset int) ([]ScoreRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `SELECT p.id, p.nickname_enc, p.picture_enc, p.total_exp, p.tier,
			COALESCE(m.best_streak, 0) AS best_streak,
			MAX(sc.points) AS score,
			COUNT(sc.points) AS hands
		FROM player_scores sc
		JOIN players p ON p.id = sc.player_id
		LEFT JOIN player_mode_stats m ON m.player_id = p.id AND m.mode = sc.mode
		WHERE sc.mode = ?`
	args := []any{mode}
	if weekly {
		q += ` AND sc.awarded_at >= ?`
		args = append(args, WeekStartUTC(time.Now()).Format(sqliteTimeLayout))
	}
	q += ` AND p.banned = 0
		GROUP BY p.id
		HAVING MAX(sc.points) > 0
		ORDER BY score DESC, hands DESC, best_streak DESC
		LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreRow
	for rows.Next() {
		var (
			r       ScoreRow
			nickEnc string
			picEnc  string
		)
		if err := rows.Scan(&r.PlayerID, &nickEnc, &picEnc, &r.TotalExp, &r.TierOverride,
			&r.BestStreak, &r.Score, &r.HandsSolved); err != nil {
			return nil, err
		}
		if r.Nickname, err = s.cr.Decrypt(nickEnc, "players.nickname"); err != nil {
			return nil, err
		}
		if r.Picture, err = s.cr.Decrypt(picEnc, "players.picture"); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
