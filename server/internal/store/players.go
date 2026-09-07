package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("store: not found")

// ErrBanned is returned when authenticating a banned player (token or Google
// sign-in); banned players are locked out of every authenticated endpoint.
var ErrBanned = errors.New("store: player banned")

type ModeStat struct {
	Exp           int64
	HandsSolved   int64
	HandsSkipped  int64
	BestStreak    int64
	CurrentStreak int64
}

type Player struct {
	ID        string
	Nickname  string
	Email     string
	Picture   string
	IsGuest   bool
	Banned    bool
	BanReason string
	TotalExp  int64
	CreatedAt time.Time
	LastSeen  time.Time
	Stats     map[string]ModeStat
}

func newToken() string { return randomID(32) }

// CreateGuest registers an anonymous player with a nickname.
func (s *Store) CreateGuest(nickname string) (Player, string, error) {
	id, token := randomID(16), newToken()
	nickEnc, err := s.cr.Encrypt(nickname, "players.nickname")
	if err != nil {
		return Player{}, "", err
	}
	tokEnc, err := s.cr.Encrypt(token, "players.token")
	if err != nil {
		return Player{}, "", err
	}
	_, err = s.db.Exec(`INSERT INTO players (id, token_hash, token_enc, nickname_enc, is_guest) VALUES (?,?,?,?,1)`,
		id, s.cr.HashToken(token), tokEnc, nickEnc)
	if err != nil {
		return Player{}, "", fmt.Errorf("store: create guest: %w", err)
	}
	s.LogEvent("system", "player.create", id, `{"kind":"guest"}`)
	p := Player{ID: id, Nickname: nickname, IsGuest: true, Stats: map[string]ModeStat{}}
	return p, token, nil
}

// GoogleLogin returns the player for a Google account, creating it on first
// sign-in. A fresh session token is issued on every login.
func (s *Store) GoogleLogin(sub, email, name, picture string) (Player, string, error) {
	subHash := s.cr.SubHash(sub)
	token := newToken()
	tokEnc, err := s.cr.Encrypt(token, "players.token")
	if err != nil {
		return Player{}, "", err
	}
	var (
		nickEnc  string
		emailEnc string
		picEnc   string
		subEnc   string
	)
	if nickEnc, err = s.cr.Encrypt(name, "players.nickname"); err != nil {
		return Player{}, "", err
	}
	if emailEnc, err = s.cr.Encrypt(email, "players.email"); err != nil {
		return Player{}, "", err
	}
	if picEnc, err = s.cr.Encrypt(picture, "players.picture"); err != nil {
		return Player{}, "", err
	}
	if subEnc, err = s.cr.Encrypt(sub, "players.google_sub"); err != nil {
		return Player{}, "", err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Player{}, "", err
	}
	defer tx.Rollback()

	var (
		id      string
		banned  bool
		existed bool
	)
	err = tx.QueryRow(`SELECT id, banned FROM players WHERE sub_hash = ?`, subHash).Scan(&id, &banned)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = randomID(16)
		if _, err = tx.Exec(`INSERT INTO players
			(id, token_hash, token_enc, nickname_enc, email_enc, sub_hash, gsub_enc, picture_enc, is_guest)
			VALUES (?,?,?,?,?,?,?,?,0)`,
			id, s.cr.HashToken(token), tokEnc, nickEnc, emailEnc, subHash, subEnc, picEnc); err != nil {
			return Player{}, "", fmt.Errorf("store: create google player: %w", err)
		}
		if err = logEventTx(tx, "system", "player.create", id, `{"kind":"google"}`); err != nil {
			return Player{}, "", err
		}
	case err != nil:
		return Player{}, "", err
	default:
		existed = true
		if banned {
			return Player{}, "", ErrBanned
		}
		if _, err = tx.Exec(`UPDATE players SET token_hash=?, token_enc=?, nickname_enc=?, email_enc=?, picture_enc=?, last_seen_at=datetime('now') WHERE id=?`,
			s.cr.HashToken(token), tokEnc, nickEnc, emailEnc, picEnc, id); err != nil {
			return Player{}, "", err
		}
	}
	if err = tx.Commit(); err != nil {
		return Player{}, "", err
	}
	if existed {
		s.LogEvent("system", "player.login.google", id, "")
	}
	p, err := s.PlayerByToken(token)
	if err != nil {
		return Player{}, "", err
	}
	return p, token, nil
}

const playerCols = `id, total_exp, is_guest, nickname_enc, email_enc, picture_enc, banned, ban_reason, created_at, last_seen_at`

func (s *Store) scanPlayer(row interface{ Scan(...any) error }) (Player, error) {
	var (
		p                 Player
		guest, banned     int
		nickEnc, emailEnc string
		picEnc            string
		createdAt, seenAt string
	)
	if err := row.Scan(&p.ID, &p.TotalExp, &guest, &nickEnc, &emailEnc, &picEnc,
		&banned, &p.BanReason, &createdAt, &seenAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}
	p.IsGuest = guest == 1
	p.Banned = banned == 1
	p.CreatedAt = parseDBTime(createdAt)
	p.LastSeen = parseDBTime(seenAt)
	var err error
	if p.Nickname, err = s.cr.Decrypt(nickEnc, "players.nickname"); err != nil {
		return p, err
	}
	if p.Email, err = s.cr.Decrypt(emailEnc, "players.email"); err != nil {
		return p, err
	}
	if p.Picture, err = s.cr.Decrypt(picEnc, "players.picture"); err != nil {
		return p, err
	}
	return p, nil
}

// parseDBTime reads SQLite datetime('now') strings (UTC). A zero time is
// returned for anything unparsable rather than failing the whole load.
func parseDBTime(v string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", v)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// PlayerByToken authenticates a session token (constant-work lookup by hash).
// Banned players authenticate as ErrBanned regardless of token validity.
func (s *Store) PlayerByToken(token string) (Player, error) {
	row := s.db.QueryRow(`SELECT `+playerCols+` FROM players WHERE token_hash = ?`, s.cr.HashToken(token))
	p, err := s.scanPlayer(row)
	if err != nil {
		return p, err
	}
	if p.Banned {
		return p, ErrBanned
	}
	s.db.Exec(`UPDATE players SET last_seen_at = datetime('now') WHERE id = ?`, p.ID)
	if p.Stats, err = s.ModeStats(p.ID); err != nil {
		return p, err
	}
	return p, nil
}

// PlayerByID loads a player for public views (leaderboard entries reuse rows).
func (s *Store) PlayerByID(id string) (Player, error) {
	row := s.db.QueryRow(`SELECT `+playerCols+` FROM players WHERE id = ?`, id)
	p, err := s.scanPlayer(row)
	if err != nil {
		return p, err
	}
	if p.Stats, err = s.ModeStats(p.ID); err != nil {
		return p, err
	}
	return p, nil
}

// RenamePlayer updates a player's nickname (guests and Google players alike).
// actor records who performed the rename in the event log ("player:<id>" or
// "admin").
func (s *Store) RenamePlayer(actor, playerID, nickname string) (Player, error) {
	nickEnc, err := s.cr.Encrypt(nickname, "players.nickname")
	if err != nil {
		return Player{}, err
	}
	res, err := s.db.Exec(`UPDATE players SET nickname_enc = ? WHERE id = ?`, nickEnc, playerID)
	if err != nil {
		return Player{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Player{}, ErrNotFound
	}
	s.LogEvent(actor, "player.rename", playerID, "")
	return s.PlayerByID(playerID)
}

func (s *Store) ModeStats(playerID string) (map[string]ModeStat, error) {
	rows, err := s.db.Query(`SELECT mode, exp, hands_solved, hands_skipped, best_streak, current_streak
		FROM player_mode_stats WHERE player_id = ?`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := map[string]ModeStat{}
	for rows.Next() {
		var (
			mode string
			st   ModeStat
		)
		if err := rows.Scan(&mode, &st.Exp, &st.HandsSolved, &st.HandsSkipped, &st.BestStreak, &st.CurrentStreak); err != nil {
			return nil, err
		}
		stats[mode] = st
	}
	return stats, rows.Err()
}

// AwardEXP updates both ledgers in one transaction: per-mode stats for the
// leaderboard and the player's total EXP for level/tier. solved=false records
// a skip (resets streak, awards nothing).
func (s *Store) AwardEXP(playerID, mode string, points int64, solved bool) (ModeStat, int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ModeStat{}, 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT OR IGNORE INTO player_mode_stats (player_id, mode) VALUES (?,?)`, playerID, mode); err != nil {
		return ModeStat{}, 0, err
	}
	if solved {
		if _, err = tx.Exec(`UPDATE player_mode_stats SET
			exp = exp + ?,
			hands_solved = hands_solved + 1,
			current_streak = current_streak + 1,
			best_streak = MAX(best_streak, current_streak + 1)
			WHERE player_id = ? AND mode = ?`, points, playerID, mode); err != nil {
			return ModeStat{}, 0, err
		}
		if _, err = tx.Exec(`UPDATE players SET total_exp = total_exp + ? WHERE id = ?`, points, playerID); err != nil {
			return ModeStat{}, 0, err
		}
	} else {
		if _, err = tx.Exec(`UPDATE player_mode_stats SET
			hands_skipped = hands_skipped + 1,
			current_streak = 0
			WHERE player_id = ? AND mode = ?`, playerID, mode); err != nil {
			return ModeStat{}, 0, err
		}
	}
	var st ModeStat
	var total int64
	if err = tx.QueryRow(`SELECT exp, hands_solved, hands_skipped, best_streak, current_streak
		FROM player_mode_stats WHERE player_id = ? AND mode = ?`, playerID, mode).
		Scan(&st.Exp, &st.HandsSolved, &st.HandsSkipped, &st.BestStreak, &st.CurrentStreak); err != nil {
		return ModeStat{}, 0, err
	}
	if err = tx.QueryRow(`SELECT total_exp FROM players WHERE id = ?`, playerID).Scan(&total); err != nil {
		return ModeStat{}, 0, err
	}
	if err = tx.Commit(); err != nil {
		return ModeStat{}, 0, err
	}
	return st, total, nil
}

type LeaderRow struct {
	PlayerID    string
	Nickname    string
	Picture     string
	Exp         int64
	HandsSolved int64
	BestStreak  int64
}

// Leaderboard returns the ranked list for one mode; guests and banned
// players never appear.
func (s *Store) Leaderboard(mode string, limit, offset int) ([]LeaderRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT p.id, p.nickname_enc, p.picture_enc, m.exp, m.hands_solved, m.best_streak
		FROM player_mode_stats m JOIN players p ON p.id = m.player_id
		WHERE m.mode = ? AND p.is_guest = 0 AND p.banned = 0
		ORDER BY m.exp DESC, m.hands_solved DESC, m.best_streak DESC
		LIMIT ? OFFSET ?`, mode, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderRow
	for rows.Next() {
		var (
			r       LeaderRow
			nickEnc string
			picEnc  string
		)
		if err := rows.Scan(&r.PlayerID, &nickEnc, &picEnc, &r.Exp, &r.HandsSolved, &r.BestStreak); err != nil {
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

// --- single-player rounds ---

type Round struct {
	ID        string
	PlayerID  string
	Mode      string
	SessionID string
	Numbers   []int
	Status    string
	Points    int64
	ElapsedMs int64
	HintsUsed int64
	DealtAt   time.Time
}

// CreateRound deals a hand into a new round. sessionID groups consecutive
// hands of one play session, so hint budgets can span rounds.
func (s *Store) CreateRound(playerID, mode string, numbers []int, sessionID string) (Round, error) {
	id := randomID(16)
	numsJSON, err := json.Marshal(numbers)
	if err != nil {
		return Round{}, err
	}
	if _, err := s.db.Exec(`INSERT INTO rounds (id, player_id, mode, session_id, numbers) VALUES (?,?,?,?,?)`,
		id, playerID, mode, sessionID, string(numsJSON)); err != nil {
		return Round{}, err
	}
	return s.RoundByID(id, playerID)
}

func (s *Store) RoundByID(id, playerID string) (Round, error) {
	var (
		r          Round
		numsJSON   string
		dealtAtStr string
	)
	err := s.db.QueryRow(`SELECT id, player_id, mode, session_id, numbers, status, points, elapsed_ms, hints_used, dealt_at
		FROM rounds WHERE id = ? AND player_id = ?`, id, playerID).
		Scan(&r.ID, &r.PlayerID, &r.Mode, &r.SessionID, &numsJSON, &r.Status, &r.Points, &r.ElapsedMs, &r.HintsUsed, &dealtAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal([]byte(numsJSON), &r.Numbers); err != nil {
		return r, err
	}
	if r.DealtAt, err = time.Parse("2006-01-02 15:04:05", dealtAtStr); err != nil {
		return r, err
	}
	return r, nil
}

// HintsUsedInSession sums the hint counters of every round the player has
// played under one session id.
func (s *Store) HintsUsedInSession(playerID, sessionID string) (int64, error) {
	var used int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(hints_used), 0) FROM rounds
		WHERE player_id = ? AND session_id = ?`, playerID, sessionID).Scan(&used)
	return used, err
}

func (s *Store) FinishRound(id, status string, points, elapsedMs int64) error {
	if _, err := s.db.Exec(`UPDATE rounds SET status=?, points=?, elapsed_ms=?, finished_at=datetime('now') WHERE id=?`,
		status, points, elapsedMs, id); err != nil {
		return err
	}
	return nil
}
