package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/registry"
)

// fakeBackend implements TerminalBackend for tests.
type fakeBackend struct {
	sessions map[string]bool
	inputs   []string
	keys     []string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{sessions: make(map[string]bool)}
}

func (f *fakeBackend) CreateSession(_ context.Context, id, name, command, workDir string) (string, error) {
	f.sessions[id] = true
	return id, nil
}

func (f *fakeBackend) DestroySession(_ context.Context, id string) error {
	delete(f.sessions, id)
	return nil
}

func (f *fakeBackend) SendInput(_ context.Context, id, data string) error {
	f.inputs = append(f.inputs, id+":"+data)
	return nil
}

func (f *fakeBackend) SendKey(_ context.Context, id, key string) error {
	f.keys = append(f.keys, id+":"+key)
	return nil
}

func (f *fakeBackend) Resize(_ context.Context, id string, cols, rows int) error {
	return nil
}

func (f *fakeBackend) CaptureOutput(_ context.Context, id string, lines int) ([]string, error) {
	return nil, nil
}

func (f *fakeBackend) SessionExists(_ context.Context, id string) bool {
	return f.sessions[id]
}

func TestAutoAcceptSequence(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		want   string
		accept bool
	}{
		{name: "empty", mode: "", want: "", accept: false},
		{name: "optional disabled", mode: "optional", want: "", accept: false},
		{name: "supported newline", mode: "supported", want: "\n", accept: true},
		{name: "shift tab", mode: "shift+tab", want: "\x1b[Z", accept: true},
		{name: "ctrl c", mode: "ctrl+c", want: "\x03", accept: true},
		{name: "escaped newline", mode: `\n`, want: "\n", accept: true},
		{name: "raw text fallback", mode: "foo", want: "foo", accept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := autoAcceptSequence(tt.mode)
			if ok != tt.accept {
				t.Fatalf("autoAcceptSequence(%q) accepted=%v want %v", tt.mode, ok, tt.accept)
			}
			if got != tt.want {
				t.Fatalf("autoAcceptSequence(%q) sequence=%q want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestManagerCreateSessionRejectsUnknownAgentType(t *testing.T) {
	database := openSessionTestDB(t)
	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	backend := newFakeBackend()
	lifecycle := NewManager(database.SQL(), backend, reg, nil)
	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	defer lifecycle.Close()

	if _, err := lifecycle.CreateSession(context.Background(), CreateSessionRequest{
		TaskID:    "missing-task",
		AgentType: "missing-agent",
		Role:      "coder",
	}); err == nil || !strings.Contains(err.Error(), "unknown agent type") {
		t.Fatalf("CreateSession error=%v, want unknown agent type", err)
	}
}

func TestManagerStartResumesDeadSessionsWhenAgentSupportsResume(t *testing.T) {
	database := openSessionTestDB(t)
	sessionRepo := db.NewSessionRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	projectRepo := db.NewProjectRepo(database.SQL())

	created := time.Now().UTC().Add(-time.Minute)
	sess := seedSession(t, sessionRepo, taskRepo, projectRepo, created)

	// Mark session as suspended (simulating prior graceful shutdown).
	sess.Status = "suspended"
	if err := sessionRepo.Update(context.Background(), sess); err != nil {
		t.Fatalf("update session: %v", err)
	}

	// Register an agent that supports resume.
	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if err := reg.Save(&registry.AgentConfig{
		ID:                    "codex",
		Name:                  "Codex",
		Command:               "codex",
		ResumeCommand:         "codex --continue",
		SupportsSessionResume: true,
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Backend starts without the session (PTY is dead).
	backend := newFakeBackend()

	lifecycle := NewManager(database.SQL(), backend, reg, nil)
	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	defer lifecycle.Close()

	// The backend should now have the session re-created.
	if !backend.sessions[sess.ID] {
		t.Fatalf("session %q was not re-spawned in backend", sess.ID)
	}

	// The DB session should be updated to "working".
	updated, err := sessionRepo.Get(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Status != "working" {
		t.Fatalf("status=%q want working", updated.Status)
	}
}

func TestManagerStartTerminatesDeadSessionsWhenAgentDoesNotSupportResume(t *testing.T) {
	database := openSessionTestDB(t)
	sessionRepo := db.NewSessionRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	projectRepo := db.NewProjectRepo(database.SQL())

	created := time.Now().UTC().Add(-time.Minute)
	sess := seedSession(t, sessionRepo, taskRepo, projectRepo, created)

	// Session is "working" but PTY is dead (ungraceful shutdown).
	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	// Agent does NOT support resume.
	if err := reg.Save(&registry.AgentConfig{
		ID:      "codex",
		Name:    "Codex",
		Command: "codex",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	backend := newFakeBackend()

	lifecycle := NewManager(database.SQL(), backend, reg, nil)
	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	defer lifecycle.Close()

	updated, err := sessionRepo.Get(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Status != "terminated" {
		t.Fatalf("status=%q want terminated", updated.Status)
	}
}

func TestManagerCloseMarksSuspended(t *testing.T) {
	database := openSessionTestDB(t)
	sessionRepo := db.NewSessionRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	projectRepo := db.NewProjectRepo(database.SQL())

	created := time.Now().UTC().Add(-time.Minute)
	sess := seedSession(t, sessionRepo, taskRepo, projectRepo, created)

	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if err := reg.Save(&registry.AgentConfig{
		ID:      "codex",
		Name:    "Codex",
		Command: "codex",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	backend := newFakeBackend()
	backend.sessions[sess.ID] = true

	lifecycle := NewManager(database.SQL(), backend, reg, nil)
	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}

	// Close should mark the session as suspended.
	lifecycle.Close()

	updated, err := sessionRepo.Get(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Status != "suspended" {
		t.Fatalf("status=%q want suspended", updated.Status)
	}
}

func TestListActiveExcludesTerminalStatuses(t *testing.T) {
	database := openSessionTestDB(t)
	sessionRepo := db.NewSessionRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	projectRepo := db.NewProjectRepo(database.SQL())

	created := time.Now().UTC().Add(-time.Minute)
	ctx := context.Background()

	// Create a project and task for seeding sessions.
	project := &db.Project{Name: "P1", RepoPath: t.TempDir(), Status: "active", CreatedAt: created, UpdatedAt: created}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "T1", Description: "D1", Status: "pending", CreatedAt: created, UpdatedAt: created}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	statuses := []string{"working", "completed", "terminated", "failed", "suspended"}
	ids := make(map[string]string) // status -> id
	for _, status := range statuses {
		s := &db.Session{
			TaskID:          task.ID,
			TmuxSessionName: "t-" + status,
			TmuxWindowID:    "t-" + status,
			AgentType:       "codex",
			Role:            "coder",
			Status:          status,
			CreatedAt:       created,
			LastActivityAt:  created,
		}
		if err := sessionRepo.Create(ctx, s); err != nil {
			t.Fatalf("create session (%s): %v", status, err)
		}
		ids[status] = s.ID
	}

	active, err := sessionRepo.ListActive(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}

	activeIDs := make(map[string]bool)
	for _, s := range active {
		activeIDs[s.ID] = true
	}

	// "working" and "suspended" should be returned.
	if !activeIDs[ids["working"]] {
		t.Error("expected 'working' session to be active")
	}
	if !activeIDs[ids["suspended"]] {
		t.Error("expected 'suspended' session to be active")
	}
	// "completed", "terminated", "failed" should NOT be returned.
	if activeIDs[ids["completed"]] {
		t.Error("expected 'completed' session to be excluded")
	}
	if activeIDs[ids["terminated"]] {
		t.Error("expected 'terminated' session to be excluded")
	}
	if activeIDs[ids["failed"]] {
		t.Error("expected 'failed' session to be excluded")
	}
}

func TestManagerObserveParsedOutputFeedsRingBuffer(t *testing.T) {
	database := openSessionTestDB(t)
	sessionRepo := db.NewSessionRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	projectRepo := db.NewProjectRepo(database.SQL())

	created := time.Now().UTC().Add(-time.Minute)
	sess := seedSession(t, sessionRepo, taskRepo, projectRepo, created)

	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	backend := newFakeBackend()
	backend.sessions[sess.TmuxWindowID] = true
	lifecycle := NewManager(database.SQL(), backend, reg, nil)
	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	defer lifecycle.Close()

	if err := lifecycle.ensureMonitorForSession(context.Background(), sess); err != nil {
		t.Fatalf("ensure monitor: %v", err)
	}
	ts := time.Now().UTC()
	lifecycle.ObserveParsedOutput(sess.ID, sess.TmuxWindowID, "line-1", "normal", ts)

	out, err := lifecycle.GetOutput(context.Background(), sess.ID, time.Time{})
	if err != nil {
		t.Fatalf("get output: %v", err)
	}
	if len(out) == 0 || out[len(out)-1].Text != "line-1" {
		t.Fatalf("output=%v want trailing line-1", out)
	}
}
