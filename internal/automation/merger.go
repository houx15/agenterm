package automation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
)

const defaultMergeInterval = 10 * time.Second

type MergeControllerConfig struct {
	Interval     time.Duration
	TaskRepo     *db.TaskRepo
	WorktreeRepo *db.WorktreeRepo
	ProjectRepo  *db.ProjectRepo
	SessionRepo  *db.SessionRepo
	SendCommand  func(ctx context.Context, sessionID string, text string) error
}

type MergeController struct {
	interval     time.Duration
	taskRepo     *db.TaskRepo
	worktreeRepo *db.WorktreeRepo
	projectRepo  *db.ProjectRepo
	sessionRepo  *db.SessionRepo
	sendCommand  func(ctx context.Context, sessionID string, text string) error

	onMerged   func(projectID, taskID, worktreeID, sourceBranch, targetBranch, commitHash string)
	onConflict func(projectID, taskID, worktreeID, sourceBranch, targetBranch, commitHash string, files []string)

	mu               sync.Mutex
	inFlightTasks    map[string]bool
	conflictNotified map[string]string // task_id -> source commit hash already notified
	projectLocks     map[string]*sync.Mutex
}

type mergeAttemptResult struct {
	SourceCommit  string
	SourceBranch  string
	TargetBranch  string
	Conflict      bool
	ConflictFiles []string
	Merged        bool
}

func NewMergeController(cfg MergeControllerConfig) *MergeController {
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultMergeInterval
	}
	return &MergeController{
		interval:         interval,
		taskRepo:         cfg.TaskRepo,
		worktreeRepo:     cfg.WorktreeRepo,
		projectRepo:      cfg.ProjectRepo,
		sessionRepo:      cfg.SessionRepo,
		sendCommand:      cfg.SendCommand,
		inFlightTasks:    make(map[string]bool),
		conflictNotified: make(map[string]string),
		projectLocks:     make(map[string]*sync.Mutex),
	}
}

func (mc *MergeController) SetOnMerged(fn func(projectID, taskID, worktreeID, sourceBranch, targetBranch, commitHash string)) {
	if mc == nil {
		return
	}
	mc.mu.Lock()
	mc.onMerged = fn
	mc.mu.Unlock()
}

func (mc *MergeController) SetOnConflict(fn func(projectID, taskID, worktreeID, sourceBranch, targetBranch, commitHash string, files []string)) {
	if mc == nil {
		return
	}
	mc.mu.Lock()
	mc.onConflict = fn
	mc.mu.Unlock()
}

func (mc *MergeController) Run(ctx context.Context) {
	if mc == nil || mc.taskRepo == nil || mc.worktreeRepo == nil || mc.projectRepo == nil {
		return
	}
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	mc.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.runOnce(ctx)
		}
	}
}

func (mc *MergeController) runOnce(ctx context.Context) {
	if mc == nil || mc.taskRepo == nil {
		return
	}
	tasks, err := mc.taskRepo.List(ctx, db.TaskFilter{})
	if err != nil {
		return
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if !isMergeCandidateTaskStatus(task.Status) {
			continue
		}
		if strings.TrimSpace(task.WorktreeID) == "" {
			continue
		}
		if mc.tryStartTask(task.ID) {
			continue
		}
		go func(taskID string) {
			defer mc.finishTask(taskID)
			mc.processTask(context.Background(), taskID)
		}(task.ID)
	}
}

func isMergeCandidateTaskStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "completed" || status == "done"
}

func (mc *MergeController) tryStartTask(taskID string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.inFlightTasks[taskID] {
		return true
	}
	mc.inFlightTasks[taskID] = true
	return false
}

func (mc *MergeController) finishTask(taskID string) {
	mc.mu.Lock()
	delete(mc.inFlightTasks, taskID)
	mc.mu.Unlock()
}

func (mc *MergeController) processTask(ctx context.Context, taskID string) {
	if mc == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	task, err := mc.taskRepo.Get(ctx, taskID)
	if err != nil || task == nil || strings.TrimSpace(task.WorktreeID) == "" || !isMergeCandidateTaskStatus(task.Status) {
		return
	}
	worktree, err := mc.worktreeRepo.Get(ctx, task.WorktreeID)
	if err != nil || worktree == nil || strings.TrimSpace(worktree.BranchName) == "" {
		return
	}
	if strings.EqualFold(strings.TrimSpace(worktree.Status), "merged") {
		return
	}
	project, err := mc.projectRepo.Get(ctx, task.ProjectID)
	if err != nil || project == nil || strings.TrimSpace(project.RepoPath) == "" {
		return
	}
	if mc.hasHumanAttachedSession(ctx, task.ID) {
		return
	}

	lock := mc.projectLock(project.ID)
	lock.Lock()
	defer lock.Unlock()

	result, err := mergeWorktreeBranch(ctx, project.RepoPath, worktree.BranchName)
	if err != nil {
		return
	}
	if result.Merged {
		worktree.Status = "merged"
		_ = mc.worktreeRepo.Update(ctx, worktree)

		mc.mu.Lock()
		delete(mc.conflictNotified, task.ID)
		onMerged := mc.onMerged
		mc.mu.Unlock()
		if onMerged != nil {
			onMerged(project.ID, task.ID, worktree.ID, result.SourceBranch, result.TargetBranch, result.SourceCommit)
		}
		return
	}
	if !result.Conflict {
		return
	}

	if strings.ToLower(strings.TrimSpace(task.Status)) != "pending" {
		task.Status = "pending"
		_ = mc.taskRepo.Update(ctx, task)
	}
	if mc.shouldNotifyConflict(task.ID, result.SourceCommit) {
		mc.notifyCoderConflict(ctx, task.ID, result)
	}
	mc.mu.Lock()
	onConflict := mc.onConflict
	mc.mu.Unlock()
	if onConflict != nil {
		onConflict(project.ID, task.ID, worktree.ID, result.SourceBranch, result.TargetBranch, result.SourceCommit, result.ConflictFiles)
	}
}

func (mc *MergeController) hasHumanAttachedSession(ctx context.Context, taskID string) bool {
	if mc == nil || mc.sessionRepo == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	sessions, err := mc.sessionRepo.ListByTask(ctx, taskID)
	if err != nil {
		return false
	}
	for _, sess := range sessions {
		if sess != nil && sess.HumanAttached {
			return true
		}
	}
	return false
}

func (mc *MergeController) projectLock(projectID string) *sync.Mutex {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	lock := mc.projectLocks[projectID]
	if lock == nil {
		lock = &sync.Mutex{}
		mc.projectLocks[projectID] = lock
	}
	return lock
}

func (mc *MergeController) shouldNotifyConflict(taskID string, sourceCommit string) bool {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(sourceCommit) == "" {
		return false
	}
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.conflictNotified[taskID] == sourceCommit {
		return false
	}
	mc.conflictNotified[taskID] = sourceCommit
	return true
}

func (mc *MergeController) notifyCoderConflict(ctx context.Context, taskID string, result mergeAttemptResult) {
	if mc == nil || mc.sessionRepo == nil || mc.sendCommand == nil {
		return
	}
	sessions, err := mc.sessionRepo.ListByTask(ctx, taskID)
	if err != nil {
		return
	}
	coder := pickCoderSession(sessions)
	if coder == nil {
		return
	}
	msg := fmt.Sprintf(
		"Merge conflict detected while merging `%s` into `%s` at commit `%s`.\nResolve conflicts in your worktree, commit the fix, and end with `[READY_FOR_REVIEW]`.\nConflicting files: %s\n",
		result.SourceBranch,
		result.TargetBranch,
		result.SourceCommit,
		strings.Join(result.ConflictFiles, ", "),
	)
	_ = mc.sendCommand(ctx, coder.ID, msg)
}

func pickCoderSession(sessions []*db.Session) *db.Session {
	var fallback *db.Session
	for _, sess := range sessions {
		if sess == nil || !strings.EqualFold(strings.TrimSpace(sess.Role), "coder") {
			continue
		}
		if fallback == nil {
			fallback = sess
		}
		status := strings.ToLower(strings.TrimSpace(sess.Status))
		if status == "completed" || status == "failed" {
			continue
		}
		return sess
	}
	return fallback
}

func mergeWorktreeBranch(ctx context.Context, repoPath string, sourceBranch string) (mergeAttemptResult, error) {
	repoPath = strings.TrimSpace(repoPath)
	sourceBranch = strings.TrimSpace(sourceBranch)
	if repoPath == "" || sourceBranch == "" {
		return mergeAttemptResult{}, fmt.Errorf("repo path and source branch are required")
	}
	targetBranch := resolveDefaultBranch(ctx, repoPath)
	sourceCommit, err := gitOutput(ctx, repoPath, "rev-parse", "--verify", sourceBranch)
	if err != nil {
		return mergeAttemptResult{}, err
	}
	sourceCommit = strings.TrimSpace(sourceCommit)
	result := mergeAttemptResult{
		SourceBranch: sourceBranch,
		SourceCommit: sourceCommit,
		TargetBranch: targetBranch,
	}

	alreadyMerged, err := isCommitMergedInto(ctx, repoPath, sourceCommit, targetBranch)
	if err == nil && alreadyMerged {
		result.Merged = true
		return result, nil
	}

	originalBranch, _ := gitOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	originalBranch = strings.TrimSpace(originalBranch)
	restoreBranch := originalBranch
	defer func() {
		if restoreBranch != "" && restoreBranch != "HEAD" {
			_, _ = gitOutput(context.Background(), repoPath, "checkout", restoreBranch)
		}
	}()

	if _, err := gitOutput(ctx, repoPath, "checkout", targetBranch); err != nil {
		return result, err
	}
	if _, err := gitOutput(ctx, repoPath, "merge", "--no-ff", "--no-edit", sourceBranch); err != nil {
		conflicts, listErr := conflictedFiles(ctx, repoPath)
		if listErr == nil && len(conflicts) > 0 {
			result.Conflict = true
			result.ConflictFiles = conflicts
			_, _ = gitOutput(context.Background(), repoPath, "merge", "--abort")
			return result, nil
		}
		_, _ = gitOutput(context.Background(), repoPath, "merge", "--abort")
		return result, nil
	}
	result.Merged = true
	return result, nil
}

func resolveDefaultBranch(ctx context.Context, repoPath string) string {
	if out, err := gitOutput(ctx, repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		branch := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "origin/"))
		if branch != "" {
			return branch
		}
	}
	if out, err := gitOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		branch := strings.TrimSpace(out)
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}
	return "main"
}

func isCommitMergedInto(ctx context.Context, repoPath string, commit string, targetBranch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "merge-base", "--is-ancestor", strings.TrimSpace(commit), strings.TrimSpace(targetBranch))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = err.Error()
	}
	return false, fmt.Errorf("merge-base --is-ancestor failed: %s", msg)
}

func conflictedFiles(ctx context.Context, repoPath string) ([]string, error) {
	out, err := gitOutput(ctx, repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(out), nil
}
