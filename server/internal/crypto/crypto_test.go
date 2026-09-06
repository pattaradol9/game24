package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// testKey returns a fresh random key in the same encoding ops will use.
func testKey(t *testing.T) string {
	t.Helper()
	raw := make([]byte, keySize)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// hexKey is a valid 64-hex-char key (all 0xab bytes).
var hexKey = strings.Repeat("ab", 32)

func TestRoundTrip(t *testing.T) {
	c, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ct, err := c.Encrypt("player@mail.com", "players.email")
	if err != nil {
		t.Fatal(err)
	}
	if ct == "player@mail.com" {
		t.Fatal("ciphertext equals plaintext")
	}
	pt, err := c.Decrypt(ct, "players.email")
	if err != nil || pt != "player@mail.com" {
		t.Fatalf("round trip = %q, %v", pt, err)
	}

	// tampering with the ciphertext must fail
	raw := []byte(ct)
	raw[len(raw)/2] ^= 0xFF
	if _, err := c.Decrypt(string(raw), "players.email"); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}

	// truncation must fail
	if _, err := c.Decrypt(ct[:len(ct)-4], "players.email"); err == nil {
		t.Fatal("truncated ciphertext accepted")
	}
}

func TestPurposeBinding(t *testing.T) {
	c, err := New(hexKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ct, err := c.Encrypt("player@mail.com", "players.email")
	if err != nil {
		t.Fatal(err)
	}
	// the purpose is associated data: a ciphertext sealed for one column
	// must not open under another
	if _, err := c.Decrypt(ct, "players.nickname"); err == nil {
		t.Fatal("ciphertext accepted under wrong purpose")
	}
}

func TestWrongKeyRejected(t *testing.T) {
	first, err := New(hexKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	second, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ct, err := first.Encrypt("secret", "players.token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Decrypt(ct, "players.token"); err == nil {
		t.Fatal("ciphertext from another key accepted")
	}
}

func TestKeyParsing(t *testing.T) {
	raw := make([]byte, keySize)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	padded := base64.StdEncoding.EncodeToString(raw) // openssl rand -base64 32
	unpadded := strings.TrimRight(padded, "=")       // 43 chars, no padding
	hexStr := hex.EncodeToString(raw)                // 64 hex chars

	valid := []string{padded, unpadded, hexStr, "  " + padded + "\n"}
	for _, s := range valid {
		if _, err := New(s); err != nil {
			t.Fatalf("New(%q): %v", s, err)
		}
	}

	invalid := []string{
		"",
		"short",
		base64.StdEncoding.EncodeToString([]byte("16-bytes-folly!!")), // 16 bytes ≠ 32
		strings.Repeat("ab", 16), // 16 bytes of hex ≠ 32
		"!!!not-base64-or-hex!!!",
	}
	for _, s := range invalid {
		if _, err := New(s); err == nil {
			t.Fatalf("New(%q) accepted invalid key", s)
		}
	}
}

func TestSubHashStable(t *testing.T) {
	key := testKey(t)
	first, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h1 := first.SubHash("1234567890")
	if h1 != first.SubHash("1234567890") {
		t.Fatal("sub hash not deterministic")
	}
	if h1 == first.SubHash("1234567891") {
		t.Fatal("different subs hash equal")
	}

	// same key, fresh instance (restart): salt is derived, so it must match
	second, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if second.SubHash("1234567890") != h1 {
		t.Fatal("sub hash changed across restart")
	}

	// a different key must salt differently
	third, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if third.SubHash("1234567890") == h1 {
		t.Fatal("sub hash identical under a different key")
	}
}

func TestTokenHash(t *testing.T) {
	if TokenHash("abc") != TokenHash("abc") {
		t.Fatal("token hash not deterministic")
	}
	if TokenHash("abc") == TokenHash("abd") {
		t.Fatal("token hash collision")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	c, err := New(hexKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if ct, _ := c.Encrypt("", "players.email"); ct != "" {
		t.Fatal("empty plaintext should encrypt to empty")
	}
	if pt, _ := c.Decrypt("", "players.email"); pt != "" {
		t.Fatal("empty ciphertext should decrypt to empty")
	}
}

func TestNonceUniqueness(t *testing.T) {
	c, err := New(hexKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ct, err := c.Encrypt("same plaintext", "players.email")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(ct)
		if err != nil {
			t.Fatal(err)
		}
		nonce := string(raw[1 : 1+nonceSize])
		if seen[nonce] {
			t.Fatal("nonce reused for identical plaintext")
		}
		seen[nonce] = true
	}
}
