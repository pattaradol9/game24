package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pattaradol9/game24/server/internal/crypto"
)

// testKey is a valid 64-hex-char (32-byte) encryption key.
var testKey = strings.Repeat("ab", 32)

func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	cr, err := crypto.New(testKey)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, cr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGuestFlow(t *testing.T) {
	s := openTest(t)
	p, token, err := s.CreateGuest("Somchai")
	if err != nil {
		t.Fatal(err)
	}
	if p.Nickname != "Somchai" || !p.IsGuest {
		t.Fatalf("guest = %+v", p)
	}

	got, err := s.PlayerByToken(token)
	if err != nil || got.ID != p.ID {
		t.Fatalf("by token: %+v, %v", got, err)
	}
	if _, err := s.PlayerByToken("wrong-token"); err == nil {
		t.Fatal("wrong token accepted")
	}
}

func TestAwardEXPAndLeaderboard(t *testing.T) {
	s := openTest(t)
	p, _, err := s.CreateGuest("guest")
	if err != nil {
		t.Fatal(err)
	}
	gp, tok, err := s.GoogleLogin("sub-123", "a@b.c", "A", "http://pic")
	if err != nil {
		t.Fatal(err)
	}

	// guest wins hands but must never appear on the board
	st, _, err := s.AwardEXP(p.ID, "queen", 100, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Exp != 100 || st.CurrentStreak != 1 {
		t.Fatalf("guest stat = %+v", st)
	}

	// google player wins two hands then skips one; the board ranks the BEST
	// single hand, so 150+150 stays a 150 high score
	for i := 0; i < 2; i++ {
		if _, _, err = s.AwardEXP(gp.ID, "queen", 150, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 0, false); err != nil {
		t.Fatal(err)
	}

	board, err := s.ScoreLeaderboard("queen", false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 1 {
		t.Fatalf("board has %d rows, want 1 (guest hidden)", len(board))
	}
	row := board[0]
	if row.PlayerID != gp.ID || row.Score != 150 || row.HandsSolved != 2 || row.Nickname != "A" {
		t.Fatalf("row = %+v", row)
	}

	after, err := s.PlayerByToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if after.TotalExp != 300 {
		t.Fatalf("total exp = %d, want 300", after.TotalExp)
	}
	qs := after.Stats["queen"]
	if qs.CurrentStreak != 0 || qs.BestStreak != 2 || qs.HandsSkipped != 1 {
		t.Fatalf("queen stat = %+v", qs)
	}
}

func TestScoreLeaderboardPeriodsAndBoostImmunity(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-score", "s@b.c", "S", "")
	if err != nil {
		t.Fatal(err)
	}

	// a ×3 EXP boost is live: the hand banks boosted EXP but the score
	// ledger must record the raw points
	if _, err := s.SetBoostConfig("admin:x", BoostKindExp, 3, 60, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableBoost("admin:x", BoostKindExp); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AwardEXP(gp.ID, "queen", 100, true); err != nil {
		t.Fatal(err)
	}
	board, err := s.ScoreLeaderboard("queen", false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 1 || board[0].Score != 100 {
		t.Fatalf("score must stay raw under a boost, got %+v", board)
	}

	// a hand scored last week counts all-time but not this week
	if _, err := s.db.Exec(`UPDATE player_scores SET awarded_at = datetime('now', '-8 days')`); err != nil {
		t.Fatal(err)
	}
	weekly, err := s.ScoreLeaderboard("queen", true, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(weekly) != 0 {
		t.Fatalf("last week's hand leaked into the weekly board: %+v", weekly)
	}
	allTime, err := s.ScoreLeaderboard("queen", false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTime) != 1 || allTime[0].Score != 100 {
		t.Fatalf("all-time board lost the old hand: %+v", allTime)
	}

	// this week's hand shows up on the weekly board; the high score never
	// accumulates — the 40-point hand must not stack onto the 100
	if _, _, err := s.AwardEXP(gp.ID, "queen", 40, true); err != nil {
		t.Fatal(err)
	}
	weekly, err = s.ScoreLeaderboard("queen", true, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(weekly) != 1 || weekly[0].Score != 40 || weekly[0].HandsSolved != 1 {
		t.Fatalf("weekly board = %+v, want one 40-point hand", weekly)
	}
	allTime, err = s.ScoreLeaderboard("queen", false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTime) != 1 || allTime[0].Score != 100 || allTime[0].HandsSolved != 2 {
		t.Fatalf("all-time high score = %+v, want 100 across 2 hands", allTime)
	}

	// the week window opens on Monday 00:00 UTC
	ws := WeekStartUTC(time.Date(2026, 9, 9, 15, 30, 0, 0, time.UTC)) // a Wednesday
	if ws.Weekday() != time.Monday || ws.Hour() != 0 || ws.UTC().Day() != 7 {
		t.Fatalf("week start = %v, want Mon Sep 7 00:00 UTC", ws)
	}
}

func TestResetPlayerStatsClearsScore(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-reset", "r@b.c", "R", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AwardEXP(gp.ID, "queen", 100, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResetPlayerStats("admin:x", gp.ID, "queen"); err != nil {
		t.Fatal(err)
	}
	board, err := s.ScoreLeaderboard("queen", false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 0 {
		t.Fatalf("score survived the stat reset: %+v", board)
	}
}

func TestGoogleReloginRotatesToken(t *testing.T) {
	s := openTest(t)
	_, tok1, err := s.GoogleLogin("sub-1", "x@y.z", "X", "")
	if err != nil {
		t.Fatal(err)
	}
	_, tok2, err := s.GoogleLogin("sub-1", "x@y.z", "X", "")
	if err != nil {
		t.Fatal(err)
	}
	if tok1 == tok2 {
		t.Fatal("token not rotated")
	}
	if _, err := s.PlayerByToken(tok1); err == nil {
		t.Fatal("old token still valid")
	}
	if _, err := s.PlayerByToken(tok2); err != nil {
		t.Fatalf("new token rejected: %v", err)
	}
}

func TestPIIEncryptedAtRest(t *testing.T) {
	s := openTest(t)
	if _, _, err := s.GoogleLogin("sub-enc", "secret@mail.com", "SecretNick", ""); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := s.db.QueryRow(`SELECT email_enc FROM players WHERE email_enc != '' LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw == "secret@mail.com" {
		t.Fatal("email stored in plaintext")
	}
	if _, err := s.cr.Decrypt(raw, "players.email"); err != nil || s.cr == nil {
		t.Fatalf("decrypt failed: %v", err)
	}
}
