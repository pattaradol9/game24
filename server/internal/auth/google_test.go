package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeGoogle struct {
	ts *httptest.Server
}

// publish sets the single fixed JWKS document for the rest of the test.
func (f *fakeGoogle) publish(t *testing.T, keys []jwk) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"keys": keys})
	if err != nil {
		t.Fatal(err)
	}
	f.ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	})
}

func jwkOf(kid string, key *rsa.PublicKey) jwk {
	return jwk{
		Kid: kid, Kty: "RSA", Alg: "RS256", Use: "sig",
		N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}), // 65537
	}
}

// sign makes an RS256 id token with the given claims.
func sign(t *testing.T, kid string, key *rsa.PrivateKey, claims IDTokenClaims) string {
	t.Helper()
	if claims.Exp == 0 {
		claims.Exp = time.Now().Add(time.Hour).Unix()
	}
	claims.Iss = "https://accounts.google.com"
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid})
	body, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerifyIDToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeGoogle{ts: httptest.NewServer(http.NewServeMux())}
	defer f.ts.Close()
	f.publish(t, []jwk{jwkOf("kid-1", &key.PublicKey)})

	v := NewGoogleVerifier("my-client-id")
	v.JWKSURL = f.ts.URL

	token := sign(t, "kid-1", key, IDTokenClaims{
		Sub: "google-sub-1", Email: "p@mail.com", Name: "Player One", Aud: "my-client-id",
	})
	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "google-sub-1" || claims.Email != "p@mail.com" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestVerifyRejects(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeGoogle{ts: httptest.NewServer(http.NewServeMux())}
	defer f.ts.Close()
	// kid-1 is the real key; kid-404 belongs to a different key.
	f.publish(t, []jwk{jwkOf("kid-1", &key.PublicKey), jwkOf("kid-404", &other.PublicKey)})

	v := NewGoogleVerifier("my-client-id")
	v.JWKSURL = f.ts.URL

	cases := []struct {
		name  string
		token string
	}{
		{"wrong audience", sign(t, "kid-1", key, IDTokenClaims{Sub: "s", Aud: "other-app"})},
		{"signed by unpublished key", sign(t, "kid-1", other, IDTokenClaims{Sub: "s", Aud: "my-client-id"})},
		{"unknown kid", sign(t, "kid-999", key, IDTokenClaims{Sub: "s", Aud: "my-client-id"})},
		{"expired", sign(t, "kid-1", key, IDTokenClaims{Sub: "s", Aud: "my-client-id", Exp: time.Now().Add(-time.Hour).Unix()})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), tc.token); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	if _, err := v.Verify(context.Background(), "not.a.token"); err == nil {
		t.Fatal("garbage accepted")
	}
}
