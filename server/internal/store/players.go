package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pattaradol9/game24/server/internal/progress"
)

var ErrNotFound = errors.New("store: not found")

// ErrBanned is returned when authenticating a banned player (token or Google
// sign-in); banned players are locked out of every authenticated endpoint.
var ErrBanned = errors.New("store: player banned")

// Skin shop errors; the handler maps them to status codes and messages.
var (
	ErrGoogleRequired    = errors.New("store: google sign-in required")
	ErrInsufficientCoins = errors.New("store: insufficient coins")
	ErrAlreadyOwned      = errors.New("store: skin already owned")
	ErrNotOwned          = errors.New("store: skin not owned")
)

type ModeStat struct {
	Exp           int64
	HandsSolved   int64
	HandsSkipped  int64
	BestStreak    int64
	CurrentStreak int64
}

type Player struct {
	ID         string
	Nickname   string
	Email      string
	Picture    string
	IsGuest    bool
	Banned     bool
	BanReason  string
	TotalExp   int64
	TotalCoins int64
	TotalWins  int64
	Skin       string // equipped card skin id ('' = classic)
	Tier       string // admin tier override ('' = derive from level)
	CreatedAt  time.Time
	LastSeen   time.Time
	Stats      map[string]ModeStat
}

// EffectiveTier returns the admin tier override when one is set, otherwise
// the tier derived from the player's level. Every tier consumer — display
// and the tier hint quota alike — goes through this.
func (p Player) EffectiveTier() int {
	if p.Tier != "" {
		if t, ok := progress.TierFromName(p.Tier); ok {
			return t
		}
	}
	return progress.TierFromLevel(progress.LevelFromExp(p.TotalExp))
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

const playerCols = `id, total_exp, total_coins, total_wins, skin, tier, is_guest, nickname_enc, email_enc, picture_enc, banned, ban_reason, created_at, last_seen_at`

func (s *Store) scanPlayer(row interface{ Scan(...any) error }) (Player, error) {
	var (
		p                 Player
		guest, banned     int
		nickEnc, emailEnc string
		picEnc            string
		createdAt, seenAt string
	)
	if err := row.Scan(&p.ID, &p.TotalExp, &p.TotalCoins, &p.TotalWins, &p.Skin, &p.Tier,
		&guest, &nickEnc, &emailEnc, &picEnc,
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

// deletePlayer removes the player row; every dependent table (mode stats,
// rounds, achievements, owned skins, item inventory, personal boost windows)
// cascades and the session token dies with the row. The event log keeps a
// non-PII record — the action name tells admin deletions from self-service
// ones.
func (s *Store) deletePlayer(actor, action, playerID string) error {
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
	s.LogEvent(actor, action, playerID,
		mustJSON(map[string]any{"wasGuest": p.IsGuest, "totalExp": p.TotalExp}))
	return nil
}

// DeleteOwnPlayer permanently removes the calling player's own account —
// the self-service path behind "delete my profile".
func (s *Store) DeleteOwnPlayer(playerID string) error {
	return s.deletePlayer("player:"+playerID, "player.delete", playerID)
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

// AwardEXP updates both ledgers in one transaction: per-mode stats and the
// player's total EXP for level/tier, plus the score ledger behind the
// leaderboard (raw points, boost-free). solved=false records
// a skip (resets streak, awards nothing). Solved hands also earn coins
// (CoinsForHand), mirroring how EXP tracks every finished hand. Live boosts
// multiply what the hand pays — server-wide campaigns and the player's own
// item boosts stack additively (a ×2 server boost plus a ×2 item pays ×3,
// never the compounded ×4).
func (s *Store) AwardEXP(playerID, mode string, points int64, solved bool) (ModeStat, int64, error) {
	st, total, _, err := s.awardEXP(playerID, mode, points, solved, false)
	return st, total, err
}

// handPayout is what one solved hand banks before achievement rewards:
// base amounts with every boost multiplier already applied.
type handPayout struct {
	ExpGain  int64
	CoinGain int64
}

func (s *Store) awardEXP(playerID, mode string, points int64, solved, roomWin bool) (ModeStat, int64, handPayout, error) {
	// the boost lookups must happen before the transaction opens: the pool
	// holds a single connection and a query inside the tx would deadlock.
	// A multiplayer round win ignores the player's own item boosts — room
	// payouts keep every seat equal — while server-wide boosts still count,
	// applying to everyone alike.
	personal := !roomWin
	payout := handPayout{CoinGain: CoinsForHand(points)}
	if solved {
		payout.ExpGain = BoostAmount(points, s.payoutMultiplier(playerID, BoostKindExp, personal))
		payout.CoinGain = BoostAmount(CoinsForHand(points), s.payoutMultiplier(playerID, BoostKindCoins, personal))
	}
	tx, err := s.db.Begin()
	if err != nil {
		return ModeStat{}, 0, payout, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT OR IGNORE INTO player_mode_stats (player_id, mode) VALUES (?,?)`, playerID, mode); err != nil {
		return ModeStat{}, 0, payout, err
	}
	if solved {
		if _, err = tx.Exec(`UPDATE player_mode_stats SET
			exp = exp + ?,
			hands_solved = hands_solved + 1,
			current_streak = current_streak + 1,
			best_streak = MAX(best_streak, current_streak + 1)
			WHERE player_id = ? AND mode = ?`, payout.ExpGain, playerID, mode); err != nil {
			return ModeStat{}, 0, payout, err
		}
		if _, err = tx.Exec(`UPDATE players SET
			total_exp = total_exp + ?,
			total_coins = total_coins + ?
			WHERE id = ?`, payout.ExpGain, payout.CoinGain, playerID); err != nil {
			return ModeStat{}, 0, payout, err
		}
		// the score ledger banks the RAW points — payout boosts inflate the
		// EXP/coin ledgers above, never the leaderboard score
		if err = recordScoreTx(tx, playerID, mode, points); err != nil {
			return ModeStat{}, 0, payout, err
		}
	} else {
		if _, err = tx.Exec(`UPDATE player_mode_stats SET
			hands_skipped = hands_skipped + 1,
			current_streak = 0
			WHERE player_id = ? AND mode = ?`, playerID, mode); err != nil {
			return ModeStat{}, 0, payout, err
		}
	}
	var st ModeStat
	var total int64
	if err = tx.QueryRow(`SELECT exp, hands_solved, hands_skipped, best_streak, current_streak
		FROM player_mode_stats WHERE player_id = ? AND mode = ?`, playerID, mode).
		Scan(&st.Exp, &st.HandsSolved, &st.HandsSkipped, &st.BestStreak, &st.CurrentStreak); err != nil {
		return ModeStat{}, 0, payout, err
	}
	if err = tx.QueryRow(`SELECT total_exp FROM players WHERE id = ?`, playerID).Scan(&total); err != nil {
		return ModeStat{}, 0, payout, err
	}
	if err = tx.Commit(); err != nil {
		return ModeStat{}, 0, payout, err
	}
	return st, total, payout, nil
}

type LeaderRow struct {
	PlayerID     string
	Nickname     string
	Picture      string
	Exp          int64
	HandsSolved  int64
	BestStreak   int64
	TierOverride string // admin tier override ('' = tier derives from exp)
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
	ExtendSec int64 // seconds a time-extension item added to this hand's countdown
	SkipUsed  int64 // 1 = the hand was folded with a skip-pass item
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
	err := s.db.QueryRow(`SELECT id, player_id, mode, session_id, numbers, status, points, elapsed_ms, hints_used, extend_sec, skip_used, dealt_at
		FROM rounds WHERE id = ? AND player_id = ?`, id, playerID).
		Scan(&r.ID, &r.PlayerID, &r.Mode, &r.SessionID, &numsJSON, &r.Status, &r.Points, &r.ElapsedMs, &r.HintsUsed, &r.ExtendSec, &r.SkipUsed, &dealtAtStr)
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

// ShiftRoundDealt moves a round's dealt_at by the signed duration — a seam
// for tests that exercise the expiry window without waiting real time.
func (s *Store) ShiftRoundDealt(roundID string, d time.Duration) error {
	_, err := s.db.Exec(`UPDATE rounds SET dealt_at = ? WHERE id = ?`,
		sqliteTime(time.Now().UTC().Add(d)), roundID)
	return err
}
