package automation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHasUnmergedStatus(t *testing.T) {
	if !hasUnmergedStatus("UU file.txt") {
		t.Fatalf("expected UU to be treated as unmerged")
	}
	if hasUnmergedStatus(" M file.txt") {
		t.Fatalf("did not expect regular modified status to be unmerged")
	}
}

func TestAutoCommitWorktreeCreatesCommit(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Tester")

	path := filepath.Join(repo, "TASK.md")
	writeFile(t, path, "# Task\n")
	runGit(t, repo, "add", "TASK.md")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, path, "# Task\n- [x] done\n")

	hash, files, err := autoCommitWorktree(ctx, repo)
	if err != nil {
		t.Fatalf("autoCommitWorktree error: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected commit hash")
	}
	if !includesTaskFile(files) {
		t.Fatalf("expected changed files to include TASK.md, got %v", files)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
