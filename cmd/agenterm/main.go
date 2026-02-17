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
	"unicode"

	"github.com/user/agenterm/internal/api"
	"github.com/user/agenterm/internal/config"
	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/orchestrator"
	"github.com/user/agenterm/internal/parser"
	"github.com/user/agenterm/internal/playbook"
	"github.com/user/agenterm/internal/registry"
	"github.com/user/agenterm/internal/server"
	"github.com/user/agenterm/internal/session"
	"github.com/user/agenterm/internal/tmux"
)

var version = "0.1.0"

type sessionRuntime struct {
	gateway *tmux.Gateway
	parser  *parser.Parser
}

type runtimeState struct {
	cfg            *config.Config
	manager        *tmux.Manager
	hub            *hub.Hub
	lifecycle      *session.Manager
	mu             sync.RWMutex
	sessions       map[string]*sessionRuntime
	windowToSessID map[string]string
}

func newRuntimeState(cfg *config.Config, manager *tmux.Manager, h *hub.Hub, lifecycle *session.Manager) *runtimeState {
	return &runtimeState{
		cfg:            cfg,
		manager:        manager,
		hub:            h,
		lifecycle:      lifecycle,
		sessions:       make(map[string]*sessionRuntime),
		windowToSessID: make(map[string]string),
	}
}

func (s *runtimeState) resolveSessionID(sessionID string, windowID string) string {
	if sessionID != "" {
		return sessionID
	}
	if windowID != "" {
		s.mu.RLock()
		mapped := s.windowToSessID[windowID]
		s.mu.RUnlock()
		if mapped != "" {
			return mapped
		}
	}
	return s.cfg.TmuxSession
}

func (s *runtimeState) ensureSession(ctx context.Context, sessionID string) (*sessionRuntime, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = s.cfg.TmuxSession
	}

	s.mu.RLock()
	if rt, ok := s.sessions[sessionID]; ok {
		s.mu.RUnlock()
		return rt, nil
	}
	s.mu.RUnlock()

	gw, err := s.manager.AttachSession(sessionID)
	if err != nil {
		return nil, err
	}

	rt := &sessionRuntime{gateway: gw, parser: parser.New()}

	s.mu.Lock()
	if existing, ok := s.sessions[sessionID]; ok {
		s.mu.Unlock()
		rt.parser.Close()
		return existing, nil
	}
	s.sessions[sessionID] = rt
	s.mu.Unlock()

	s.registerWindows(sessionID, gw.ListWindows())
	s.broadcastWindows()
	s.startSessionLoops(ctx, sessionID, rt)

	return rt, nil
}

func (s *runtimeState) startSessionLoops(ctx context.Context, sessionID string, rt *sessionRuntime) {
	go func() {
		for event := range rt.gateway.Events() {
			switch event.Type {
			case tmux.EventOutput:
				if event.WindowID != "" {
					s.setWindowSession(event.WindowID, sessionID)
					s.hub.BroadcastTerminal(hub.TerminalDataMessage{
						Type:      "terminal_data",
						SessionID: sessionID,
						Window:    event.WindowID,
						Text:      event.Data,
					})
				}
				rt.parser.Feed(event.WindowID, event.Data)
			case tmux.EventWindowAdd, tmux.EventWindowClose, tmux.EventWindowRenamed:
				s.registerWindows(sessionID, rt.gateway.ListWindows())
				s.broadcastWindows()
			}
		}
	}()

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

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				windows := rt.gateway.ListWindows()
				s.registerWindows(sessionID, windows)
				for _, w := range windows {
					status := rt.parser.Status(w.ID)
					s.hub.BroadcastStatusForSession(sessionID, w.ID, string(status))
				}
			}
		}
	}()
}

func (s *runtimeState) setWindowSession(windowID string, sessionID string) {
	s.mu.Lock()
	s.windowToSessID[windowID] = sessionID
	s.mu.Unlock()
}

func (s *runtimeState) registerWindows(sessionID string, windows []tmux.Window) {
	s.mu.Lock()
	for _, w := range windows {
		s.windowToSessID[w.ID] = sessionID
	}
	s.mu.Unlock()
}

func (s *runtimeState) broadcastWindows() {
	s.mu.RLock()
	all := make([]hub.WindowInfo, 0)
	for sessionID, rt := range s.sessions {
		all = append(all, convertWindows(sessionID, rt.gateway.ListWindows())...)
	}
	s.mu.RUnlock()
	s.hub.BroadcastWindows(all)
}

func (s *runtimeState) sendKeys(ctx context.Context, sessionID string, windowID string, keys string) error {
	targetSession := s.resolveSessionID(sessionID, windowID)
	rt, err := s.ensureSession(ctx, targetSession)
	if err != nil {
		return err
	}
	return rt.gateway.SendKeys(windowID, keys)
}

func (s *runtimeState) sendRaw(ctx context.Context, sessionID string, windowID string, keys string) error {
	targetSession := s.resolveSessionID(sessionID, windowID)
	rt, err := s.ensureSession(ctx, targetSession)
	if err != nil {
		return err
	}
	return rt.gateway.SendRaw(windowID, keys)
}

func (s *runtimeState) resizeWindow(ctx context.Context, sessionID string, windowID string, cols int, rows int) error {
	targetSession := s.resolveSessionID(sessionID, windowID)
	rt, err := s.ensureSession(ctx, targetSession)
	if err != nil {
		return err
	}
	return rt.gateway.ResizeWindow(windowID, cols, rows)
}

func (s *runtimeState) newWindow(ctx context.Context, sessionID string, name string) error {
	targetSession := s.resolveSessionID(sessionID, "")
	rt, err := s.ensureSession(ctx, targetSession)
	if err != nil {
		return err
	}
	if err := rt.gateway.NewWindow(name, s.cfg.DefaultDir); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	s.registerWindows(targetSession, rt.gateway.ListWindows())
	s.broadcastWindows()
	return nil
}

func (s *runtimeState) killWindow(ctx context.Context, sessionID string, windowID string) error {
	targetSession := s.resolveSessionID(sessionID, windowID)
	rt, err := s.ensureSession(ctx, targetSession)
	if err != nil {
		return err
	}
	if err := rt.gateway.KillWindow(windowID); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	s.broadcastWindows()
	return nil
}

func (s *runtimeState) close() {
	s.mu.Lock()
	parsers := make([]*parser.Parser, 0, len(s.sessions))
	for _, rt := range s.sessions {
		parsers = append(parsers, rt.parser)
	}
	s.sessions = make(map[string]*sessionRuntime)
	s.windowToSessID = make(map[string]string)
	s.mu.Unlock()

	s.manager.Close()
	for _, p := range parsers {
		p.Close()
	}
}

func (s *runtimeState) watchManagerSessions(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, sessionID := range s.manager.ListSessions() {
				_, _ = s.ensureSession(ctx, sessionID)
			}
		}
	}
}

func (s *runtimeState) defaultSessionWindows() []tmux.Window {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if rt, ok := s.sessions[s.cfg.TmuxSession]; ok {
		return rt.gateway.ListWindows()
	}
	return nil
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
	playbookRegistry, err := playbook.NewRegistry(cfg.PlaybooksDir)
	if err != nil {
		slog.Error("failed to initialize playbook registry", "dir", cfg.PlaybooksDir, "error", err)
		os.Exit(1)
	}

	manager := tmux.NewManager(cfg.DefaultDir)
	h := hub.New(cfg.Token, nil)
	lifecycleManager := session.NewManager(appDB.SQL(), manager, agentRegistry, h)
	state := newRuntimeState(cfg, manager, h, lifecycleManager)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if _, err := state.ensureSession(ctx, cfg.TmuxSession); err != nil {
		slog.Error("failed to attach default tmux session", "session", cfg.TmuxSession, "error", err)
		os.Exit(1)
	}

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
		if err := state.newWindow(ctx, sessionID, name); err != nil {
			slog.Error("failed to create window", "session", sessionID, "error", err)
		}
	})
	h.SetOnNewSessionWithSession(func(_ string, name string) {
		sessionName := normalizeSessionName(name)
		if sessionName == "" {
			slog.Error("failed to create session", "error", "session name is required")
			return
		}
		if _, err := manager.CreateSession(sessionName, cfg.DefaultDir); err != nil {
			// If already exists, attach instead.
			if _, attachErr := manager.AttachSession(sessionName); attachErr != nil {
				slog.Error("failed to create or attach session", "session", sessionName, "error", err)
				return
			}
		}
		if _, err := state.ensureSession(ctx, sessionName); err != nil {
			slog.Error("failed to activate session", "session", sessionName, "error", err)
			return
		}
		state.broadcastWindows()
	})
	h.SetOnKillWindowWithSession(func(sessionID string, windowID string) {
		if err := state.killWindow(ctx, sessionID, windowID); err != nil {
			slog.Error("failed to kill window", "session", sessionID, "window", windowID, "error", err)
		}
	})
	h.SetDefaultDir(cfg.DefaultDir)

	defaultGateway, err := manager.GetGateway(cfg.TmuxSession)
	if err != nil {
		slog.Error("failed to resolve default gateway", "session", cfg.TmuxSession, "error", err)
		os.Exit(1)
	}

	if lifecycleManager != nil {
		if err := lifecycleManager.Start(ctx); err != nil {
			slog.Error("failed to start session lifecycle manager", "error", err)
			os.Exit(1)
		}
	}

	projectRepo := db.NewProjectRepo(appDB.SQL())
	taskRepo := db.NewTaskRepo(appDB.SQL())
	worktreeRepo := db.NewWorktreeRepo(appDB.SQL())
	sessionRepo := db.NewSessionRepo(appDB.SQL())
	historyRepo := db.NewOrchestratorHistoryRepo(appDB.SQL())
	projectOrchestratorRepo := db.NewProjectOrchestratorRepo(appDB.SQL())
	workflowRepo := db.NewWorkflowRepo(appDB.SQL())
	knowledgeRepo := db.NewProjectKnowledgeRepo(appDB.SQL())
	roleBindingRepo := db.NewRoleBindingRepo(appDB.SQL())
	if projects, err := projectRepo.List(ctx, db.ProjectFilter{}); err == nil {
		for _, p := range projects {
			if p == nil {
				continue
			}
			_ = projectOrchestratorRepo.EnsureDefaultForProject(ctx, p.ID)
		}
	}

	orchestratorInst := orchestrator.New(orchestrator.Options{
		APIKey:                  cfg.LLMAPIKey,
		Model:                   cfg.LLMModel,
		AnthropicBaseURL:        cfg.LLMBaseURL,
		APIToolBaseURL:          fmt.Sprintf("http://127.0.0.1:%d", cfg.Port),
		APIToken:                cfg.Token,
		ProjectRepo:             projectRepo,
		TaskRepo:                taskRepo,
		WorktreeRepo:            worktreeRepo,
		SessionRepo:             sessionRepo,
		HistoryRepo:             historyRepo,
		ProjectOrchestratorRepo: projectOrchestratorRepo,
		WorkflowRepo:            workflowRepo,
		KnowledgeRepo:           knowledgeRepo,
		RoleBindingRepo:         roleBindingRepo,
		Registry:                agentRegistry,
		PlaybookRegistry:        playbookRegistry,
		GlobalMaxParallel:       cfg.OrchestratorGlobalMaxParallel,
	})

	h.SetOnOrchestratorChat(func(ctx context.Context, projectID string, message string) (<-chan hub.OrchestratorServerMessage, error) {
		stream, err := orchestratorInst.Chat(ctx, projectID, message)
		if err != nil {
			return nil, err
		}
		out := make(chan hub.OrchestratorServerMessage, 32)
		go func() {
			defer close(out)
			for evt := range stream {
				out <- hub.OrchestratorServerMessage{
					Type:   evt.Type,
					Text:   evt.Text,
					Name:   evt.Name,
					Args:   evt.Args,
					Result: evt.Result,
					Error:  evt.Error,
				}
			}
		}()
		return out, nil
	})

	eventTrigger := orchestrator.NewEventTrigger(orchestratorInst, sessionRepo, taskRepo, projectRepo, worktreeRepo)
	eventTrigger.SetOnEvent(func(projectID string, event string, data map[string]any) {
		h.BroadcastProjectEvent(projectID, event, data)
	})
	go eventTrigger.Start(ctx, 15*time.Second)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				projects, err := projectRepo.List(ctx, db.ProjectFilter{Status: "active"})
				if err != nil {
					continue
				}
				for _, project := range projects {
					eventTrigger.OnTimer(project.ID)
				}
			}
		}
	}()

	apiRouter := api.NewRouter(appDB.SQL(), defaultGateway, manager, lifecycleManager, h, orchestratorInst, cfg.Token, cfg.TmuxSession, agentRegistry, playbookRegistry)
	srv, err := server.New(cfg, h, appDB.SQL(), apiRouter)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	go h.Run(ctx)
	go state.watchManagerSessions(ctx)
	state.broadcastWindows()

	printStartupBanner(cfg, state.defaultSessionWindows())

	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	gracefulShutdown(state, h, lifecycleManager)
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

func convertWindows(sessionID string, windows []tmux.Window) []hub.WindowInfo {
	result := make([]hub.WindowInfo, len(windows))
	for i, w := range windows {
		result[i] = hub.WindowInfo{
			ID:        w.ID,
			SessionID: sessionID,
			Name:      w.Name,
			Status:    string(parser.StatusIdle),
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

func gracefulShutdown(state *runtimeState, h *hub.Hub, lifecycle *session.Manager) {
	slog.Info("shutting down...")

	h.FlushPendingOutput()
	if lifecycle != nil {
		lifecycle.Close()
	}
	state.close()

	slog.Info("agenterm stopped")
}

func normalizeSessionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
