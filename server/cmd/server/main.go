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
	"github.com/pattaradol9/game24/server/internal/store"
	"github.com/pattaradol9/game24/server/internal/webui"
	"github.com/pattaradol9/game24/server/internal/ws"
)

func main() {
	cfg := config.Load()

	// 1. database (crypter attached afterwards: crypto needs the DB for the
	// wrapped keyset, the DB needs crypto for PII).
	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir db dir: %v", err)
		}
	}
	db, err := store.Open(cfg.DBPath, nil)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	// 2. envelope encryption — one KMS call at startup, fail-closed.
	cr, err := crypto.New(context.Background(), cfg.EncryptionMode, cfg.KMSKeyURI, cfg.CredentialsPath, storeAdapter{db})
	if err != nil {
		log.Fatalf("crypto (refusing to start): %v", err)
	}
	db.SetCrypter(cr)

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

	api := &handler.API{
		Store:  db,
		Google: verifier,
		Hub:    hub,
		Cfg:    cfg,
		WS:     ws.NewHandler(hub, db, cfg.CORSOrigins),
	}

	srv := httpserver.New(cfg, api, webui.FS())

	go func() {
		log.Printf("game24 server listening on :%s (mode=%s)", cfg.Port, cfg.EncryptionMode)
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

// storeAdapter adapts *store.Store to crypto.BlobStore.
type storeAdapter struct{ db *store.Store }

func (a storeAdapter) Load() (string, []byte, error) { return a.db.LoadSystemKeys() }
func (a storeAdapter) Save(uri string, blob []byte) error {
	return a.db.SaveSystemKeys(uri, blob)
}

var _ crypto.BlobStore = storeAdapter{}
