package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/playbook"
	"github.com/user/agenterm/internal/registry"
)

func openOrchestratorTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "orchestrator-test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestToolSchemasIncludeCoreDefinitions(t *testing.T) {
	schemas := NewToolset(&RESTToolClient{}).JSONSchemas()
	if len(schemas) < 8 {
		t.Fatalf("schemas len=%d want >= 8", len(schemas))
	}
	seenCreateProject := false
	for _, schema := range schemas {
		if schema["name"] == "create_project" {
			seenCreateProject = true
			break
		}
	}
	if !seenCreateProject {
		t.Fatalf("create_project schema missing")
	}
}

func TestBuildSystemPromptIncludesStateAndAgents(t *testing.T) {
	prompt := BuildSystemPrompt(&ProjectState{
		Project: &db.Project{ID: "p1", Name: "Demo", RepoPath: "/tmp/demo", Status: "active"},
		Tasks:   []*db.Task{{Status: "pending"}, {Status: "running"}},
	}, []*registry.AgentConfig{{ID: "codex", Name: "Codex", Capabilities: []string{"code"}, Languages: []string{"go"}, SpeedTier: "fast", CostTier: "medium"}}, nil)

	if !contains(prompt, "Demo") {
		t.Fatalf("prompt missing project name: %s", prompt)
	}
	if !contains(prompt, "codex") {
		t.Fatalf("prompt missing agent id: %s", prompt)
	}
	if !contains(prompt, "Never send commands to sessions in status human_takeover") {
		t.Fatalf("prompt missing safety rule")
	}
}

func TestChatToolExecutionLoop(t *testing.T) {
	database := openOrchestratorTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	worktreeRepo := db.NewWorktreeRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())
	historyRepo := db.NewOrchestratorHistoryRepo(database.SQL())

	project := &db.Project{Name: "Demo", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	var llmCalls atomic.Int32
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/messages" && req.Method == http.MethodPost {
				call := llmCalls.Add(1)
				if call == 1 {
					return jsonResponse(map[string]any{
						"content": []any{map[string]any{
							"type":  "tool_use",
							"id":    "tool-1",
							"name":  "get_project_status",
							"input": map[string]any{"project_id": project.ID},
						}},
					}), nil
				}
				return jsonResponse(map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "Plan created."}},
				}), nil
			}
			if req.URL.Path == "/api/projects/"+project.ID && req.Method == http.MethodGet {
				return jsonResponse(map[string]any{
					"project":   map[string]any{"id": project.ID, "name": project.Name, "repo_path": project.RepoPath, "status": project.Status},
					"tasks":     []any{},
					"worktrees": []any{},
					"sessions":  []any{},
				}), nil
			}
			return notFoundResponse(), nil
		}),
	}

	o := New(Options{
		APIKey:           "test-key",
		Model:            "test-model",
		AnthropicBaseURL: "http://mock/v1/messages",
		APIToolBaseURL:   "http://mock",
		ProjectRepo:      projectRepo,
		TaskRepo:         taskRepo,
		WorktreeRepo:     worktreeRepo,
		SessionRepo:      sessionRepo,
		HistoryRepo:      historyRepo,
		Registry:         nil,
		HTTPClient:       httpClient,
	})

	stream, err := o.Chat(context.Background(), project.ID, "create a plan")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	hasToolCall := false
	hasToolResult := false
	hasToken := false
	hasDone := false
	for evt := range stream {
		switch evt.Type {
		case "tool_call":
			hasToolCall = true
		case "tool_result":
			hasToolResult = true
		case "token":
			hasToken = true
		case "done":
			hasDone = true
		}
	}

	if !hasToolCall || !hasToolResult || !hasToken || !hasDone {
		t.Fatalf("events missing tool_call=%v tool_result=%v token=%v done=%v", hasToolCall, hasToolResult, hasToken, hasDone)
	}

	history, err := historyRepo.ListByProject(context.Background(), project.ID, 10)
	if err != nil {
		t.Fatalf("history list: %v", err)
	}
	if len(history) < 2 {
		t.Fatalf("history len=%d want >= 2", len(history))
	}
}

func TestEventTriggerOnSessionIdle(t *testing.T) {
	database := openOrchestratorTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	worktreeRepo := db.NewWorktreeRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())
	historyRepo := db.NewOrchestratorHistoryRepo(database.SQL())

	project := &db.Project{Name: "Demo", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "T1", Description: "D", Status: "pending"}
	if err := taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	sess := &db.Session{TaskID: task.ID, TmuxSessionName: "s", TmuxWindowID: "@1", AgentType: "codex", Role: "coder", Status: "idle"}
	if err := sessionRepo.Create(context.Background(), sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/messages" && req.Method == http.MethodPost {
				return jsonResponse(map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "Checked."}},
				}), nil
			}
			if req.URL.Path == "/api/projects/"+project.ID {
				return jsonResponse(map[string]any{
					"project":   map[string]any{"id": project.ID, "name": project.Name, "repo_path": project.RepoPath, "status": project.Status},
					"tasks":     []any{},
					"worktrees": []any{},
					"sessions":  []any{},
				}), nil
			}
			return notFoundResponse(), nil
		}),
	}

	o := New(Options{
		APIKey:           "test-key",
		AnthropicBaseURL: "http://mock/v1/messages",
		APIToolBaseURL:   "http://mock",
		ProjectRepo:      projectRepo,
		TaskRepo:         taskRepo,
		WorktreeRepo:     worktreeRepo,
		SessionRepo:      sessionRepo,
		HistoryRepo:      historyRepo,
		HTTPClient:       httpClient,
	})
	trigger := NewEventTrigger(o, sessionRepo, taskRepo, projectRepo, nil)
	trigger.OnSessionIdle(sess.ID)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		history, err := historyRepo.ListByProject(context.Background(), project.ID, 10)
		if err == nil && len(history) >= 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("expected orchestrator history to be written by trigger")
}

func TestExecuteToolRequiresExplicitSessionID(t *testing.T) {
	o := New(Options{
		Toolset: &Toolset{
			tools: map[string]Tool{
				"read_session_output": {
					Name: "read_session_output",
					Execute: func(ctx context.Context, args map[string]any) (any, error) {
						return map[string]any{"ok": true}, nil
					},
				},
			},
		},
	})

	_, err := o.executeTool(context.Background(), "read_session_output", map[string]any{})
	if err == nil {
		t.Fatalf("expected missing session_id error")
	}
	if !strings.Contains(err.Error(), "requires explicit session_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendCommandQueueSerializesPerSessionAndWritesLedger(t *testing.T) {
	var running atomic.Int32
	var maxRunning atomic.Int32
	var calls atomic.Int32

	ts := &Toolset{
		tools: map[string]Tool{
			"send_command": {
				Name: "send_command",
				Execute: func(ctx context.Context, args map[string]any) (any, error) {
					cur := running.Add(1)
					for {
						prev := maxRunning.Load()
						if cur <= prev {
							break
						}
						if maxRunning.CompareAndSwap(prev, cur) {
							break
						}
					}
					time.Sleep(40 * time.Millisecond)
					calls.Add(1)
					running.Add(-1)
					return map[string]any{"status": "sent"}, nil
				},
			},
		},
	}
	o := New(Options{Toolset: ts})

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := o.executeTool(context.Background(), "send_command", map[string]any{
				"session_id": "sess-1",
				"text":       "ls\n",
			})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("executeTool send_command failed: %v", err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d want 2", calls.Load())
	}
	if maxRunning.Load() != 1 {
		t.Fatalf("max concurrent send_command calls=%d want 1", maxRunning.Load())
	}

	ledger := o.RecentCommandLedger(10)
	if len(ledger) != 2 {
		t.Fatalf("ledger len=%d want 2", len(ledger))
	}
	for _, entry := range ledger {
		if entry.SessionID != "sess-1" {
			t.Fatalf("ledger session=%q want sess-1", entry.SessionID)
		}
		if entry.Status != "succeeded" {
			t.Fatalf("ledger status=%q want succeeded", entry.Status)
		}
		if entry.Command == "" {
			t.Fatalf("ledger command should be present")
		}
	}
}

func TestExecuteToolBlocksWhenRoleActionNotAllowed(t *testing.T) {
	database := openOrchestratorTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())

	pbRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	if err := pbRegistry.Save(&playbook.Playbook{
		ID:          "contract-playbook",
		Name:        "Contract Playbook",
		Description: "desc",
		Workflow: playbook.Workflow{
			Plan: playbook.Stage{Enabled: false, Roles: []playbook.StageRole{}},
			Build: playbook.Stage{Enabled: true, Roles: []playbook.StageRole{{
				Name:             "reviewer",
				Mode:             "reviewer",
				Responsibilities: "review",
				AllowedAgents:    []string{"codex"},
				ActionsAllowed:   []string{"read_session_output"},
			}}},
			Test: playbook.Stage{Enabled: false, Roles: []playbook.StageRole{}},
		},
	}); err != nil {
		t.Fatalf("save playbook: %v", err)
	}

	project := &db.Project{Name: "Demo", RepoPath: t.TempDir(), Status: "active", Playbook: "contract-playbook"}
	if err := projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "T1", Description: "D", Status: "running"}
	if err := taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	sess := &db.Session{TaskID: task.ID, TmuxSessionName: "s", TmuxWindowID: "@1", AgentType: "codex", Role: "reviewer", Status: "working"}
	if err := sessionRepo.Create(context.Background(), sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	o := New(Options{
		ProjectRepo:      projectRepo,
		TaskRepo:         taskRepo,
		SessionRepo:      sessionRepo,
		PlaybookRegistry: pbRegistry,
		Toolset: &Toolset{
			tools: map[string]Tool{
				"send_command": {
					Name: "send_command",
					Execute: func(ctx context.Context, args map[string]any) (any, error) {
						return map[string]any{"status": "sent"}, nil
					},
				},
			},
		},
	})

	_, err = o.executeTool(context.Background(), "send_command", map[string]any{
		"session_id": sess.ID,
		"text":       "ls\n",
	})
	if err == nil {
		t.Fatalf("expected role contract action error")
	}
	if !strings.Contains(err.Error(), "not allowed for role") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteToolBlocksWhenRoleInputsMissing(t *testing.T) {
	database := openOrchestratorTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())

	pbRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	if err := pbRegistry.Save(&playbook.Playbook{
		ID:          "contract-inputs-playbook",
		Name:        "Contract Inputs Playbook",
		Description: "desc",
		Workflow: playbook.Workflow{
			Plan: playbook.Stage{Enabled: false, Roles: []playbook.StageRole{}},
			Build: playbook.Stage{Enabled: true, Roles: []playbook.StageRole{{
				Name:             "worker",
				Mode:             "worker",
				Responsibilities: "code",
				AllowedAgents:    []string{"codex"},
				ActionsAllowed:   []string{"send_command"},
				InputsRequired:   []string{"spec_path"},
			}}},
			Test: playbook.Stage{Enabled: false, Roles: []playbook.StageRole{}},
		},
	}); err != nil {
		t.Fatalf("save playbook: %v", err)
	}

	project := &db.Project{Name: "Demo", RepoPath: t.TempDir(), Status: "active", Playbook: "contract-inputs-playbook"}
	if err := projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "T1", Description: "D", Status: "running", SpecPath: ""}
	if err := taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	sess := &db.Session{TaskID: task.ID, TmuxSessionName: "s", TmuxWindowID: "@1", AgentType: "codex", Role: "worker", Status: "working"}
	if err := sessionRepo.Create(context.Background(), sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	o := New(Options{
		ProjectRepo:      projectRepo,
		TaskRepo:         taskRepo,
		SessionRepo:      sessionRepo,
		PlaybookRegistry: pbRegistry,
		Toolset: &Toolset{
			tools: map[string]Tool{
				"send_command": {
					Name: "send_command",
					Execute: func(ctx context.Context, args map[string]any) (any, error) {
						return map[string]any{"status": "sent"}, nil
					},
				},
			},
		},
	})

	_, err = o.executeTool(context.Background(), "send_command", map[string]any{
		"session_id": sess.ID,
		"text":       "ls\n",
	})
	if err == nil {
		t.Fatalf("expected role required input error")
	}
	if !strings.Contains(err.Error(), "missing required role inputs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func contains(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(v any) *http.Response {
	buf, _ := json.Marshal(v)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(buf))),
	}
}

func notFoundResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
	}
}
