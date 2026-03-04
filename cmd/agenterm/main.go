package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/user/agenterm/internal/api"
	"github.com/user/agenterm/internal/config"
	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/parser"
	"github.com/user/agenterm/internal/pty"
	"github.com/user/agenterm/internal/registry"
	"github.com/user/agenterm/internal/server"
	"github.com/user/agenterm/internal/session"
)

var version = "0.1.0"

// sessionRuntime holds the parser for a single PTY session.
type sessionRuntime struct {
	parser *parser.Parser
}

type runtimeState struct {
	cfg       *config.Config
	backend   *pty.Backend
	hub       *hub.Hub
	lifecycle *session.Manager

	mu       sync.RWMutex
	sessions map[string]*sessionRuntime
}

func newRuntimeState(cfg *config.Config, backend *pty.Backend, h *hub.Hub, lifecycle *session.Manager) *runtimeState {
	return &runtimeState{
		cfg:       cfg,
		backend:   backend,
		hub:       h,
		lifecycle: lifecycle,
		sessions:  make(map[string]*sessionRuntime),
	}
}

// ensureSessionLoop starts reading events from the PTY backend for
// the given session and forwards them to the hub and parser.
func (s *runtimeState) ensureSessionLoop(ctx context.Context, sessionID string) {
	s.mu.Lock()
	if _, ok := s.sessions[sessionID]; ok {
		s.mu.Unlock()
		return
	}
	rt := &sessionRuntime{parser: parser.New()}
	s.sessions[sessionID] = rt
	s.mu.Unlock()

	s.broadcastWindows()

	events := s.backend.Events(sessionID)
	if events == nil {
		return
	}

	// Event forwarding loop.
	go func() {
		for evt := range events {
			switch evt.Type {
			case pty.EventOutput:
				s.hub.BroadcastTerminal(hub.TerminalDataMessage{
					Type:      "terminal_data",
					SessionID: sessionID,
					Window:    sessionID,
					Text:      evt.Data,
				})
				rt.parser.Feed(sessionID, evt.Data)
			case pty.EventClosed:
				s.broadcastWindows()
			}
		}
		// Session ended — clean up.
		s.mu.Lock()
		if current := s.sessions[sessionID]; current == rt {
			delete(s.sessions, sessionID)
		}
		s.mu.Unlock()
		rt.parser.Close()
		s.broadcastWindows()
	}()

	// Parser output forwarding.
	go func() {
		for msg := range rt.parser.Messages() {
			if s.lifecycle != nil {
				s.lifecycle.ObserveParsedOutput(sessionID, msg.WindowID, msg.Text, string(msg.Class), msg.Timestamp)
			}
			s.hub.BroadcastOutput(hub.OutputMessage{
				Type:      "output",
				SessionID: sessionID,
				Window:    msg.WindowID,
				Text:      msg.Text,
				Class:     string(msg.Class),
				Actions:   convertActions(msg.Actions),
				ID:        msg.ID,
				Ts:        msg.Timestamp.Unix(),
			})
		}
	}()

	// Periodic status broadcast.
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mu.RLock()
				_, alive := s.sessions[sessionID]
				s.mu.RUnlock()
				if !alive {
					return
				}
				status := rt.parser.Status(sessionID)
				s.hub.BroadcastStatusForSession(sessionID, sessionID, string(status))
			}
		}
	}()
}

func (s *runtimeState) broadcastWindows() {
	infos := s.backend.Manager().ListSessions()
	windows := make([]hub.WindowInfo, 0, len(infos))
	for _, info := range infos {
		if !info.Active {
			continue
		}
		windows = append(windows, hub.WindowInfo{
			ID:        info.ID,
			SessionID: info.ID,
			Name:      info.Name,
			Status:    string(parser.StatusIdle),
		})
	}
	s.hub.BroadcastWindows(windows)
}

func (s *runtimeState) sendKeys(_ context.Context, sessionID string, windowID string, keys string) error {
	id := sessionID
	if id == "" {
		id = windowID
	}
	return s.backend.SendInput(context.Background(), id, keys)
}

func (s *runtimeState) sendRaw(_ context.Context, sessionID string, windowID string, keys string) error {
	id := sessionID
	if id == "" {
		id = windowID
	}
	return s.backend.SendInput(context.Background(), id, keys)
}

func (s *runtimeState) resizeWindow(_ context.Context, sessionID string, windowID string, cols int, rows int) error {
	id := sessionID
	if id == "" {
		id = windowID
	}
	return s.backend.Resize(context.Background(), id, cols, rows)
}

func (s *runtimeState) close() {
	s.mu.Lock()
	parsers := make([]*parser.Parser, 0, len(s.sessions))
	for _, rt := range s.sessions {
		parsers = append(parsers, rt.parser)
	}
	s.sessions = make(map[string]*sessionRuntime)
	s.mu.Unlock()

	s.backend.Close()
	for _, p := range parsers {
		p.Close()
	}
}

// watchManagedSessions polls PTY sessions created by the lifecycle manager
// and ensures each one has an event-forwarding loop.
func (s *runtimeState) watchManagedSessions(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, info := range s.backend.Manager().ListSessions() {
				if info.Active {
					s.ensureSessionLoop(ctx, info.ID)
				}
			}
		}
	}
}

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

	appDB, err := db.Open(context.Background(), cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize database", "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := appDB.Close(); err != nil {
			slog.Error("failed to close database", "error", err)
		}
	}()

	agentRegistry, err := registry.NewRegistry(cfg.AgentsDir)
	if err != nil {
		slog.Error("failed to initialize agent registry", "dir", cfg.AgentsDir, "error", err)
		os.Exit(1)
	}
	backend := pty.NewBackend()
	h := hub.New(cfg.Token, nil)
	lifecycleManager := session.NewManager(appDB.SQL(), backend, agentRegistry, h)
	state := newRuntimeState(cfg, backend, h, lifecycleManager)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Hub callbacks for terminal I/O ---

	h.SetOnInputWithSession(func(sessionID string, windowID string, keys string) {
		if err := state.sendKeys(ctx, sessionID, windowID, keys); err != nil {
			slog.Error("failed to send keys", "session", sessionID, "window", windowID, "error", err)
		}
	})
	h.SetOnTerminalInputWithSession(func(sessionID string, windowID string, keys string) {
		if err := state.sendRaw(ctx, sessionID, windowID, keys); err != nil {
			slog.Error("failed to send raw input", "session", sessionID, "window", windowID, "error", err)
		}
	})
	h.SetOnTerminalResizeWithSession(func(sessionID string, windowID string, cols int, rows int) {
		if err := state.resizeWindow(ctx, sessionID, windowID, cols, rows); err != nil {
			slog.Error("failed to resize terminal", "session", sessionID, "window", windowID, "cols", cols, "rows", rows, "error", err)
		}
	})
	h.SetOnNewWindowWithSession(func(sessionID string, name string) {
		// In PTY mode, new windows are created as new sessions via the API.
		slog.Debug("new window request ignored in PTY mode", "session", sessionID, "name", name)
	})
	h.SetOnNewSessionWithSession(func(_ string, name string) {
		// In PTY mode, sessions are created through the lifecycle manager.
		slog.Debug("new session request ignored in PTY mode", "name", name)
	})
	h.SetOnKillWindowWithSession(func(sessionID string, windowID string) {
		id := sessionID
		if id == "" {
			id = windowID
		}
		if err := backend.DestroySession(ctx, id); err != nil {
			slog.Error("failed to kill session", "session", sessionID, "window", windowID, "error", err)
		}
		state.broadcastWindows()
	})
	h.SetDefaultDir(cfg.DefaultDir)

	// --- Start lifecycle manager ---

	if lifecycleManager != nil {
		if err := lifecycleManager.Start(ctx); err != nil {
			slog.Error("failed to start session lifecycle manager", "error", err)
			os.Exit(1)
		}
	}

	// --- Attach/Detach callbacks ---

	h.SetOnTerminalAttach(func(sessionID string) {
		if strings.TrimSpace(sessionID) == "" {
			return
		}
		callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer callCancel()
		if lifecycleManager != nil {
			if err := lifecycleManager.SetTakeover(callCtx, sessionID, true); err != nil {
				slog.Debug("failed to set human takeover", "session", sessionID, "error", err)
			}
		}
	})

	h.SetOnTerminalDetach(func(sessionID string) {
		if strings.TrimSpace(sessionID) == "" {
			return
		}
		callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer callCancel()
		if lifecycleManager != nil {
			if err := lifecycleManager.SetTakeover(callCtx, sessionID, false); err != nil {
				slog.Debug("failed to clear human takeover", "session", sessionID, "error", err)
			}
		}
	})

	// --- Server ---

	apiRouter := api.NewRouter(appDB.SQL(), lifecycleManager, h, cfg.Token, agentRegistry)
	srv, err := server.New(cfg, h, appDB.SQL(), apiRouter)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	go h.Run(ctx)
	go state.watchManagedSessions(ctx)
	state.broadcastWindows()

	printStartupBanner(cfg)

	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	gracefulShutdown(state, h, lifecycleManager)
}

func printStartupBanner(cfg *config.Config) {
	fmt.Printf("\nagenterm v%s\n", version)
	fmt.Printf("  backend:      pty\n")
	fmt.Printf("  listening on: http://0.0.0.0:%d\n", cfg.Port)
	if cfg.PrintToken {
		fmt.Printf("  access URL:   http://localhost:%d?token=%s\n", cfg.Port, cfg.Token)
	} else {
		fmt.Printf("  access URL:   http://localhost:%d?token=<token>\n", cfg.Port)
		fmt.Printf("  (use --print-token to reveal token)\n")
	}
	fmt.Println("\nCtrl+C to stop")
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

func gracefulShutdown(state *runtimeState, h *hub.Hub, lifecycle *session.Manager) {
	slog.Info("shutting down...")

	h.FlushPendingOutput()
	if lifecycle != nil {
		lifecycle.Close()
	}
	state.close()

	slog.Info("agenterm stopped")
}
