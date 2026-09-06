// Package config loads service configuration from the environment
// (.env supported via godotenv), following the fiber-boilerplate pattern.
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DBPath          string
	CORSOrigins     []string
	EncryptionMode  string // "local" | "kms"
	KMSKeyURI       string
	CredentialsPath string // GCP service account file (optional; empty = ADC)
	GoogleClientID  string
}

func Load() Config {
	_ = godotenv.Load()
	return Config{
		Port:            env("APP_PORT", "8080"),
		DBPath:          env("DB_PATH", "data/game24.db"),
		CORSOrigins:     splitCSV(env("CORS_ORIGINS", "http://localhost:5173")),
		EncryptionMode:  env("ENCRYPTION_MODE", "local"),
		KMSKeyURI:       os.Getenv("KMS_KEY_URI"),
		CredentialsPath: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		GoogleClientID:  os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
