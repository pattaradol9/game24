// Package auth verifies Google-issued ID tokens (Sign in with Google).
// Tokens arrive from the web client via Google Identity Services; we check
// the RS256 signature against Google's published JWKS plus iss/aud/exp.
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const googleJWKS = "https://www.googleapis.com/oauth2/v3/certs"

type IDTokenClaims struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Aud     string `json:"aud"`
	Iss     string `json:"iss"`
	Exp     int64  `json:"exp"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type GoogleVerifier struct {
	clientID string
	// JWKSURL is where signing keys are fetched from. It defaults to
	// Google's published JWKS; the E2E environment overrides it (through
	// GOOGLE_JWKS_URL) so locally minted ID tokens can be verified without
	// touching the network.
	JWKSURL string
	hc      *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func NewGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{clientID: clientID, JWKSURL: googleJWKS, hc: &http.Client{Timeout: 10 * time.Second}}
}

func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (IDTokenClaims, error) {
	var claims IDTokenClaims
	parts := splitToken(idToken)
	if len(parts) != 3 {
		return claims, fmt.Errorf("auth: malformed id token")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, fmt.Errorf("auth: header: %w", err)
	}
	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return claims, fmt.Errorf("auth: header json: %w", err)
	}
	if header.Alg != "RS256" {
		return claims, fmt.Errorf("auth: unexpected alg %q", header.Alg)
	}
	key, err := v.publicKey(ctx, header.Kid)
	if err != nil {
		return claims, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return claims, fmt.Errorf("auth: signature: %w", err)
	}
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return claims, fmt.Errorf("auth: bad signature: %w", err)
	}

	claimsRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, fmt.Errorf("auth: payload: %w", err)
	}
	if err := json.Unmarshal(claimsRaw, &claims); err != nil {
		return claims, fmt.Errorf("auth: payload json: %w", err)
	}
	if claims.Iss != "accounts.google.com" && claims.Iss != "https://accounts.google.com" {
		return claims, fmt.Errorf("auth: unexpected issuer %q", claims.Iss)
	}
	if claims.Aud != v.clientID {
		return claims, fmt.Errorf("auth: audience mismatch")
	}
	if time.Now().Unix() >= claims.Exp {
		return claims, fmt.Errorf("auth: token expired")
	}
	if claims.Sub == "" {
		return claims, fmt.Errorf("auth: missing subject")
	}
	return claims, nil
}

func (v *GoogleVerifier) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys == nil || time.Since(v.fetched) > time.Hour {
		if err := v.refresh(ctx); err != nil {
			return nil, err
		}
	}
	key, ok := v.keys[kid]
	if !ok {
		// unknown kid: Google may have rotated; force one refresh
		if err := v.refresh(ctx); err != nil {
			return nil, err
		}
		key, ok = v.keys[kid]
	}
	if !ok {
		return nil, fmt.Errorf("auth: unknown key id %q", kid)
	}
	return key, nil
}

func (v *GoogleVerifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil)
	if err != nil {
		return err
	}
	res, err := v.hc.Do(req)
	if err != nil {
		return fmt.Errorf("auth: fetch jwks: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: jwks status %d", res.StatusCode)
	}
	var set jwks
	if err := json.Unmarshal(body, &set); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := parseRSA(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("auth: jwks has no usable keys")
	}
	v.keys = keys
	v.fetched = time.Now()
	return nil
}

func parseRSA(k jwk) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	var exp int
	for _, b := range e {
		exp = exp<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}, nil
}

func splitToken(tok string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(tok) && len(parts) < 2; i++ {
		if tok[i] == '.' {
			parts = append(parts, tok[start:i])
			start = i + 1
		}
	}
	parts = append(parts, tok[start:])
	return parts
}
