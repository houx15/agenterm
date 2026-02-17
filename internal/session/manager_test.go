package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/registry"
	"github.com/user/agenterm/internal/tmux"
)

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
	manager := tmux.NewManager(t.TempDir())
	lifecycle := NewManager(database.SQL(), manager, reg, nil)
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

func TestManagerLifecycleWithRealTmux(t *testing.T) {
	if os.Getenv("RUN_TMUX_TESTS") != "1" {
		t.Skip("set RUN_TMUX_TESTS=1 to run tmux integration test")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux binary not found")
	}
	if out, err := exec.Command("tmux", "list-sessions").CombinedOutput(); err != nil {
		t.Skipf("tmux unavailable in test environment: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	database := openSessionTestDB(t)
	ctx := context.Background()

	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())

	now := time.Now().UTC()
	project := &db.Project{
		Name:      "Lifecycle",
		RepoPath:  t.TempDir(),
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{
		ProjectID:   project.ID,
		Title:       "session lifecycle",
		Description: "validate manager flows",
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if err := reg.Save(&registry.AgentConfig{
		ID:                "test-agent",
		Name:              "Test Agent",
		Model:             "test",
		Command:           "printf 'agent-ready\\n'",
		MaxParallelAgents: 1,
	}); err != nil {
		t.Fatalf("save test agent: %v", err)
	}

	tmuxMgr := tmux.NewManager(project.RepoPath)
	lifecycle := NewManager(database.SQL(), tmuxMgr, reg, nil)
	if err := lifecycle.Start(ctx); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	defer lifecycle.Close()

	sess, err := lifecycle.CreateSession(ctx, CreateSessionRequest{
		TaskID:    task.ID,
		AgentType: "test-agent",
		Role:      "coder",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer func() {
		_ = tmuxMgr.DestroySession(sess.TmuxSessionName)
	}()

	if !waitForPaneContains(t, sess.TmuxWindowID, "agent-ready", 2*time.Second) {
		t.Fatalf("initial agent command did not reach tmux pane")
	}

	if err := lifecycle.SendCommand(ctx, sess.ID, "printf 'from-send'\\n"); err != nil {
		t.Fatalf("send command: %v", err)
	}
	if !waitForPaneContains(t, sess.TmuxWindowID, "from-send", 2*time.Second) {
		t.Fatalf("sent command did not reach tmux pane")
	}

	if err := lifecycle.SetTakeover(ctx, sess.ID, true); err != nil {
		t.Fatalf("set takeover true: %v", err)
	}
	taken, err := sessionRepo.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get takeover session: %v", err)
	}
	if !taken.HumanAttached || taken.Status != "human_takeover" {
		t.Fatalf("takeover state = (%v, %q), want (true, human_takeover)", taken.HumanAttached, taken.Status)
	}

	if err := lifecycle.SetTakeover(ctx, sess.ID, false); err != nil {
		t.Fatalf("set takeover false: %v", err)
	}
	released, err := sessionRepo.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get released session: %v", err)
	}
	if released.HumanAttached {
		t.Fatalf("human_attached=true, want false")
	}

	if err := lifecycle.DestroySession(ctx, sess.ID); err != nil {
		t.Fatalf("destroy session: %v", err)
	}
	updated, err := sessionRepo.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get destroyed session: %v", err)
	}
	if updated.Status != "completed" {
		t.Fatalf("status=%q want completed", updated.Status)
	}

	cmd := exec.Command("tmux", "has-session", "-t", sess.TmuxSessionName)
	if err := cmd.Run(); err == nil {
		t.Fatalf("tmux session %q still exists after destroy", sess.TmuxSessionName)
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
	lifecycle := NewManager(database.SQL(), tmux.NewManager(t.TempDir()), reg, nil)
	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	defer lifecycle.Close()

	if err := lifecycle.ensureMonitorForSession(context.Background(), sess); err != nil {
		t.Fatalf("ensure monitor: %v", err)
	}
	ts := time.Now().UTC()
	lifecycle.ObserveParsedOutput(sess.TmuxSessionName, sess.TmuxWindowID, "line-1", "normal", ts)

	out, err := lifecycle.GetOutput(context.Background(), sess.ID, time.Time{})
	if err != nil {
		t.Fatalf("get output: %v", err)
	}
	if len(out) == 0 || out[len(out)-1].Text != "line-1" {
		t.Fatalf("output=%v want trailing line-1", out)
	}
}

func waitForPaneContains(t *testing.T, windowID string, want string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("tmux", "capture-pane", "-p", "-t", windowID, "-S", "-120").Output()
		if err == nil && strings.Contains(string(out), want) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
