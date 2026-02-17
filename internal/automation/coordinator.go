package automation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
)

const (
	defaultCoordinatorPollInterval = 2 * time.Second
	defaultMaxReviewIterations     = 3
)

type OutputEntry struct {
	Text      string
	Timestamp time.Time
}

type CoordinatorConfig struct {
	SessionRepo    *db.SessionRepo
	TaskRepo       *db.TaskRepo
	WorktreeRepo   *db.WorktreeRepo
	ProjectRepo    *db.ProjectRepo
	PollInterval   time.Duration
	MaxIterations  int
	SendCommand    func(ctx context.Context, sessionID string, text string) error
	GetOutputSince func(ctx context.Context, sessionID string, since time.Time) ([]OutputEntry, error)
}

type Coordinator struct {
	sessionRepo    *db.SessionRepo
	taskRepo       *db.TaskRepo
	worktreeRepo   *db.WorktreeRepo
	projectRepo    *db.ProjectRepo
	pollInterval   time.Duration
	maxIterations  int
	sendCommand    func(ctx context.Context, sessionID string, text string) error
	getOutputSince func(ctx context.Context, sessionID string, since time.Time) ([]OutputEntry, error)

	mu                sync.RWMutex
	pausedSession     map[string]bool
	activeMonitorPair map[string]bool
}

func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	interval := cfg.PollInterval
	if interval <= 0 {
		interval = defaultCoordinatorPollInterval
	}
	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxReviewIterations
	}
	return &Coordinator{
		sessionRepo:       cfg.SessionRepo,
		taskRepo:          cfg.TaskRepo,
		worktreeRepo:      cfg.WorktreeRepo,
		projectRepo:       cfg.ProjectRepo,
		pollInterval:      interval,
		maxIterations:     maxIterations,
		sendCommand:       cfg.SendCommand,
		getOutputSince:    cfg.GetOutputSince,
		pausedSession:     make(map[string]bool),
		activeMonitorPair: make(map[string]bool),
	}
}

func (c *Coordinator) SetSessionPaused(sessionID string, paused bool) {
	if c == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	c.mu.Lock()
	c.pausedSession[sessionID] = paused
	c.mu.Unlock()
}

func (c *Coordinator) Run(ctx context.Context) {
	if c == nil || c.sessionRepo == nil {
		return
	}
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	c.scanAndLaunch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.scanAndLaunch(ctx)
		}
	}
}

func (c *Coordinator) scanAndLaunch(ctx context.Context) {
	if c == nil || c.sessionRepo == nil {
		return
	}
	sessions, err := c.sessionRepo.List(ctx, db.SessionFilter{})
	if err != nil {
		return
	}
	byTask := make(map[string]map[string]*db.Session)
	for _, sess := range sessions {
		if sess == nil || strings.TrimSpace(sess.TaskID) == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(sess.Role))
		if role != "coder" && role != "reviewer" {
			continue
		}
		if byTask[sess.TaskID] == nil {
			byTask[sess.TaskID] = make(map[string]*db.Session)
		}
		if byTask[sess.TaskID][role] == nil {
			byTask[sess.TaskID][role] = sess
		}
	}

	for _, roles := range byTask {
		coder := roles["coder"]
		reviewer := roles["reviewer"]
		if coder == nil || reviewer == nil {
			continue
		}
		if c.isPaused(coder.ID) || c.isPaused(reviewer.ID) {
			continue
		}
		key := pairKey(coder.ID, reviewer.ID)
		if c.markPairActive(key) {
			continue
		}
		go func(coderID, reviewerID, pair string) {
			defer c.markPairDone(pair)
			c.MonitorCoderSession(ctx, coderID, reviewerID)
		}(coder.ID, reviewer.ID, key)
	}
}

func (c *Coordinator) markPairActive(pair string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.activeMonitorPair[pair] {
		return true
	}
	c.activeMonitorPair[pair] = true
	return false
}

func (c *Coordinator) markPairDone(pair string) {
	c.mu.Lock()
	delete(c.activeMonitorPair, pair)
	c.mu.Unlock()
}

func (c *Coordinator) isPaused(sessionID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pausedSession[sessionID]
}

func pairKey(coderID, reviewerID string) string {
	return coderID + "|" + reviewerID
}

func (c *Coordinator) MonitorCoderSession(ctx context.Context, coderSessionID string, reviewerSessionID string) {
	if c == nil || c.sendCommand == nil || c.getOutputSince == nil {
		return
	}
	if strings.TrimSpace(coderSessionID) == "" || strings.TrimSpace(reviewerSessionID) == "" {
		return
	}

	workDir, taskPath, taskID, err := c.resolveSessionContext(ctx, coderSessionID)
	if err != nil {
		return
	}

	lastSeenCommit := ""
	iterations := 0
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		if iterations >= c.maxIterations {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.isPaused(coderSessionID) || c.isPaused(reviewerSessionID) {
				continue
			}
			commitHash, err := latestReadyForReviewCommit(ctx, workDir)
			if err != nil || commitHash == "" || commitHash == lastSeenCommit {
				continue
			}
			lastSeenCommit = commitHash

			diff, _ := diffForCommit(ctx, workDir, commitHash)
			taskBody, _ := os.ReadFile(taskPath)
			reviewPrompt := buildReviewPrompt(string(taskBody), diff)
			if err := c.sendCommand(ctx, reviewerSessionID, reviewPrompt+"\n"); err != nil {
				continue
			}

			since := time.Now().UTC()
			decision := c.waitForReviewerDecision(ctx, reviewerSessionID, since)
			if decision.approved {
				c.markTaskCompleted(ctx, taskID)
				return
			}
			if strings.TrimSpace(decision.feedback) != "" {
				_ = c.sendCommand(ctx, coderSessionID, "Reviewer feedback:\n"+decision.feedback+"\n")
			}
			iterations++
		}
	}
}

func (c *Coordinator) waitForReviewerDecision(ctx context.Context, reviewerSessionID string, since time.Time) reviewDecision {
	deadline := time.Now().UTC().Add(2 * time.Minute)
	for time.Now().UTC().Before(deadline) {
		entries, err := c.getOutputSince(ctx, reviewerSessionID, since)
		if err == nil && len(entries) > 0 {
			text := joinOutput(entries)
			if dec := parseReviewDecision(text); dec.approved || dec.feedback != "" {
				return dec
			}
		}
		select {
		case <-ctx.Done():
			return reviewDecision{}
		case <-time.After(2 * time.Second):
		}
	}
	return reviewDecision{}
}

func (c *Coordinator) markTaskCompleted(ctx context.Context, taskID string) {
	if c.taskRepo == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	task, err := c.taskRepo.Get(ctx, taskID)
	if err != nil || task == nil {
		return
	}
	task.Status = "completed"
	_ = c.taskRepo.Update(ctx, task)
}

func (c *Coordinator) resolveSessionContext(ctx context.Context, sessionID string) (workDir string, taskPath string, taskID string, err error) {
	if c.sessionRepo == nil || c.taskRepo == nil {
		return "", "", "", fmt.Errorf("repositories unavailable")
	}
	sess, err := c.sessionRepo.Get(ctx, sessionID)
	if err != nil || sess == nil {
		return "", "", "", fmt.Errorf("session not found")
	}
	taskID = sess.TaskID
	task, err := c.taskRepo.Get(ctx, taskID)
	if err != nil || task == nil {
		return "", "", "", fmt.Errorf("task not found")
	}
	if strings.TrimSpace(task.WorktreeID) != "" && c.worktreeRepo != nil {
		if wt, wtErr := c.worktreeRepo.Get(ctx, task.WorktreeID); wtErr == nil && wt != nil && strings.TrimSpace(wt.Path) != "" {
			return wt.Path, filepath.Join(wt.Path, "TASK.md"), taskID, nil
		}
	}
	if c.projectRepo == nil {
		return "", "", "", fmt.Errorf("project repository unavailable")
	}
	project, err := c.projectRepo.Get(ctx, task.ProjectID)
	if err != nil || project == nil {
		return "", "", "", fmt.Errorf("project not found")
	}
	return project.RepoPath, filepath.Join(project.RepoPath, "TASK.md"), taskID, nil
}

func latestReadyForReviewCommit(ctx context.Context, workDir string) (string, error) {
	out, err := gitOutput(ctx, workDir, "log", "--grep=\\[READY_FOR_REVIEW\\]", "-1", "--pretty=%H")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func diffForCommit(ctx context.Context, workDir string, commitHash string) (string, error) {
	commitHash = strings.TrimSpace(commitHash)
	if commitHash == "" {
		return "", nil
	}
	out, err := gitOutput(ctx, workDir, "show", "--patch", "--stat", "--pretty=", commitHash)
	if err != nil {
		return "", err
	}
	return out, nil
}

func buildReviewPrompt(taskContent string, diff string) string {
	var b strings.Builder
	b.WriteString("Review this implementation. Reply with one of:\n")
	b.WriteString("- APPROVED or LGTM if acceptable\n")
	b.WriteString("- Detailed requested changes otherwise\n\n")
	b.WriteString("TASK.md:\n")
	b.WriteString(taskContent)
	b.WriteString("\n\nDiff:\n")
	b.WriteString(diff)
	return b.String()
}

type reviewDecision struct {
	approved bool
	feedback string
}

func parseReviewDecision(text string) reviewDecision {
	normalized := strings.ToUpper(text)
	if strings.Contains(normalized, "APPROVED") || strings.Contains(normalized, "LGTM") {
		return reviewDecision{approved: true}
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return reviewDecision{}
	}
	return reviewDecision{feedback: cleaned}
}

func joinOutput(entries []OutputEntry) string {
	var b bytes.Buffer
	for _, entry := range entries {
		line := strings.TrimSpace(entry.Text)
		if line == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}
