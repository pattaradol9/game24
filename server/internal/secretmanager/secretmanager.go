// Package secretmanager resolves the encryption key from Google Secret
// Manager over its REST API using Application Default Credentials (ADC),
// which keeps the dependency footprint far smaller than the official client
// library. It is called exactly once, at service start (fail-closed).
package secretmanager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	apiBase = "https://secretmanager.googleapis.com/v1/"
	scope   = "https://www.googleapis.com/auth/cloud-platform"
)

// ResolveName turns a SECRETMANAGER_ENCRYPTION_KEY reference into a full
// secret resource name. Accepted forms:
//
//		projects/PROJECT/secrets/NAME              -> latest version
//		projects/PROJECT/secrets/NAME/versions/V   -> specific version
//		NAME                                       -> latest version, project from
//		                                              GOOGLE_CLOUD_PROJECT (or
//	                                             GOOGLE_PROJECT)
func ResolveName(ref string) (string, error) {
	r := strings.TrimSpace(ref)
	if r == "" {
		return "", fmt.Errorf("secretmanager: empty secret reference")
	}
	if strings.HasPrefix(r, "projects/") {
		parts := strings.Split(r, "/")
		switch {
		case len(parts) == 4:
			return r + "/versions/latest", nil
		case len(parts) == 6 && parts[4] == "versions":
			return r, nil
		default:
			return "", fmt.Errorf("secretmanager: bad resource name %q (want projects/PROJECT/secrets/NAME[/versions/VERSION])", r)
		}
	}
	if strings.Contains(r, "/") || !validSecretID(r) {
		return "", fmt.Errorf("secretmanager: bad secret name %q", r)
	}
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		project = os.Getenv("GOOGLE_PROJECT")
	}
	if project == "" {
		return "", fmt.Errorf("secretmanager: bare secret name %q needs GOOGLE_CLOUD_PROJECT (or GOOGLE_PROJECT), or use projects/PROJECT/secrets/NAME", r)
	}
	return "projects/" + project + "/secrets/" + r + "/versions/latest", nil
}

func validSecretID(s string) bool {
	if s == "" || len(s) > 255 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// Fetch returns the secret's payload as a string (the key material the
// crypto package expects: base64- or hex-encoded 32 bytes). Credentials come
// from ADC — GOOGLE_APPLICATION_CREDENTIALS locally, or the attached service
// account on Cloud Run/GKE/GCE (needs roles/secretmanager.secretAccessor).
func Fetch(ctx context.Context, ref string) (string, error) {
	name, err := ResolveName(ref)
	if err != nil {
		return "", err
	}
	creds, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		return "", fmt.Errorf("secretmanager: no credentials (set GOOGLE_APPLICATION_CREDENTIALS or attach a service account with roles/secretmanager.secretAccessor): %w", err)
	}
	hc := oauth2.NewClient(ctx, creds.TokenSource)
	hc.Timeout = 15 * time.Second

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+name+":access", nil)
	if err != nil {
		return "", fmt.Errorf("secretmanager: build request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("secretmanager: fetch %s: %w", name, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("secretmanager: fetch %s: HTTP %d: %s", name, resp.StatusCode, snippet(body))
	}

	var out struct {
		Payload struct {
			Data string `json:"data"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("secretmanager: parse response: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(out.Payload.Data))
	if err != nil || len(raw) == 0 {
		return "", fmt.Errorf("secretmanager: secret %s has no payload", name)
	}
	return string(raw), nil
}

// snippet flattens an API error body into one short line for logs.
func snippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
}
