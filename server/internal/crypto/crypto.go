// Package crypto provides envelope encryption for player data:
// a Tink keyset (DEK) wrapped by a remote KMS key (KEK, Google Cloud KMS).
// The KMS is called only while unwrapping the keyset at service start;
// the unwrapped keyset lives in memory until shutdown (fail-closed).
package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/tink/go/aead"
	"github.com/google/tink/go/insecurecleartextkeyset"
	"github.com/google/tink/go/integration/gcpkms"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/testing/fakekms"
	"github.com/google/tink/go/tink"
)

const (
	ModeLocal = "local"
	ModeKMS   = "kms"

	adSystemKeys = "system-keys/v1"
)

// BlobStore persists the wrapped keyset envelope (system_keys table).
type BlobStore interface {
	// Load returns the persisted KMS key URI and wrapped blob, or empty
	// values when nothing has been provisioned yet.
	Load() (keyURI string, blob []byte, err error)
	Save(keyURI string, blob []byte) error
}

// fakeClientAEAD resolves a persisted fake-KMS key URI after a restart.
func fakeClientAEAD(keyURI string) (tink.AEAD, error) {
	fc, err := fakekms.NewClient("fake-kms://")
	if err != nil {
		return nil, fmt.Errorf("crypto: fake kms: %w", err)
	}
	return fc.GetAEAD(keyURI)
}

type envelope struct {
	Version int    `json:"v"`
	Salt    string `json:"salt"`   // base64, used for keyed lookup hashes
	Keyset  string `json:"keyset"` // base64, keyset wrapped by the KMS
}

type Crypter struct {
	aead tink.AEAD
	salt []byte
}

// New unlocks or provisions the system keyset.
//
// mode "kms" uses Google Cloud KMS (kmsKeyURI must be a gcp-kms:// URI;
// credentialsPath may be empty to fall back to Application Default
// Credentials). mode "local" uses Tink's in-process fake KMS for
// development and tests.
func New(ctx context.Context, mode, kmsKeyURI, credentialsPath string, blobs BlobStore) (*Crypter, error) {
	var (
		remote    tink.AEAD
		activeURI string
	)
	switch mode {
	case ModeKMS:
		if kmsKeyURI == "" {
			return nil, fmt.Errorf("crypto: KMS mode requires a key URI")
		}
		var (
			client interface {
				GetAEAD(string) (tink.AEAD, error)
			}
			err error
		)
		if credentialsPath != "" {
			client, err = gcpkms.NewClientWithCredentials("gcp-kms://", credentialsPath)
		} else {
			client, err = gcpkms.NewClient("gcp-kms://")
		}
		if err != nil {
			return nil, fmt.Errorf("crypto: kms client: %w", err)
		}
		remote, err = client.GetAEAD(kmsKeyURI)
		if err != nil {
			return nil, fmt.Errorf("crypto: kms aead: %w", err)
		}
		activeURI = kmsKeyURI
	case ModeLocal:
		fc, err := fakekms.NewClient("fake-kms://")
		if err != nil {
			return nil, fmt.Errorf("crypto: fake kms: %w", err)
		}
		activeURI = kmsKeyURI
		if activeURI == "" {
			if activeURI, err = fakekms.NewKeyURI(); err != nil {
				return nil, fmt.Errorf("crypto: fake key uri: %w", err)
			}
		}
		if remote, err = fc.GetAEAD(activeURI); err != nil {
			return nil, fmt.Errorf("crypto: fake kms aead: %w", err)
		}
	default:
		return nil, fmt.Errorf("crypto: unknown encryption mode %q", mode)
	}

	template := aead.AES256GCMKeyTemplate()
	envAEAD := aead.NewKMSEnvelopeAEAD2(template, remote)

	storedURI, storedBlob, err := blobs.Load()
	if err != nil {
		return nil, fmt.Errorf("crypto: load system keys: %w", err)
	}
	if mode == ModeLocal && storedURI != "" {
		activeURI = storedURI
		if remote, err = fakeClientAEAD(activeURI); err != nil {
			return nil, err
		}
		envAEAD = aead.NewKMSEnvelopeAEAD2(template, remote)
	}

	var salt []byte
	if len(storedBlob) == 0 {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("crypto: salt: %w", err)
		}
		handle, err := keyset.NewHandle(template)
		if err != nil {
			return nil, fmt.Errorf("crypto: keyset: %w", err)
		}
		var buf bytes.Buffer
		if err := insecurecleartextkeyset.Write(handle, keyset.NewBinaryWriter(&buf)); err != nil {
			return nil, fmt.Errorf("crypto: serialize keyset: %w", err)
		}
		wrapped, err := envAEAD.Encrypt(buf.Bytes(), []byte(adSystemKeys))
		if err != nil {
			return nil, fmt.Errorf("crypto: wrap keyset: %w", err)
		}
		envJSON, err := json.Marshal(envelope{Version: 1, Salt: base64.StdEncoding.EncodeToString(salt), Keyset: base64.StdEncoding.EncodeToString(wrapped)})
		if err != nil {
			return nil, fmt.Errorf("crypto: encode system keys: %w", err)
		}
		if err := blobs.Save(activeURI, envJSON); err != nil {
			return nil, fmt.Errorf("crypto: save system keys: %w", err)
		}
		primitive, err := aead.New(handle)
		if err != nil {
			return nil, fmt.Errorf("crypto: primitive: %w", err)
		}
		return &Crypter{aead: primitive, salt: salt}, nil
	}

	var env envelope
	if err := json.Unmarshal(storedBlob, &env); err != nil {
		return nil, fmt.Errorf("crypto: parse system keys: %w", err)
	}
	if env.Version != 1 {
		return nil, fmt.Errorf("crypto: unsupported system keys version %d", env.Version)
	}
	salt, err = base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return nil, fmt.Errorf("crypto: salt: %w", err)
	}
	wrapped, err := base64.StdEncoding.DecodeString(env.Keyset)
	if err != nil {
		return nil, fmt.Errorf("crypto: keyset: %w", err)
	}
	cleartext, err := envAEAD.Decrypt(wrapped, []byte(adSystemKeys))
	if err != nil {
		// Fail closed: a broken KMS or tampered blob must not start.
		return nil, fmt.Errorf("crypto: unwrap keyset (KMS unreachable or data tampered): %w", err)
	}
	handle, err := insecurecleartextkeyset.Read(keyset.NewBinaryReader(bytes.NewReader(cleartext)))
	if err != nil {
		return nil, fmt.Errorf("crypto: read keyset: %w", err)
	}
	primitive, err := aead.New(handle)
	if err != nil {
		return nil, fmt.Errorf("crypto: primitive: %w", err)
	}
	return &Crypter{aead: primitive, salt: salt}, nil
}

// Encrypt seals plaintext for the given purpose (e.g. "players.email").
func (c *Crypter) Encrypt(plaintext, purpose string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	ct, err := c.aead.Encrypt([]byte(plaintext), []byte(purpose))
	if err != nil {
		return "", fmt.Errorf("crypto: encrypt %s: %w", purpose, err)
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt opens ciphertext previously sealed for the purpose.
func (c *Crypter) Decrypt(ciphertext, purpose string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	ct, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: decode %s: %w", purpose, err)
	}
	pt, err := c.aead.Decrypt(ct, []byte(purpose))
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt %s: %w", purpose, err)
	}
	return string(pt), nil
}

// SubHash keys an identifier for lookup: sha256(salt || value).
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
