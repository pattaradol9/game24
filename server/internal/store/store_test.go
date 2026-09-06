package store

import (
	"path/filepath"
	"strings"
	"testing"

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

	// google player wins two hands then skips one
	for i := 0; i < 2; i++ {
		if _, _, err = s.AwardEXP(gp.ID, "queen", 150, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 0, false); err != nil {
		t.Fatal(err)
	}

	board, err := s.Leaderboard("queen", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 1 {
		t.Fatalf("board has %d rows, want 1 (guest hidden)", len(board))
	}
	row := board[0]
	if row.PlayerID != gp.ID || row.Exp != 300 || row.Nickname != "A" {
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
