package httpserver

import (
	"context"
	"errors"
	"io/fs"
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

	if webFS != nil {
		r.NotFound(spaHandler(webFS))
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
// client-side routes like /room/CODE work on hard refresh.
func spaHandler(webFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := webFS.Open(path)
		if err != nil {
			index, idxErr := fs.ReadFile(webFS, "index.html")
			if idxErr != nil {
				http.Error(w, "frontend not built (run make build-web)", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(index)
			return
		}
		f.Close()
		http.FileServerFS(webFS).ServeHTTP(w, r)
	}
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
