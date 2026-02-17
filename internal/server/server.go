package server

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/user/agenterm/internal/config"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/web"
)

type Server struct {
	cfg        *config.Config
	db         *sql.DB
	httpServer *http.Server
}

func New(cfg *config.Config, h *hub.Hub, db *sql.DB, apiHandler http.Handler) (*Server, error) {
	mux := http.NewServeMux()

	subFS, err := fs.Sub(web.Assets, "frontend/dist")
	if err != nil {
		return nil, fmt.Errorf("failed to sub filesystem: %w", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "" || cleanPath == "." {
			cleanPath = "index.html"
		}

		if _, err := fs.Stat(subFS, cleanPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		fallbackReq := r.Clone(r.Context())
		fallbackURL := *r.URL
		fallbackURL.Path = "/index.html"
		fallbackReq.URL = &fallbackURL
		fileServer.ServeHTTP(w, fallbackReq)
	}))

	mux.HandleFunc("/ws", h.HandleWebSocket)
	if apiHandler != nil {
		mux.Handle("/api/", apiHandler)
	}

	return &Server{
		cfg: cfg,
		db:  db,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Port),
			Handler: mux,
		},
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}
