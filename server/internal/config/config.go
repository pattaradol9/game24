// Package config loads service configuration from the environment
// (.env supported via godotenv), following the fiber-boilerplate pattern.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// adcEnv is the one variable for which a .env value takes precedence over
// the inherited environment. The ADC key-file path is developer-local wiring
// that belongs next to the code, while shell profiles routinely export a
// value for some unrelated project; cloud runtimes never set it (they attach
// a service account instead). Empty or commented .env entries fall back to
// the inherited environment.
const adcEnv = "GOOGLE_APPLICATION_CREDENTIALS"

type Config struct {
	Port                       string
	DBPath                     string
	CORSOrigins                []string
	EncryptionKey              string // 32-byte AES key, base64 or hex (local dev)
	SecretManagerEncryptionKey string // Secret Manager ref; when set, fetched key overrides EncryptionKey
	GoogleClientID             string
	GoogleJWKSURL              string   // ID-token signing-key source; empty = Google's real JWKS (E2E tests point it at a local stand-in)
	AdminEmails                []string // allowlist guarding /api/v1/admin/*; empty = admin API disabled
	RateLimitPerMin            int      // per-IP API request cap; <= 0 keeps the built-in 200/min default
	PublicBaseURL              string   // canonical site origin (https://example.com); empty = derive per-request from Host
}

func Load() Config {
	loadDotenv()
	return Config{
		Port:                       env("APP_PORT", "8080"),
		DBPath:                     env("DB_PATH", "data/game24.db"),
		CORSOrigins:                splitCSV(env("CORS_ORIGINS", "http://localhost:5173")),
		EncryptionKey:              os.Getenv("ENCRYPTION_KEY"),
		SecretManagerEncryptionKey: os.Getenv("SECRETMANAGER_ENCRYPTION_KEY"),
		GoogleClientID:             os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleJWKSURL:              os.Getenv("GOOGLE_JWKS_URL"),
		AdminEmails:                splitEmails(os.Getenv("ADMIN_EMAILS")),
		RateLimitPerMin:            envInt("RATE_LIMIT_PER_MIN", 0),
		PublicBaseURL:              normalizeBaseURL(os.Getenv("PUBLIC_URL")),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt reads an integer variable; empty or unparsable values fall back so a
// typo can never zero out a safety knob.
func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// loadDotenv reads .env from the working directory, then from the parent
// directories (the repo root when running via "make dev" or
// "cd server && go run"). First match wins; real environment variables
// always override file values — except adcEnv, where a non-empty file value
// wins — so deploying with plain env vars still works.
func loadDotenv() {
	for _, dir := range []string{".", "..", "../.."} {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		envMap, err := godotenv.Read(path)
		if err != nil {
			log.Printf("config: ignoring unparsable %s: %v", path, err)
			return
		}
		for key, value := range envMap {
			if key == adcEnv {
				if value != "" {
					_ = os.Setenv(key, expandHome(value))
				}
				continue
			}
			if _, inherited := os.LookupEnv(key); !inherited {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

// expandHome resolves a leading "~" or "~/" against the user's home
// directory; .env values never pass through a shell, so a tilde would
// otherwise reach the Google SDK as a literal path that does not exist.
func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	return filepath.Join(home, p[2:])
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// splitEmails parses the ADMIN_EMAILS allowlist, normalizing entries so the
// comparison against a Google account email is case-insensitive.
func splitEmails(s string) []string {
	raw := splitCSV(s)
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

// normalizeBaseURL keeps only a valid http(s) origin without a trailing
// slash; anything else (including an empty value) means "derive the base
// URL from each request's Host header".
func normalizeBaseURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	if s == "" || !(strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")) {
		return ""
	}
	return s
}
