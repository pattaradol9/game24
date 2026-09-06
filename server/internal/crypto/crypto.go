// Package crypto encrypts player data with AES-256-GCM under a single
// 32-byte secret key. The key is supplied through configuration and lives
// entirely outside the service (recommended: Google Secret Manager, injected
// as the ENCRYPTION_KEY environment variable); it is never written to the
// database or logs. Lookup hashes are salted with a value derived from the
// key, so the database alone reveals nothing useful.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // 96-bit GCM nonce, standard length
	tagSize   = 16 // GCM authentication tag

	// versionAESGCM marks ciphertext sealed as version || nonce || ct+tag.
	versionAESGCM = 1

	// subHashInfo domain-separates the SubHash salt derivation from the
	// encryption key: sha256(key || subHashInfo) is never used as an AES key.
	subHashInfo = "game24/subhash-salt/v1"
)

type Crypter struct {
	aead cipher.AEAD
	salt []byte
}

// New builds a Crypter from a 32-byte secret key encoded as base64 or hex.
// It fails closed on a missing or malformed key so a misconfigured service
// refuses to start instead of silently writing undecryptable data.
func New(secret string) (*Crypter, error) {
	key, err := parseKey(secret)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: init cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: init gcm: %w", err)
	}
	h := sha256.New()
	h.Write(key)
	h.Write([]byte(subHashInfo))
	return &Crypter{aead: aead, salt: h.Sum(nil)}, nil
}

// parseKey decodes a 32-byte key from base64 (padded or not — the output of
// `openssl rand -base64 32`) or hex (64 hex chars). Surrounding whitespace
// from pasting or env files is tolerated.
func parseKey(secret string) ([]byte, error) {
	s := strings.TrimSpace(secret)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == keySize {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(b) == keySize {
		return b, nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == keySize {
		return b, nil
	}
	return nil, fmt.Errorf("crypto: ENCRYPTION_KEY must be a 32-byte key encoded as base64 or hex (generate one with: openssl rand -base64 32)")
}

// Encrypt seals plaintext for the given purpose (e.g. "players.email").
// The purpose is bound as associated data, so ciphertexts cannot be swapped
// between columns. Output: base64(version || nonce || ciphertext+tag).
func (c *Crypter) Encrypt(plaintext, purpose string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	out := make([]byte, 0, 1+nonceSize+len(plaintext)+tagSize)
	out = append(out, versionAESGCM)
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	out = append(out, nonce...)
	out = c.aead.Seal(out, nonce, []byte(plaintext), []byte(purpose))
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt opens ciphertext previously sealed for the purpose. Any tampering
// (including truncation or a wrong purpose) fails authentication.
func (c *Crypter) Decrypt(ciphertext, purpose string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: decode %s: %w", purpose, err)
	}
	if len(raw) < 1+nonceSize+tagSize {
		return "", fmt.Errorf("crypto: decrypt %s: ciphertext too short", purpose)
	}
	if raw[0] != versionAESGCM {
		return "", fmt.Errorf("crypto: decrypt %s: unsupported ciphertext version %d", purpose, raw[0])
	}
	pt, err := c.aead.Open(nil, raw[1:1+nonceSize], raw[1+nonceSize:], []byte(purpose))
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt %s: %w", purpose, err)
	}
	return string(pt), nil
}

// SubHash keys an identifier for lookup: sha256(salt || value) with a salt
// derived from the secret key, so it is stable across restarts but useless
// to an attacker holding only the database.
func (c *Crypter) SubHash(value string) string {
	h := sha256.New()
	h.Write(c.salt)
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// TokenHash indexes session tokens: sha256(value), no salt needed because
// tokens are high-entropy random.
func (c *Crypter) HashToken(value string) string {
	return TokenHash(value)
}

// TokenHash is the unsalted token index used for session lookups.
func TokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
