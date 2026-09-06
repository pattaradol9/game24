package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pattaradol9/game24/server/internal/auth"
	"github.com/pattaradol9/game24/server/internal/config"
	"github.com/pattaradol9/game24/server/internal/crypto"
	"github.com/pattaradol9/game24/server/internal/game"
	"github.com/pattaradol9/game24/server/internal/handler"
	"github.com/pattaradol9/game24/server/internal/httpserver"
	"github.com/pattaradol9/game24/server/internal/room"
	"github.com/pattaradol9/game24/server/internal/secretmanager"
	"github.com/pattaradol9/game24/server/internal/store"
	"github.com/pattaradol9/game24/server/internal/webui"
	"github.com/pattaradol9/game24/server/internal/ws"
)

func main() {
	cfg := config.Load()

	// 1. resolve the encryption key. SECRETMANAGER_ENCRYPTION_KEY (a Google
	// Secret Manager reference) takes priority and its fetched value
	// overrides ENCRYPTION_KEY; local dev just sets ENCRYPTION_KEY. Either
	// way we fail closed on a missing or malformed key.
	encryptionKey := cfg.EncryptionKey
	if cfg.SecretManagerEncryptionKey != "" {
		fetched, err := secretmanager.Fetch(context.Background(), cfg.SecretManagerEncryptionKey)
		if err != nil {
			log.Fatalf("secretmanager (refusing to start): %v", err)
		}
		encryptionKey = fetched
	}
	cr, err := crypto.New(encryptionKey)
	if err != nil {
		log.Fatalf("crypto (refusing to start): %v", err)
	}

	// 2. database with the crypter attached (PII is encrypted before write).
	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir db dir: %v", err)
		}
	}
	db, err := store.Open(cfg.DBPath, cr)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	// 3. multiplayer hub with EXP awarding for signed-in winners.
	hub := room.NewHub(func(dbPlayerID string, mode game.Mode, points int64) {
		if _, _, err := db.AwardEXP(dbPlayerID, string(mode), points, true); err != nil {
			log.Printf("award exp: %v", err)
		}
	})

	// 4. REST + websocket + embedded SPA.
	var verifier *auth.GoogleVerifier
	if cfg.GoogleClientID != "" {
		verifier = auth.NewGoogleVerifier(cfg.GoogleClientID)
	} else {
		log.Printf("GOOGLE_OAUTH_CLIENT_ID not set: google sign-in disabled (guest play only)")
	}
	if len(cfg.AdminEmails) == 0 {
		log.Printf("ADMIN_EMAILS not set: admin portal disabled (/api/v1/admin/* returns 404)")
	} else {
		log.Printf("admin portal enabled at /admin for %d allowlisted account(s) (API under /api/v1/admin)", len(cfg.AdminEmails))
	}

	api := &handler.API{
		Store:  db,
		Google: verifier,
		Hub:    hub,
		Cfg:    cfg,
		WS:     ws.NewHandler(hub, db, cfg.CORSOrigins),
	}

	srv := httpserver.New(cfg, api, webui.FS())

	go func() {
		log.Printf("game24 server listening on :%s", cfg.Port)
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
