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
			dealt_at    TEXT NOT NULL DEFAULT (datetime('now')),
			finished_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS event_logs (
			id     TEXT PRIMARY KEY,
			ts     TEXT NOT NULL DEFAULT (datetime('now')),
			actor  TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pms_board ON player_mode_stats(mode, exp DESC, hands_solved DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts ON event_logs(ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_action ON event_logs(action, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rounds_dealt ON rounds(dealt_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rounds_session ON rounds(player_id, session_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	// Column additions for databases created before the admin portal: ALTER
	// TABLE has no IF NOT EXISTS for columns, so ignore duplicate-column
	// errors and fail on anything else.
	alters := []string{
		`ALTER TABLE players ADD COLUMN banned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE players ADD COLUMN banned_at TEXT`,
		`ALTER TABLE players ADD COLUMN ban_reason TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE rounds ADD COLUMN session_id TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range alters {
		if _, err := s.db.Exec(q); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("store: migrate: %w", err)
			}
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
