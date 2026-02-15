package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/user/agenterm/internal/config"
	"github.com/user/agenterm/internal/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	fmt.Printf("\nagenterm running at http://localhost:%d?token=%s\n\n", cfg.Port, cfg.Token)

	srv := server.New(cfg)
	if err := srv.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
