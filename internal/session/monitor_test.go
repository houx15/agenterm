package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/agenterm/internal/db"
)

func openSessionTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "session-monitor-test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func seedSession(t *testing.T, sessionRepo *db.SessionRepo, taskRepo *db.TaskRepo, projectRepo *db.ProjectRepo, createdAt time.Time) *db.Session {
	t.Helper()
	ctx := context.Background()

	project := &db.Project{Name: "P1", RepoPath: t.TempDir(), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &db.Task{
		ProjectID:   project.ID,
		Title:       "T1",
		Description: "D1",
		Status:      "pending",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	sess := &db.Session{
		TaskID:          task.ID,
		TmuxSessionName: "tmux-test",
		TmuxWindowID:    "@1",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "working",
		CreatedAt:       createdAt,
		LastActivityAt:  createdAt,
	}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return sess
}

func TestMonitorDetectStatusPromptBeatsMarker(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".orchestra"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, ".orchestra", "done"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	m := NewMonitor(MonitorConfig{
		SessionID:    "s1",
		TmuxSession:  "tmux-s1",
		WindowID:     "@1",
		WorkDir:      workDir,
		IdleTimeout:  30 * time.Second,
		PollInterval: 10 * time.Millisecond,
	})
	m.buffer.Add(OutputEntry{Text: "$ ", Timestamp: time.Now().UTC()})

	if got := m.detectStatus(); got != "waiting_review" {
		t.Fatalf("detectStatus()=%q want waiting_review", got)
	}
}

func TestMonitorDetectStatusIdleBeatsMarker(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".orchestra"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, ".orchestra", "done"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	m := NewMonitor(MonitorConfig{
		SessionID:    "s2",
		TmuxSession:  "tmux-s2",
		WindowID:     "@2",
		WorkDir:      workDir,
		IdleTimeout:  10 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
	})
	m.mu.Lock()
	m.lastOutput = time.Now().UTC().Add(-time.Second)
	m.mu.Unlock()

	if got := m.detectStatus(); got != "idle" {
		t.Fatalf("detectStatus()=%q want idle", got)
	}
}

func TestMonitorRunRefreshesLastActivityWithoutStatusChange(t *testing.T) {
	database := openSessionTestDB(t)
	sessionRepo := db.NewSessionRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	projectRepo := db.NewProjectRepo(database.SQL())

	old := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	sess := seedSession(t, sessionRepo, taskRepo, projectRepo, old)

	origExists := tmuxSessionExistsFn
	origCapture := capturePaneFn
	t.Cleanup(func() {
		tmuxSessionExistsFn = origExists
		capturePaneFn = origCapture
	})

	tmuxSessionExistsFn = func(string) bool { return true }
	capturePaneFn = func(string, int) ([]string, error) { return []string{"agent running"}, nil }

	m := NewMonitor(MonitorConfig{
		SessionID:    sess.ID,
		TmuxSession:  sess.TmuxSessionName,
		WindowID:     sess.TmuxWindowID,
		SessionRepo:  sessionRepo,
		IdleTimeout:  10 * time.Second,
		PollInterval: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()

	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done

	updated, err := sessionRepo.Get(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}
	if !updated.LastActivityAt.After(old) {
		t.Fatalf("last_activity_at=%v, want > %v", updated.LastActivityAt, old)
	}
}
