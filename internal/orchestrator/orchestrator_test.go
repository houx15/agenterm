package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/user/agenterm/internal/db"
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
