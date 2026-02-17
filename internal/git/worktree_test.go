package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestCreateListAndRemoveWorktree(t *testing.T) {
	repo := initGitRepo(t)
	wtPath := filepath.Join(repo, ".worktrees", "task-1")
	if err := CreateWorktree(repo, wtPath, "feature/task-1"); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree path to exist: %v", err)
	}

	list, err := ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if !containsWorktree(list, wtPath, "feature/task-1") {
		t.Fatalf("created worktree not in list: %+v", list)
	}

	if err := RemoveWorktree(repo, wtPath); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree path removed, err=%v", err)
	}
}

func TestGetStatusParsesAllCategories(t *testing.T) {
	repo := initGitRepo(t)
	runGitCmd(t, repo, "checkout", "-b", "feature/status-fixture")
	if err := os.WriteFile(filepath.Join(repo, "delete-me.txt"), []byte("bye\n"), 0o644); err != nil {
		t.Fatalf("write delete fixture: %v", err)
	}
	runGitCmd(t, repo, "add", "delete-me.txt")
	runGitCmd(t, repo, "commit", "-m", "add delete fixture")

	wtPath := filepath.Join(repo, ".worktrees", "status-check")
	if err := CreateWorktree(repo, wtPath, "feature/status-check"); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	t.Cleanup(func() { _ = RemoveWorktree(repo, wtPath) })

	if err := os.WriteFile(filepath.Join(wtPath, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "added.txt"), []byte("added\n"), 0o644); err != nil {
		t.Fatalf("write added file: %v", err)
	}
	runGitCmd(t, wtPath, "add", "added.txt")
	if err := os.Remove(filepath.Join(wtPath, "delete-me.txt")); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	status, err := GetStatus(wtPath)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Clean {
		t.Fatalf("expected dirty status")
	}
	if !slices.Contains(status.Modified, "README.md") {
		t.Fatalf("expected modified README.md in %+v", status)
	}
	if !slices.Contains(status.Added, "added.txt") {
		t.Fatalf("expected added.txt in %+v", status)
	}
	if !slices.Contains(status.Deleted, "delete-me.txt") {
		t.Fatalf("expected delete-me.txt in %+v", status)
	}
	if !slices.Contains(status.Untracked, "untracked.txt") {
		t.Fatalf("expected untracked.txt in %+v", status)
	}
}

func TestGetLogReturnsStructuredCommits(t *testing.T) {
	repo := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "two.txt"), []byte("2\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	runGitCmd(t, repo, "add", "two.txt")
	runGitCmd(t, repo, "commit", "-m", "second commit")

	wtPath := filepath.Join(repo, ".worktrees", "log-check")
	if err := CreateWorktree(repo, wtPath, "feature/log-check"); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	t.Cleanup(func() { _ = RemoveWorktree(repo, wtPath) })

	commits, err := GetLog(wtPath, 2)
	if err != nil {
		t.Fatalf("GetLog failed: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("len(commits)=%d want 2", len(commits))
	}
	for _, commit := range commits {
		if commit.Hash == "" || commit.Message == "" || commit.Author == "" || commit.Date == "" {
			t.Fatalf("expected structured commit fields, got %+v", commit)
		}
	}
}

func TestCreateWorktreeErrorCases(t *testing.T) {
	notRepo := t.TempDir()
	wt := filepath.Join(t.TempDir(), "wt")
	if err := CreateWorktree(notRepo, wt, "feature/nope"); err == nil {
		t.Fatalf("expected error for non-git repo")
	}

	repo := initGitRepo(t)
	first := filepath.Join(repo, ".worktrees", "first")
	if err := CreateWorktree(repo, first, "feature/existing"); err != nil {
		t.Fatalf("first CreateWorktree failed: %v", err)
	}
	t.Cleanup(func() { _ = RemoveWorktree(repo, first) })

	second := filepath.Join(repo, ".worktrees", "second")
	if err := CreateWorktree(repo, second, "feature/existing"); err == nil {
		t.Fatalf("expected error for existing branch")
	}

	preExistingPath := filepath.Join(repo, ".worktrees", "already-there")
	if err := os.MkdirAll(preExistingPath, 0o755); err != nil {
		t.Fatalf("mkdir existing path: %v", err)
	}
	if err := CreateWorktree(repo, preExistingPath, "feature/path-exists"); err == nil {
		t.Fatalf("expected error for existing worktree path")
	}
}

func containsWorktree(worktrees []WorktreeInfo, path, branch string) bool {
	wantPath := canonicalPath(path)
	for _, wt := range worktrees {
		if canonicalPath(wt.Path) == wantPath && wt.Branch == branch {
			return true
		}
	}
	return false
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runGitCmd(t, repo, "init")
	runGitCmd(t, repo, "config", "user.email", "test@example.com")
	runGitCmd(t, repo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitCmd(t, repo, "add", "README.md")
	runGitCmd(t, repo, "commit", "-m", "init")
	return repo
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return path
}
