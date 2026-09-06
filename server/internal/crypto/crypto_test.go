package crypto

import (
	"bytes"
	"context"
	"testing"
)

type memStore struct {
	keyURI string
	blob   []byte
}

func (m *memStore) Load() (string, []byte, error) { return m.keyURI, m.blob, nil }

func (m *memStore) Save(keyURI string, blob []byte) error {
	m.keyURI, m.blob = keyURI, blob
	return nil
}

func newLocal(t *testing.T, store *memStore) *Crypter {
	t.Helper()
	c, err := New(context.Background(), ModeLocal, "", "", store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestRoundTrip(t *testing.T) {
	store := &memStore{}
	c := newLocal(t, store)

	ct, err := c.Encrypt("player@mail.com", "players.email")
	if err != nil {
		t.Fatal(err)
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
}

func TestRestartWithPersistedKeyset(t *testing.T) {
	store := &memStore{}
	first := newLocal(t, store)
	secret := "opaque-token-value"
	ct, err := first.Encrypt(secret, "players.token")
	if err != nil {
		t.Fatal(err)
	}

	// simulate restart: fresh crypter over the same persisted envelope
	second := newLocal(t, store)
	got, err := second.Decrypt(ct, "players.token")
	if err != nil || got != secret {
		t.Fatalf("after restart: %q, %v", got, err)
	}
	if !bytes.Equal(first.salt, second.salt) {
		t.Fatal("salt changed across restart")
	}
}

func TestSubHashStable(t *testing.T) {
	store := &memStore{}
	c := newLocal(t, store)
	h1 := c.SubHash("1234567890")
	if h1 != c.SubHash("1234567890") {
		t.Fatal("sub hash not deterministic")
	}
	if h1 == c.SubHash("1234567891") {
		t.Fatal("different subs hash equal")
	}

	second := newLocal(t, store)
	if second.SubHash("1234567890") != h1 {
		t.Fatal("sub hash changed across restart")
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
	c := newLocal(t, &memStore{})
	if ct, _ := c.Encrypt("", "players.email"); ct != "" {
		t.Fatal("empty plaintext should encrypt to empty")
	}
	if pt, _ := c.Decrypt("", "players.email"); pt != "" {
		t.Fatal("empty ciphertext should decrypt to empty")
	}
}
