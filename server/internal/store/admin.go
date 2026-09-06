// Admin-facing persistence: overview counters, player administration
// (ban/unban/delete/rename, EXP adjustment, stat resets), full leaderboard
// and round listings, and the event log. Every mutation here also writes an
// event_logs row so the admin portal keeps a durable audit trail.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// maxPlayerScan caps how many rows a nickname/email search decrypts in
// memory — PII is encrypted, so search cannot run in SQL.
const maxPlayerScan = 10_000

// --- event log ---

type EventRow struct {
	ID     string
	TS     time.Time
	Actor  string
	Action string
	Target string
	Detail string
}

// LogEvent records an entry in the event log (best-effort, own transaction).
func (s *Store) LogEvent(actor, action, target, detail string) {
	s.db.Exec(`INSERT INTO event_logs (id, actor, action, target, detail) VALUES (?,?,?,?,?)`,
		randomID(16), actor, action, target, detail)
}

func logEventTx(tx *sql.Tx, actor, action, target, detail string) error {
	_, err := tx.Exec(`INSERT INTO event_logs (id, actor, action, target, detail) VALUES (?,?,?,?,?)`,
		randomID(16), actor, action, target, detail)
	return err
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// AdminEvents lists event log entries newest-first, optionally filtered by
// exact action, with the total count for pagination.
func (s *Store) AdminEvents(action string, limit, offset int) ([]EventRow, int, error) {
	limit, offset = clampPage(limit, offset)
	where, args := "1=1", []any{}
	if action != "" {
		where = "action = ?"
		args = append(args, action)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM event_logs WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT id, ts, actor, action, target, detail FROM event_logs WHERE ` + where +
		` ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.Query(q, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Actor, &e.Action, &e.Target, &e.Detail); err != nil {
			return nil, 0, err
		}
		e.TS = parseDBTime(ts)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func clampPage(limit, offset int) (int, int) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// --- overview ---

type ModeOverview struct {
	Mode         string
	Exp          int64
	HandsSolved  int64
	HandsSkipped int64
	BestStreak   int64
}

type DayCount struct {
	Day   string
	Count int64
}

type Overview struct {
	Players    map[string]int64 // total, guests, google, banned
	Rounds     map[string]int64 // status -> count (open/solved/skipped/expired)
	RoundTotal int64
	TotalExp   int64
	Modes      []ModeOverview
	Signups    []DayCount // last 14 days, oldest first
	Events     int64
}

// Overview aggregates the dashboard counters in a few single-pass queries.
func (s *Store) Overview() (Overview, error) {
	o := Overview{
		Players: map[string]int64{"total": 0, "guests": 0, "google": 0, "banned": 0},
		Rounds:  map[string]int64{},
	}
	var total, guests, banned int64
	if err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(is_guest),0), COALESCE(SUM(banned),0) FROM players`).
		Scan(&total, &guests, &banned); err != nil {
		return o, err
	}
	o.Players["total"] = total
	o.Players["guests"] = guests
	o.Players["banned"] = banned
	o.Players["google"] = total - guests

	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM rounds GROUP BY status`)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var st string
		var n int64
		if err := rows.Scan(&st, &n); err != nil {
			rows.Close()
			return o, err
		}
		o.Rounds[st] = n
		o.RoundTotal += n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}

	if err := s.db.QueryRow(`SELECT COALESCE(SUM(total_exp),0) FROM players`).Scan(&o.TotalExp); err != nil {
		return o, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM event_logs`).Scan(&o.Events); err != nil {
		return o, err
	}

	rows, err = s.db.Query(`SELECT mode, COALESCE(SUM(exp),0), COALESCE(SUM(hands_solved),0), COALESCE(SUM(hands_skipped),0), COALESCE(MAX(best_streak),0)
		FROM player_mode_stats GROUP BY mode ORDER BY mode`)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var m ModeOverview
		if err := rows.Scan(&m.Mode, &m.Exp, &m.HandsSolved, &m.HandsSkipped, &m.BestStreak); err != nil {
			rows.Close()
			return o, err
		}
		o.Modes = append(o.Modes, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}

	rows, err = s.db.Query(`SELECT substr(created_at,1,10), COUNT(*) FROM players
		WHERE created_at >= date('now','-13 days') GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			rows.Close()
			return o, err
		}
		o.Signups = append(o.Signups, d)
	}
	rows.Close()
	return o, rows.Err()
}

// --- player administration ---

type AdminPlayerRow struct {
	ID           string
	Nickname     string
	Email        string
	Picture      string
	IsGuest      bool
	Banned       bool
	BanReason    string
	TotalExp     int64
	HandsSolved  int64
	HandsSkipped int64
	CreatedAt    time.Time
	LastSeen     time.Time
}

const adminPlayerSelect = `SELECT p.id, p.nickname_enc, p.email_enc, p.picture_enc, p.is_guest, p.banned, p.ban_reason,
	p.total_exp, p.created_at, p.last_seen_at,
	COALESCE(a.solved, 0), COALESCE(a.skipped, 0)
FROM players p
LEFT JOIN (SELECT player_id, SUM(hands_solved) solved, SUM(hands_skipped) skipped
	FROM player_mode_stats GROUP BY player_id) a ON a.player_id = p.id`

func (s *Store) scanAdminPlayer(row interface{ Scan(...any) error }) (AdminPlayerRow, error) {
	var (
		r                 AdminPlayerRow
		guest, banned     int
		nickEnc, emailEnc string
		picEnc            string
		createdAt, seenAt string
	)
	if err := row.Scan(&r.ID, &nickEnc, &emailEnc, &picEnc, &guest, &banned, &r.BanReason,
		&r.TotalExp, &createdAt, &seenAt, &r.HandsSolved, &r.HandsSkipped); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return r, ErrNotFound
		}
		return r, err
	}
	r.IsGuest = guest == 1
	r.Banned = banned == 1
	r.CreatedAt = parseDBTime(createdAt)
	r.LastSeen = parseDBTime(seenAt)
	var err error
	if r.Nickname, err = s.cr.Decrypt(nickEnc, "players.nickname"); err != nil {
		return r, err
	}
	if r.Email, err = s.cr.Decrypt(emailEnc, "players.email"); err != nil {
		return r, err
	}
	if r.Picture, err = s.cr.Decrypt(picEnc, "players.picture"); err != nil {
		return r, err
	}
	return r, nil
}

// adminPlayerFilter maps a UI filter to SQL; only plaintext columns filter
// in SQL, everything else (nickname/email search) is applied in Go.
func adminPlayerFilter(filter string) (string, error) {
	switch filter {
	case "", "all":
		return "1=1", nil
	case "guests":
		return "p.is_guest = 1", nil
	case "google":
		return "p.is_guest = 0", nil
	case "banned":
		return "p.banned = 1", nil
	default:
		return "", fmt.Errorf("store: unknown player filter %q", filter)
	}
}

// AdminListPlayers returns one page of players newest-first. Search matches
// player id, nickname or email (contains, case-insensitive); because those
// fields are encrypted the match runs in Go over at most maxPlayerScan rows.
func (s *Store) AdminListPlayers(search, filter string, limit, offset int) ([]AdminPlayerRow, int, error) {
	limit, offset = clampPage(limit, offset)
	where, err := adminPlayerFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	search = strings.ToLower(strings.TrimSpace(search))

	if search == "" {
		var total int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM players p WHERE ` + where).Scan(&total); err != nil {
			return nil, 0, err
		}
		rows, err := s.db.Query(adminPlayerSelect+` WHERE `+where+` ORDER BY p.created_at DESC, p.id DESC LIMIT ? OFFSET ?`,
			limit, offset)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		var out []AdminPlayerRow
		for rows.Next() {
			r, err := s.scanAdminPlayer(rows)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, r)
		}
		return out, total, rows.Err()
	}

	// search path: decrypt + filter in memory, then slice the page
	rows, err := s.db.Query(adminPlayerSelect + ` WHERE ` + where + ` ORDER BY p.created_at DESC, p.id DESC LIMIT ` +
		fmt.Sprint(maxPlayerScan))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var matched []AdminPlayerRow
	for rows.Next() {
		r, err := s.scanAdminPlayer(rows)
		if err != nil {
			return nil, 0, err
		}
		if strings.Contains(strings.ToLower(r.Nickname), search) ||
			strings.Contains(strings.ToLower(r.Email), search) ||
			strings.HasPrefix(strings.ToLower(r.ID), search) {
			matched = append(matched, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	total := len(matched)
	if offset >= total {
		return []AdminPlayerRow{}, total, nil
	}
	page := matched[offset:]
	if len(page) > limit {
		page = page[:limit]
	}
	return page, total, nil
}

// SetPlayerBanned bans or unbans a player. Banning revokes access on the
// player's next authenticated request (token or Google sign-in). actor names
// the admin in the event log ("admin:<player id>").
func (s *Store) SetPlayerBanned(actor, playerID string, banned bool, reason string) (Player, error) {
	if banned {
		_, err := s.db.Exec(`UPDATE players SET banned = 1, banned_at = datetime('now'), ban_reason = ? WHERE id = ?`,
			reason, playerID)
		if err != nil {
			return Player{}, err
		}
		s.LogEvent(actor, "admin.ban", playerID, mustJSON(map[string]string{"reason": reason}))
	} else {
		_, err := s.db.Exec(`UPDATE players SET banned = 0, banned_at = NULL, ban_reason = '' WHERE id = ?`, playerID)
		if err != nil {
			return Player{}, err
		}
		s.LogEvent(actor, "admin.unban", playerID, "")
	}
	p, err := s.PlayerByID(playerID)
	if errors.Is(err, ErrNotFound) {
		return Player{}, ErrNotFound
	}
	return p, err
}

// DeletePlayer removes a player and, through cascades, all of their mode
// stats and rounds. The event log keeps a non-PII record of the deletion.
func (s *Store) DeletePlayer(actor, playerID string) error {
	p, err := s.PlayerByID(playerID)
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM players WHERE id = ?`, playerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.LogEvent(actor, "admin.delete", playerID,
		mustJSON(map[string]any{"wasGuest": p.IsGuest, "totalExp": p.TotalExp}))
	return nil
}

// AdjustPlayerEXP applies an EXP delta (possibly negative) to one mode,
// clamped at zero, then recomputes total_exp as the sum of all mode EXP so
// the level/total ledger never desyncs from the per-mode leaderboards.
func (s *Store) AdjustPlayerEXP(actor, playerID, mode string, delta int64) (Player, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Player{}, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT OR IGNORE INTO player_mode_stats (player_id, mode) VALUES (?,?)`, playerID, mode); err != nil {
		return Player{}, err
	}
	var before int64
	if err := tx.QueryRow(`SELECT exp FROM player_mode_stats WHERE player_id = ? AND mode = ?`, playerID, mode).Scan(&before); err != nil {
		return Player{}, err
	}
	after := before + delta
	if after < 0 {
		after = 0
	}
	if _, err := tx.Exec(`UPDATE player_mode_stats SET exp = ? WHERE player_id = ? AND mode = ?`, after, playerID, mode); err != nil {
		return Player{}, err
	}
	if err := syncTotalExp(tx, playerID); err != nil {
		return Player{}, err
	}
	if err := logEventTx(tx, actor, "admin.exp.adjust", playerID,
		mustJSON(map[string]any{"mode": mode, "delta": delta, "before": before, "after": after})); err != nil {
		return Player{}, err
	}
	if err := tx.Commit(); err != nil {
		return Player{}, err
	}
	return s.PlayerByID(playerID)
}

// ResetPlayerStats wipes per-mode stats for one mode (empty mode = all
// modes) and resyncs the player's total EXP.
func (s *Store) ResetPlayerStats(actor, playerID, mode string) (Player, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Player{}, err
	}
	defer tx.Rollback()

	if mode == "" {
		if _, err := tx.Exec(`DELETE FROM player_mode_stats WHERE player_id = ?`, playerID); err != nil {
			return Player{}, err
		}
	} else {
		res, err := tx.Exec(`DELETE FROM player_mode_stats WHERE player_id = ? AND mode = ?`, playerID, mode)
		if err != nil {
			return Player{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Player{}, ErrNotFound
		}
	}
	if err := syncTotalExp(tx, playerID); err != nil {
		return Player{}, err
	}
	label := mode
	if label == "" {
		label = "all"
	}
	if err := logEventTx(tx, actor, "admin.stats.reset", playerID, mustJSON(map[string]string{"mode": label})); err != nil {
		return Player{}, err
	}
	if err := tx.Commit(); err != nil {
		return Player{}, err
	}
	return s.PlayerByID(playerID)
}

func syncTotalExp(tx *sql.Tx, playerID string) error {
	res, err := tx.Exec(`UPDATE players SET total_exp =
		(SELECT COALESCE(SUM(exp), 0) FROM player_mode_stats WHERE player_id = ?) WHERE id = ?`, playerID, playerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- full leaderboard (admin view: banned and guest rows visible) ---

type AdminLeaderRow struct {
	LeaderRow
	IsGuest bool
	Banned  bool
}

func (s *Store) AdminLeaderboard(mode string, limit, offset int) ([]AdminLeaderRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT p.id, p.nickname_enc, p.picture_enc, m.exp, m.hands_solved, m.best_streak, p.is_guest, p.banned
		FROM player_mode_stats m JOIN players p ON p.id = m.player_id
		WHERE m.mode = ?
		ORDER BY m.exp DESC, m.hands_solved DESC, m.best_streak DESC
		LIMIT ? OFFSET ?`, mode, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminLeaderRow
	for rows.Next() {
		var (
			r              AdminLeaderRow
			nickEnc        string
			picEnc         string
			guest, bannedv int
		)
		if err := rows.Scan(&r.PlayerID, &nickEnc, &picEnc, &r.Exp, &r.HandsSolved, &r.BestStreak, &guest, &bannedv); err != nil {
			return nil, err
		}
		r.IsGuest = guest == 1
		r.Banned = bannedv == 1
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

// --- round listings ---

type AdminRoundRow struct {
	ID         string
	PlayerID   string
	Nickname   string
	Mode       string
	Status     string
	Points     int64
	ElapsedMs  int64
	HintsUsed  int64
	DealtAt    time.Time
	FinishedAt *time.Time
}

const adminRoundCols = `r.id, r.player_id, p.nickname_enc, r.mode, r.status, r.points, r.elapsed_ms, r.hints_used, r.dealt_at, r.finished_at`
const adminRoundFrom = ` FROM rounds r LEFT JOIN players p ON p.id = r.player_id `

func (s *Store) scanAdminRound(rows *sql.Rows) (AdminRoundRow, error) {
	var (
		r                 AdminRoundRow
		nickEnc           string
		dealtAt, finished sql.NullString
	)
	if err := rows.Scan(&r.ID, &r.PlayerID, &nickEnc, &r.Mode, &r.Status, &r.Points,
		&r.ElapsedMs, &r.HintsUsed, &dealtAt, &finished); err != nil {
		return r, err
	}
	var err error
	if r.Nickname, err = s.cr.Decrypt(nickEnc, "players.nickname"); err != nil {
		return r, err
	}
	if dealtAt.Valid {
		r.DealtAt = parseDBTime(dealtAt.String)
	}
	if finished.Valid && finished.String != "" {
		t := parseDBTime(finished.String)
		r.FinishedAt = &t
	}
	return r, nil
}

// AdminListRounds returns the newest rounds across all players, optionally
// filtered by mode and/or status, with the total count for pagination.
func (s *Store) AdminListRounds(mode, status string, limit, offset int) ([]AdminRoundRow, int, error) {
	limit, offset = clampPage(limit, offset)
	where, args := "1=1", []any{}
	if mode != "" {
		where += " AND r.mode = ?"
		args = append(args, mode)
	}
	if status != "" {
		where += " AND r.status = ?"
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*)`+adminRoundFrom+`WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT `+adminRoundCols+adminRoundFrom+`WHERE `+where+
		` ORDER BY r.dealt_at DESC, r.id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminRoundRow
	for rows.Next() {
		r, err := s.scanAdminRound(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// AdminPlayerRounds returns one player's rounds newest-first.
func (s *Store) AdminPlayerRounds(playerID string, limit, offset int) ([]AdminRoundRow, int, error) {
	limit, offset = clampPage(limit, offset)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM rounds WHERE player_id = ?`, playerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT `+adminRoundCols+adminRoundFrom+`WHERE r.player_id = ?
		ORDER BY r.dealt_at DESC, r.id DESC LIMIT ? OFFSET ?`, playerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminRoundRow
	for rows.Next() {
		r, err := s.scanAdminRound(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// --- database backup & reset ---

// BackupTo writes a consistent standalone snapshot of the database to path
// (which must not exist yet) using SQLite's VACUUM INTO — the image includes
// everything currently in the WAL, so it is safe while the service runs.
func (s *Store) BackupTo(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("store: backup target already exists: %s", path)
	}
	if _, err := s.db.Exec(`VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("store: backup: %w", err)
	}
	return nil
}

// ResetStats reports what a reset wiped.
type ResetStats struct {
	Players int64
	Rounds  int64
	Events  int64
}

// Reset drops every table and recreates the schema, returning a fresh empty
// database. A snapshot is written to backupPath first (pass "" to skip —
// the admin API never does), and the fresh event log records who reset what.
// All sessions die with the data: every token, including the acting
// admin's, stops working immediately after.
func (s *Store) Reset(actor, backupPath string) (ResetStats, error) {
	var st ResetStats
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&st.Players); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM rounds`).Scan(&st.Rounds); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM event_logs`).Scan(&st.Events); err != nil {
		return st, err
	}
	if backupPath != "" {
		if err := s.BackupTo(backupPath); err != nil {
			return st, err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return st, err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DROP TABLE IF EXISTS event_logs`,
		`DROP TABLE IF EXISTS rounds`,
		`DROP TABLE IF EXISTS player_mode_stats`,
		`DROP TABLE IF EXISTS players`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return st, fmt.Errorf("store: reset: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return st, err
	}
	if err := s.migrate(); err != nil {
		return st, err
	}
	// reclaim the space and log the reset into the fresh database
	if _, err := s.db.Exec(`VACUUM`); err != nil {
		return st, fmt.Errorf("store: reset vacuum: %w", err)
	}
	s.LogEvent("system", "db.reset", "", mustJSON(map[string]any{
		"actor": actor, "wipedPlayers": st.Players, "wipedRounds": st.Rounds, "wipedEvents": st.Events,
	}))
	return st, nil
}

// QueryCounts fills the table row counts shown on the Settings page. Errors
// leave the counter at zero — these are informational only.
func (s *Store) QueryCounts(players, rounds, events *int64) {
	s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(players)
	s.db.QueryRow(`SELECT COUNT(*) FROM rounds`).Scan(rounds)
	s.db.QueryRow(`SELECT COUNT(*) FROM event_logs`).Scan(events)
}
