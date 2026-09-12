// Package store persists players, mode stats and rounds in SQLite. All
// player PII is encrypted through the Crypter before hitting the database;
// only gameplay statistics stay queryable.
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	cr Crypter
}

// Crypter is the subset of crypto.Crypter the store needs.
type Crypter interface {
	Encrypt(plaintext, purpose string) (string, error)
	Decrypt(ciphertext, purpose string) (string, error)
	SubHash(value string) string
	HashToken(value string) string
}

func Open(path string, cr Crypter) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// modernc sqlite is happiest with limited concurrency; serialize writes.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, cr: cr}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SetCrypter attaches the crypter after opening (useful when the crypter is
// built later than the store, e.g. in tests).
func (s *Store) SetCrypter(cr Crypter) { s.cr = cr }

// TryUseHint atomically increments the hint counter of an open round and
// returns the new count.
func (s *Store) TryUseHint(roundID, playerID string) (int, error) {
	var used int
	err := s.db.QueryRow(`UPDATE rounds SET hints_used = hints_used + 1
		WHERE id = ? AND player_id = ? AND status = 'open'
		RETURNING hints_used`, roundID, playerID).Scan(&used)
	return used, err
}

// migrateLegacyExpBoost carries the single-kind exp_boost row of installs
// from before the coins boost existed into the boosts table (as the "exp"
// kind) and drops the old table. No-op when exp_boost is absent.
func (s *Store) migrateLegacyExpBoost() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'exp_boost'`).Scan(&n); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	if n == 0 {
		return nil
	}
	if _, err := s.db.Exec(`ALTER TABLE exp_boost RENAME TO exp_boost_legacy`); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	// REPLACE, not IGNORE: the seeds for kind rows have already run by the
	// time we get here, and the carried-over config must win over them
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO boosts (id, kind, multiplier, duration_min, label, enabled, starts_at, ends_at, updated_by, updated_at)
		SELECT id, 'exp', multiplier, duration_min, label, enabled, starts_at, ends_at, updated_by, updated_at FROM exp_boost_legacy`); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	if _, err := s.db.Exec(`DROP TABLE exp_boost_legacy`); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS players (
			id           TEXT PRIMARY KEY,
			token_hash   TEXT UNIQUE NOT NULL,
			token_enc    TEXT NOT NULL,
			nickname_enc TEXT NOT NULL,
			email_enc    TEXT NOT NULL DEFAULT '',
			sub_hash     TEXT UNIQUE,
			gsub_enc     TEXT NOT NULL DEFAULT '',
			picture_enc  TEXT NOT NULL DEFAULT '',
			is_guest     INTEGER NOT NULL DEFAULT 1,
			total_exp    INTEGER NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL DEFAULT (datetime('now')),
			last_seen_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS player_mode_stats (
			player_id      TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			mode           TEXT NOT NULL,
			exp            INTEGER NOT NULL DEFAULT 0,
			hands_solved   INTEGER NOT NULL DEFAULT 0,
			hands_skipped  INTEGER NOT NULL DEFAULT 0,
			best_streak    INTEGER NOT NULL DEFAULT 0,
			current_streak INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (player_id, mode)
		)`,
		`CREATE TABLE IF NOT EXISTS rounds (
			id          TEXT PRIMARY KEY,
			player_id   TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			mode        TEXT NOT NULL,
			session_id  TEXT NOT NULL DEFAULT '',
			numbers     TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'open',
			points      INTEGER NOT NULL DEFAULT 0,
			elapsed_ms  INTEGER NOT NULL DEFAULT 0,
			hints_used  INTEGER NOT NULL DEFAULT 0,
			skip_used   INTEGER NOT NULL DEFAULT 0,
			time_limit  INTEGER NOT NULL DEFAULT 0,
			dealt_at    TEXT NOT NULL DEFAULT (datetime('now')),
			finished_at TEXT
		)`,
		// gameplay score ledger: one row per solved hand with its raw points.
		// Score is a metric of its own — separate from EXP/coins and never
		// multiplied by payout boosts — so the leaderboard ranks pure play.
		// The awarded_at timestamp is what weekly windows aggregate over.
		`CREATE TABLE IF NOT EXISTS player_scores (
			id         INTEGER PRIMARY KEY,
			player_id  TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			mode       TEXT NOT NULL,
			points     INTEGER NOT NULL,
			awarded_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS event_logs (
		id     TEXT PRIMARY KEY,
		ts     TEXT NOT NULL DEFAULT (datetime('now')),
		actor  TEXT NOT NULL,
		action TEXT NOT NULL,
		target TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT ''
	)`,
		// server-wide boosts: one row per kind (EXP and coin payouts), an
		// admin-authored draft that can be armed later
		`CREATE TABLE IF NOT EXISTS boosts (
			id           INTEGER PRIMARY KEY,
			kind         TEXT NOT NULL UNIQUE,
			multiplier   REAL NOT NULL DEFAULT 2,
			duration_min INTEGER NOT NULL DEFAULT 60,
			label        TEXT NOT NULL DEFAULT '',
			enabled      INTEGER NOT NULL DEFAULT 0,
			starts_at    TEXT,
			ends_at      TEXT,
			updated_by   TEXT NOT NULL DEFAULT '',
			updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`INSERT OR IGNORE INTO boosts (id, kind) VALUES (1, 'exp')`,
		`INSERT OR IGNORE INTO boosts (id, kind) VALUES (2, 'coins')`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	// Column additions for databases created before the admin portal: ALTER
	// TABLE has no IF NOT EXISTS for columns, so ignore duplicate-column
	// errors and fail on anything else. These must run before index creation
	// because indexes may reference the added columns.
	alters := []string{
		`ALTER TABLE players ADD COLUMN banned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE players ADD COLUMN banned_at TEXT`,
		`ALTER TABLE players ADD COLUMN ban_reason TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE rounds ADD COLUMN session_id TEXT NOT NULL DEFAULT ''`,
		// seconds one time-extension item added to the hand's countdown
		// (0 = the round never got extended)
		`ALTER TABLE rounds ADD COLUMN extend_sec INTEGER NOT NULL DEFAULT 0`,
		// 1 = the hand was folded with a skip-pass item (the once-per-session
		// skip rule keys off this flag)
		`ALTER TABLE rounds ADD COLUMN skip_used INTEGER NOT NULL DEFAULT 0`,
		// the hand's base countdown in seconds (0 = use the mode default).
		// Solo ladder rounds carry their stage-shrunk window here
		`ALTER TABLE rounds ADD COLUMN time_limit INTEGER NOT NULL DEFAULT 0`,
		// coin economy + achievements + skins
		`ALTER TABLE players ADD COLUMN total_coins INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE players ADD COLUMN total_wins INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE players ADD COLUMN skin TEXT NOT NULL DEFAULT ''`,
		// admin tier override ('' = derive the tier from the level)
		`ALTER TABLE players ADD COLUMN tier TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range alters {
		if _, err := s.db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("store: migrate: %w", err)
			}
		}
	}
	if err := s.migrateLegacyExpBoost(); err != nil {
		return err
	}
	// Unlocked achievements and owned card skins live in their own tables so
	// both the catalog views and the grant transactions stay single-purpose.
	shopTables := []string{
		`CREATE TABLE IF NOT EXISTS player_achievements (
			player_id      TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			achievement_id TEXT NOT NULL,
			unlocked_at    TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (player_id, achievement_id)
		)`,
		`CREATE TABLE IF NOT EXISTS player_skins (
			player_id   TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			skin_id     TEXT NOT NULL,
			acquired_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (player_id, skin_id)
		)`,
		// consumable boost items: one stacked qty per catalog item
		`CREATE TABLE IF NOT EXISTS player_items (
			player_id TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			item_id   TEXT NOT NULL,
			qty       INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (player_id, item_id)
		)`,
		// personal boost windows armed by using an item (one per kind — a
		// kind holds a single window: activating another item of the kind
		// replaces it; a window simply expires when ends_at passes)
		`CREATE TABLE IF NOT EXISTS player_boosts (
			player_id  TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			kind       TEXT NOT NULL,
			multiplier REAL NOT NULL,
			ends_at    TEXT,
			item_id    TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (player_id, kind)
		)`,
	}
	for _, q := range shopTables {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	// column additions for the shop tables themselves — they can only run
	// once the tables exist (fresh installs already get them from the CREATE
	// above and swallow the duplicate-column error)
	for _, q := range []string{
		// which item armed a personal boost ('' = predates item tracking)
		`ALTER TABLE player_boosts ADD COLUMN item_id TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("store: migrate: %w", err)
			}
		}
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_pms_board ON player_mode_stats(mode, exp DESC, hands_solved DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_scores_board ON player_scores(mode, player_id, points)`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts ON event_logs(ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_action ON event_logs(action, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rounds_dealt ON rounds(dealt_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rounds_session ON rounds(player_id, session_id)`,
	}
	for _, q := range indexes {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	return nil
}

func randomID(n int) string {
	const hexDigits = "0123456789abcdef"
	b := make([]byte, n)
	if err := randRead(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	out := make([]byte, n*2)
	for i, v := range b {
		out[i*2] = hexDigits[v>>4]
		out[i*2+1] = hexDigits[v&0x0f]
	}
	return string(out)
}
