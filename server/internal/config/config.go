// Package config loads service configuration from the environment
// (.env supported via godotenv), following the fiber-boilerplate pattern.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                       string
	DBPath                     string
	CORSOrigins                []string
	EncryptionKey              string // 32-byte AES key, base64 or hex (local dev)
	SecretManagerEncryptionKey string // Secret Manager ref; when set, fetched key overrides EncryptionKey
	GoogleClientID             string
	AdminEmails                []string // allowlist guarding /api/v1/admin/*; empty = admin API disabled
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
		AdminEmails:                splitEmails(os.Getenv("ADMIN_EMAILS")),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotenv reads .env from the working directory, then from the parent
// directories (the repo root when running via "make dev" or
// "cd server && go run"). First match wins; real environment variables
// always override file values, so deploying with plain env vars still works.
func loadDotenv() {
	for _, dir := range []string{".", "..", "../.."} {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		_ = godotenv.Load(path)
		return
	}
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
