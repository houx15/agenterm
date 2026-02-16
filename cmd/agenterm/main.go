package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/user/agenterm/internal/config"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/parser"
	"github.com/user/agenterm/internal/server"
	"github.com/user/agenterm/internal/tmux"
)

var version = "0.1.0"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("agenterm v%s\n", version)
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	gw := tmux.New(cfg.TmuxSession)
	psr := parser.New()
	h := hub.New(cfg.Token, func(windowID, keys string) {
		if err := gw.SendKeys(windowID, keys); err != nil {
			slog.Error("failed to send keys", "window", windowID, "error", err)
		}
	})
	h.SetOnTerminalInput(func(windowID, keys string) {
		if err := gw.SendRaw(windowID, keys); err != nil {
			slog.Error("failed to send raw input", "window", windowID, "error", err)
		}
	})
	h.SetOnTerminalResize(func(windowID string, cols int, rows int) {
		if err := gw.ResizeWindow(windowID, cols, rows); err != nil {
			slog.Error("failed to resize terminal", "window", windowID, "cols", cols, "rows", rows, "error", err)
		}
	})
	h.SetOnNewWindow(func(name string) {
		if err := gw.NewWindow(name, cfg.DefaultDir); err != nil {
			slog.Error("failed to create window", "error", err)
		}
		time.Sleep(100 * time.Millisecond)
		h.BroadcastWindows(convertWindows(gw.ListWindows()))
	})
	h.SetOnKillWindow(func(windowID string) {
		if err := gw.KillWindow(windowID); err != nil {
			slog.Error("failed to kill window", "error", err)
		}
		time.Sleep(100 * time.Millisecond)
		h.BroadcastWindows(convertWindows(gw.ListWindows()))
	})
	h.SetDefaultDir(cfg.DefaultDir)
	srv, err := server.New(cfg, h)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := gw.Start(ctx); err != nil {
		slog.Error("Failed to connect to tmux", "error", err)
		os.Exit(1)
	}

	go h.Run(ctx)
	h.BroadcastWindows(convertWindows(gw.ListWindows()))
	go processEvents(ctx, gw, psr, h)

	printStartupBanner(cfg, gw.ListWindows())

	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	gracefulShutdown(gw, psr, h)
}

func printStartupBanner(cfg *config.Config, windows []tmux.Window) {
	var windowNames []string
	for _, w := range windows {
		windowNames = append(windowNames, w.Name)
	}
	windowsStr := "none"
	if len(windowNames) > 0 {
		windowsStr = fmt.Sprintf("%d (%s)", len(windowNames), strings.Join(windowNames, ", "))
	}

	fmt.Printf("\nagenterm v%s\n", version)
	fmt.Printf("  tmux session: %s\n", cfg.TmuxSession)
	fmt.Printf("  listening on: http://0.0.0.0:%d\n", cfg.Port)
	if cfg.PrintToken {
		fmt.Printf("  access URL:   http://localhost:%d?token=%s\n", cfg.Port, cfg.Token)
	} else {
		fmt.Printf("  access URL:   http://localhost:%d?token=<token>\n", cfg.Port)
		fmt.Printf("  (use --print-token to reveal token)\n")
	}
	fmt.Printf("  windows:      %s\n", windowsStr)
	fmt.Println("\nCtrl+C to stop")
}

func processEvents(ctx context.Context, gw *tmux.Gateway, psr *parser.Parser, h *hub.Hub) {
	go func() {
		for event := range gw.Events() {
			switch event.Type {
			case tmux.EventOutput:
				if event.WindowID != "" {
					h.BroadcastTerminal(hub.TerminalDataMessage{
						Type:   "terminal_data",
						Window: event.WindowID,
						Text:   event.Data,
					})
				}
				psr.Feed(event.WindowID, event.Data)
			case tmux.EventWindowAdd, tmux.EventWindowClose, tmux.EventWindowRenamed:
				h.BroadcastWindows(convertWindows(gw.ListWindows()))
			}
		}
	}()

	go func() {
		for msg := range psr.Messages() {
			h.BroadcastOutput(hub.OutputMessage{
				Type:    "output",
				Window:  msg.WindowID,
				Text:    msg.Text,
				Class:   string(msg.Class),
				Actions: convertActions(msg.Actions),
				ID:      msg.ID,
				Ts:      msg.Timestamp.Unix(),
			})
		}
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			windows := gw.ListWindows()
			for _, w := range windows {
				status := psr.Status(w.ID)
				h.BroadcastStatus(w.ID, string(status))
			}
		}
	}
}

func convertWindows(windows []tmux.Window) []hub.WindowInfo {
	result := make([]hub.WindowInfo, len(windows))
	for i, w := range windows {
		result[i] = hub.WindowInfo{
			ID:     w.ID,
			Name:   w.Name,
			Status: string(parser.StatusIdle),
		}
	}
	return result
}

func convertActions(actions []parser.QuickAction) []hub.ActionMessage {
	result := make([]hub.ActionMessage, len(actions))
	for i, a := range actions {
		result[i] = hub.ActionMessage{
			Label: a.Label,
			Keys:  a.Keys,
		}
	}
	return result
}

func gracefulShutdown(gw *tmux.Gateway, psr *parser.Parser, h *hub.Hub) {
	slog.Info("shutting down...")

	h.FlushPendingOutput()

	if err := gw.Stop(); err != nil {
		slog.Error("gateway stop error", "error", err)
	}

	psr.Close()

	slog.Info("agenterm stopped")
}
