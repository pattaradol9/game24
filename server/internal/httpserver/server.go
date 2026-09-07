package httpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/pattaradol9/game24/server/internal/config"
	"github.com/pattaradol9/game24/server/internal/handler"
)

// Server wraps http.Server following the echo-service convention.
type Server struct {
	http *http.Server
}

func New(cfg config.Config, api *handler.API, webFS fs.FS) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(90 * time.Second))
	r.Use(httprate.LimitByIP(200, time.Minute))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Mount("/api/v1", api.Routes())

	// SEO endpoints: robots.txt needs PUBLIC_URL (or falls back to a
	// conservative block-all), sitemap.xml only exists with PUBLIC_URL.
	r.Get("/robots.txt", robotsHandler(cfg.PublicBaseURL))
	r.Get("/sitemap.xml", sitemapHandler(cfg.PublicBaseURL))

	if webFS != nil {
		r.NotFound(spaHandler(webFS, cfg.PublicBaseURL))
	}

	return &Server{
		http: &http.Server{
			Addr:              ":" + cfg.Port,
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// spaHandler serves static assets and falls back to index.html so
// client-side routes like /room/CODE work on hard refresh. HTML responses
// get their meta tags rewritten per route (SEO/link previews), and every
// asset class gets the right Cache-Control policy.
func spaHandler(webFS fs.FS, publicBaseURL string) http.HandlerFunc {
	index, indexErr := fs.ReadFile(webFS, "index.html")
	mime.AddExtensionType(".webmanifest", "application/manifest+json")

	serveHTML := func(w http.ResponseWriter, r *http.Request) {
		if indexErr != nil {
			http.Error(w, "frontend not built (run make build-web)", http.StatusNotFound)
			return
		}
		base := requestBaseURL(r, publicBaseURL)
		meta := seoMetaForPath(r.URL.Path, base)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// HTML must revalidate so deploys are picked up immediately.
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(renderIndex(index, meta))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			serveHTML(w, r)
			return
		}
		f, err := webFS.Open(path)
		if err != nil {
			serveHTML(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			serveHTML(w, r)
			return
		}
		if path == "index.html" {
			serveHTML(w, r)
			return
		}
		data, err := io.ReadAll(f)
		if err != nil {
			serveHTML(w, r)
			return
		}
		switch {
		case isPWABootstrap(path):
			// Service worker and manifest must revalidate on every load,
			// otherwise new deploys would never reach installed clients.
			w.Header().Set("Cache-Control", "no-cache")
		case strings.HasPrefix(path, "assets/"):
			// Vite fingerprints files in assets/, so they never change.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		default:
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		http.ServeContent(w, r, info.Name(), time.Time{}, bytes.NewReader(data))
	}
}

// isPWABootstrap reports whether the path is the service worker, its
// workbox chunks, or the web app manifest.
func isPWABootstrap(path string) bool {
	return path == "sw.js" ||
		path == "registerSW.js" ||
		path == "manifest.webmanifest" ||
		strings.HasPrefix(path, "workbox-")
}

func (s *Server) Start() error {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
