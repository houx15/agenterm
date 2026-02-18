package automation

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/agenterm/internal/db"
)

func TestMergeControllerMergesCompletedTaskBranch(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	initTestRepo(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "base")
	defaultBranch := strings.TrimSpace(mustGitOutput(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	runGit(t, repo, "checkout", "-b", "feature/task-1")
	writeFile(t, filepath.Join(repo, "feature.txt"), "feature\n")
	runGit(t, repo, "add", "feature.txt")
	runGit(t, repo, "commit", "-m", "feature commit")
	runGit(t, repo, "checkout", defaultBranch)

	database := openMergeTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	worktreeRepo := db.NewWorktreeRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())

	project := &db.Project{Name: "p", RepoPath: repo, Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "t", Description: "d", Status: "completed"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	worktree := &db.Worktree{ProjectID: project.ID, BranchName: "feature/task-1", Path: repo, TaskID: task.ID, Status: "active"}
	if err := worktreeRepo.Create(ctx, worktree); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	task.WorktreeID = worktree.ID
	if err := taskRepo.Update(ctx, task); err != nil {
		t.Fatalf("update task with worktree: %v", err)
	}

	mc := NewMergeController(MergeControllerConfig{
		Interval:     time.Millisecond,
		TaskRepo:     taskRepo,
		WorktreeRepo: worktreeRepo,
		ProjectRepo:  projectRepo,
		SessionRepo:  sessionRepo,
	})
	mc.processTask(ctx, task.ID)

	updatedWT, err := worktreeRepo.Get(ctx, worktree.ID)
	if err != nil {
		t.Fatalf("get worktree: %v", err)
	}
	if updatedWT.Status != "merged" {
		t.Fatalf("worktree status=%q want merged", updatedWT.Status)
	}
	if _, err := gitOutput(ctx, repo, "merge-base", "--is-ancestor", "feature/task-1", defaultBranch); err != nil {
		t.Fatalf("expected feature branch commit merged into %s: %v", defaultBranch, err)
	}
}

func TestMergeControllerConflictReopensTaskAndNotifiesCoder(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	initTestRepo(t, repo)
	writeFile(t, filepath.Join(repo, "shared.txt"), "line-a\n")
	runGit(t, repo, "add", "shared.txt")
	runGit(t, repo, "commit", "-m", "base")
	defaultBranch := strings.TrimSpace(mustGitOutput(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, repo, "checkout", "-b", "feature/conflict")
	writeFile(t, filepath.Join(repo, "shared.txt"), "line-feature\n")
	runGit(t, repo, "add", "shared.txt")
	runGit(t, repo, "commit", "-m", "feature change")
	runGit(t, repo, "checkout", defaultBranch)
	writeFile(t, filepath.Join(repo, "shared.txt"), "line-master\n")
	runGit(t, repo, "add", "shared.txt")
	runGit(t, repo, "commit", "-m", "master change")

	database := openMergeTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	worktreeRepo := db.NewWorktreeRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())

	project := &db.Project{Name: "p2", RepoPath: repo, Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "t2", Description: "d", Status: "completed"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	worktree := &db.Worktree{ProjectID: project.ID, BranchName: "feature/conflict", Path: repo, TaskID: task.ID, Status: "active"}
	if err := worktreeRepo.Create(ctx, worktree); err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	task.WorktreeID = worktree.ID
	if err := taskRepo.Update(ctx, task); err != nil {
		t.Fatalf("update task with worktree: %v", err)
	}
	coder := &db.Session{
		TaskID:          task.ID,
		TmuxSessionName: "merge-conflict-coder",
		TmuxWindowID:    "@1",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "waiting_review",
	}
	if err := sessionRepo.Create(ctx, coder); err != nil {
		t.Fatalf("create coder session: %v", err)
	}

	var sent []string
	mc := NewMergeController(MergeControllerConfig{
		Interval:     time.Millisecond,
		TaskRepo:     taskRepo,
		WorktreeRepo: worktreeRepo,
		ProjectRepo:  projectRepo,
		SessionRepo:  sessionRepo,
		SendCommand: func(ctx context.Context, sessionID string, text string) error {
			if sessionID == coder.ID {
				sent = append(sent, text)
			}
			return nil
		},
	})
	mc.processTask(ctx, task.ID)

	updatedTask, err := taskRepo.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updatedTask.Status != "pending" {
		t.Fatalf("task status=%q want pending", updatedTask.Status)
	}
	if len(sent) != 1 {
		t.Fatalf("expected exactly one conflict notification, got %d", len(sent))
	}
	if !strings.Contains(sent[0], "Merge conflict detected") {
		t.Fatalf("unexpected conflict message: %q", sent[0])
	}
	// Merge must be aborted, leaving repository without unresolved conflict markers in index.
	status, err := gitOutput(ctx, repo, "status", "--porcelain")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.Contains(status, "UU ") {
		t.Fatalf("expected merge abort to clear unmerged index, got status: %q", status)
	}
}

func openMergeTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "merge-controller-test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Tester")
}

func mustGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitOutput(context.Background(), dir, args...)
	if err != nil {
		t.Fatalf("git output %v failed: %v", args, err)
	}
	return out
}
