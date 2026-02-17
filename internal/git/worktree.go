package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	HEAD   string `json:"head"`
	Bare   bool   `json:"bare"`
}

type GitStatus struct {
	Modified  []string `json:"modified"`
	Added     []string `json:"added"`
	Deleted   []string `json:"deleted"`
	Untracked []string `json:"untracked"`
	Clean     bool     `json:"clean"`
}

type CommitInfo struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

func CreateWorktree(repoPath, worktreePath, branchName string) error {
	repoRoot, err := GetRepoRoot(repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	if err := validateBranchName(repoRoot, branchName); err != nil {
		return err
	}
	cleanPath, err := cleanAbsolutePath(worktreePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(cleanPath); err == nil {
		return fmt.Errorf("worktree path already exists: %s", cleanPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check worktree path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return fmt.Errorf("create worktree parent directory: %w", err)
	}
	if _, err := runGit(repoRoot, "worktree", "add", cleanPath, "-b", branchName); err != nil {
		return err
	}
	return nil
}

func RemoveWorktree(repoPath, worktreePath string) error {
	repoRoot, err := GetRepoRoot(repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	cleanPath, err := cleanAbsolutePath(worktreePath)
	if err != nil {
		return err
	}
	if _, err := runGit(repoRoot, "worktree", "remove", cleanPath); err != nil {
		return err
	}
	return nil
}

func ListWorktrees(repoPath string) ([]WorktreeInfo, error) {
	repoRoot, err := GetRepoRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	out, err := runGit(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out), nil
}

func GetStatus(worktreePath string) (*GitStatus, error) {
	cleanPath, err := cleanAbsolutePath(worktreePath)
	if err != nil {
		return nil, err
	}
	out, err := runGit(cleanPath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	status := parseGitStatus(out)
	return &status, nil
}

func GetLog(worktreePath string, count int) ([]CommitInfo, error) {
	cleanPath, err := cleanAbsolutePath(worktreePath)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		count = 20
	}
	if count > 100 {
		count = 100
	}
	out, err := runGit(cleanPath, "log", "-n", strconv.Itoa(count), "--pretty=format:%H%x1f%an%x1f%aI%x1f%s")
	if err != nil {
		return nil, err
	}
	return parseGitLog(out), nil
}

func GetDiff(worktreePath, ref1, ref2 string) (string, error) {
	cleanPath, err := cleanAbsolutePath(worktreePath)
	if err != nil {
		return "", err
	}
	ref1 = strings.TrimSpace(ref1)
	ref2 = strings.TrimSpace(ref2)
	if ref1 == "" || ref2 == "" {
		return "", fmt.Errorf("ref1 and ref2 are required")
	}
	return runGit(cleanPath, "diff", ref1+"..."+ref2)
}

func cleanAbsolutePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute: %s", path)
	}
	return filepath.Clean(path), nil
}

func validateBranchName(repoRoot, branchName string) error {
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return fmt.Errorf("branch name is required")
	}
	if _, err := runGit(repoRoot, "check-ref-format", "--branch", branchName); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", branchName, err)
	}
	return nil
}

func parseWorktreeList(output string) []WorktreeInfo {
	list := make([]WorktreeInfo, 0)
	curr := WorktreeInfo{}
	haveEntry := false

	flush := func() {
		if haveEntry {
			list = append(list, curr)
		}
		curr = WorktreeInfo{}
		haveEntry = false
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		key := parts[0]
		value := ""
		if len(parts) > 1 {
			value = strings.TrimSpace(parts[1])
		}
		haveEntry = true
		switch key {
		case "worktree":
			curr.Path = value
		case "HEAD":
			curr.HEAD = value
		case "branch":
			curr.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			curr.Bare = true
		}
	}
	flush()
	return list
}

func parseGitStatus(output string) GitStatus {
	status := GitStatus{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		file := strings.TrimSpace(line[3:])
		switch {
		case strings.HasPrefix(code, "??"):
			status.Untracked = append(status.Untracked, file)
		case strings.Contains(code, "A"):
			status.Added = append(status.Added, file)
		case strings.Contains(code, "D"):
			status.Deleted = append(status.Deleted, file)
		default:
			status.Modified = append(status.Modified, file)
		}
	}
	status.Clean = len(status.Modified) == 0 && len(status.Added) == 0 && len(status.Deleted) == 0 && len(status.Untracked) == 0
	return status
}

func parseGitLog(output string) []CommitInfo {
	commits := make([]CommitInfo, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
		})
	}
	return commits
}
