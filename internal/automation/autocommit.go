package automation

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
)

const defaultAutoCommitInterval = 30 * time.Second

type AutoCommitterConfig struct {
	Interval     time.Duration
	WorktreeRepo *db.WorktreeRepo
}

type AutoCommitter struct {
	interval     time.Duration
	worktreeRepo *db.WorktreeRepo

	onReadyForReview func(worktreeID string, commitHash string)

	mu             sync.RWMutex
	pausedWorktree map[string]bool
}

func NewAutoCommitter(cfg AutoCommitterConfig) *AutoCommitter {
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultAutoCommitInterval
	}
	return &AutoCommitter{
		interval:       interval,
		worktreeRepo:   cfg.WorktreeRepo,
		pausedWorktree: make(map[string]bool),
	}
}

func (ac *AutoCommitter) SetOnReadyForReview(fn func(worktreeID string, commitHash string)) {
	if ac == nil {
		return
	}
	ac.mu.Lock()
	ac.onReadyForReview = fn
	ac.mu.Unlock()
}

func (ac *AutoCommitter) SetWorktreePaused(worktreeID string, paused bool) {
	if ac == nil || strings.TrimSpace(worktreeID) == "" {
		return
	}
	ac.mu.Lock()
	ac.pausedWorktree[worktreeID] = paused
	ac.mu.Unlock()
}

func (ac *AutoCommitter) Run(ctx context.Context) {
	if ac == nil || ac.worktreeRepo == nil {
		return
	}
	ticker := time.NewTicker(ac.interval)
	defer ticker.Stop()

	ac.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ac.runOnce(ctx)
		}
	}
}

func (ac *AutoCommitter) runOnce(ctx context.Context) {
	if ac == nil || ac.worktreeRepo == nil {
		return
	}
	items, err := ac.worktreeRepo.List(ctx, db.WorktreeFilter{Status: "active"})
	if err != nil {
		return
	}
	for _, wt := range items {
		if wt == nil || strings.TrimSpace(wt.Path) == "" {
			continue
		}
		if ac.isPaused(wt.ID) {
			continue
		}
		commitHash, changedFiles, err := autoCommitWorktree(ctx, wt.Path)
		if err != nil || commitHash == "" {
			continue
		}
		if includesTaskFile(changedFiles) {
			ac.mu.RLock()
			notify := ac.onReadyForReview
			ac.mu.RUnlock()
			if notify != nil {
				notify(wt.ID, commitHash)
			}
		}
	}
}

func (ac *AutoCommitter) isPaused(worktreeID string) bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.pausedWorktree[worktreeID]
}

func includesTaskFile(files []string) bool {
	for _, file := range files {
		if strings.EqualFold(strings.TrimSpace(file), "TASK.md") {
			return true
		}
	}
	return false
}

func autoCommitWorktree(ctx context.Context, worktreePath string) (string, []string, error) {
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return "", nil, fmt.Errorf("worktree path is required")
	}

	statusOut, err := gitOutput(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return "", nil, err
	}
	statusOut = strings.TrimSpace(statusOut)
	if statusOut == "" {
		return "", nil, nil
	}
	if hasUnmergedStatus(statusOut) {
		return "", nil, nil
	}

	if _, err := gitOutput(ctx, worktreePath, "add", "-A"); err != nil {
		return "", nil, err
	}
	if _, err := gitOutput(ctx, worktreePath, "diff", "--cached", "--quiet"); err == nil {
		return "", nil, nil
	}

	msg := fmt.Sprintf("[auto] checkpoint %s", time.Now().UTC().Format(time.RFC3339))
	if _, err := gitOutput(ctx, worktreePath, "commit", "-m", msg); err != nil {
		return "", nil, nil
	}
	hash, err := gitOutput(ctx, worktreePath, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return "", nil, err
	}
	filesRaw, err := gitOutput(ctx, worktreePath, "show", "--pretty=", "--name-only", "HEAD")
	if err != nil {
		return strings.TrimSpace(hash), nil, nil
	}
	files := splitNonEmptyLines(filesRaw)
	return strings.TrimSpace(hash), files, nil
}

func hasUnmergedStatus(porcelain string) bool {
	for _, line := range splitNonEmptyLines(porcelain) {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') {
			return true
		}
	}
	return false
}

func splitNonEmptyLines(raw string) []string {
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func gitOutput(ctx context.Context, worktreePath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", worktreePath}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}
