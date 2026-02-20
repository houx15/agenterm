package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		op := strings.Join(args, " ")
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git %s failed: %s", op, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("git %s failed: %w", op, err)
	}

	return string(out), nil
}

func IsGitRepo(path string) bool {
	out, err := runGit(path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

func GetRepoRoot(path string) (string, error) {
	out, err := runGit(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", fmt.Errorf("git repo root is empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	return filepath.Clean(absRoot), nil
}

func EnsureRepoInitialized(path string) (repoRoot string, initialized bool, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false, fmt.Errorf("path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("resolve path: %w", err)
	}
	absPath = filepath.Clean(absPath)
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return "", false, fmt.Errorf("create repo directory: %w", err)
	}
	if IsGitRepo(absPath) {
		root, err := GetRepoRoot(absPath)
		return root, false, err
	}
	if _, err := runGit(absPath, "init"); err != nil {
		return "", false, err
	}
	root, err := GetRepoRoot(absPath)
	if err != nil {
		return "", false, err
	}
	return root, true, nil
}
