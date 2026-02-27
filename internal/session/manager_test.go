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
