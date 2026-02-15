package server

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/agenterm/internal/config"
	"github.com/user/agenterm/web"
)

type Server struct {
	cfg    *config.Config
	server *http.Server
}

func New(cfg *config.Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	subFS, err := fs.Sub(web.Assets, ".")
	if err != nil {
		return fmt.Errorf("failed to sub filesystem: %w", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	mux.HandleFunc("/ws", s.handleWebSocket)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.cfg.Port),
		Handler: mux,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		slog.Info("shutdown signal received", "signal", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, "websocket upgrade not available", http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nWebSocket stub - connection accepted and closed\r\n")
}
