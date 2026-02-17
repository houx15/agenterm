package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
)

type EventTrigger struct {
	orchestrator *Orchestrator
	sessionRepo  *db.SessionRepo
	taskRepo     *db.TaskRepo
	projectRepo  *db.ProjectRepo
	worktreeRepo *db.WorktreeRepo
	onEvent      func(projectID string, event string, data map[string]any)

	mu         sync.Mutex
	lastStatus map[string]string
}

func NewEventTrigger(o *Orchestrator, sessionRepo *db.SessionRepo, taskRepo *db.TaskRepo, projectRepo *db.ProjectRepo, worktreeRepo *db.WorktreeRepo) *EventTrigger {
	return &EventTrigger{
		orchestrator: o,
		sessionRepo:  sessionRepo,
		taskRepo:     taskRepo,
		projectRepo:  projectRepo,
		worktreeRepo: worktreeRepo,
		lastStatus:   make(map[string]string),
	}
}

func (et *EventTrigger) SetOnEvent(fn func(projectID string, event string, data map[string]any)) {
	et.onEvent = fn
}

func (et *EventTrigger) OnSessionIdle(sessionID string) {
	et.emitProjectPhaseEventForSession(sessionID, "dispatch")
	et.emitForSession(sessionID, fmt.Sprintf("Session %s is idle. Evaluate whether to dispatch next tasks or request review.", sessionID))
}

func (et *EventTrigger) OnTimer(projectID string) {
	if et == nil || et.orchestrator == nil || strings.TrimSpace(projectID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ch, err := et.orchestrator.Chat(ctx, projectID, "Periodic project check: summarize progress, detect blockers, and suggest next actions.")
	if err != nil {
		slog.Debug("orchestrator timer trigger failed", "project_id", projectID, "error", err)
		return
	}
	et.emitEvent(projectID, "project_phase_changed", map[string]any{"phase": "status_check", "source": "timer"})
	drain(ch)
	report, reportErr := et.orchestrator.GenerateProgressReport(ctx, projectID)
	if reportErr == nil {
		if blockers, ok := report["blockers"].([]any); ok && len(blockers) > 0 {
			if et.shouldNotifyOnBlocked(ctx, projectID) {
				et.emitEvent(projectID, "project_blocked", map[string]any{"source": "timer", "blockers": blockers})
			}
		}
	}
}

func (et *EventTrigger) OnReviewReady(sessionID string, commitHash string) {
	et.emitProjectPhaseEventForSession(sessionID, "review")
	message := fmt.Sprintf("Session %s produced commit %s with [READY_FOR_REVIEW]. Summarize changes and prepare review workflow.", sessionID, commitHash)
	et.emitForSession(sessionID, message)
}

func (et *EventTrigger) Start(ctx context.Context, pollInterval time.Duration) {
	if et == nil || et.sessionRepo == nil || et.orchestrator == nil {
		return
	}
	if pollInterval <= 0 {
		pollInterval = 15 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			et.poll(ctx)
		}
	}
}

func (et *EventTrigger) poll(ctx context.Context) {
	sessions, err := et.sessionRepo.List(ctx, db.SessionFilter{})
	if err != nil {
		slog.Debug("event trigger poll failed", "error", err)
		return
	}
	for _, sess := range sessions {
		if sess == nil {
			continue
		}
		et.handleTransition(sess)
	}
}

func (et *EventTrigger) handleTransition(sess *db.Session) {
	if sess == nil {
		return
	}
	status := strings.TrimSpace(sess.Status)
	if status == "" {
		return
	}

	et.mu.Lock()
	prev := et.lastStatus[sess.ID]
	et.lastStatus[sess.ID] = status
	et.mu.Unlock()
	if prev == status {
		return
	}

	switch status {
	case "idle":
		go et.OnSessionIdle(sess.ID)
	case "waiting_review":
		go et.OnReviewReady(sess.ID, et.resolveReviewCommitHash(sess))
	}
}

func (et *EventTrigger) emitForSession(sessionID string, syntheticMessage string) {
	if et == nil || et.orchestrator == nil || et.sessionRepo == nil || et.taskRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sess, err := et.sessionRepo.Get(ctx, sessionID)
	if err != nil || sess == nil {
		return
	}
	task, err := et.taskRepo.Get(ctx, sess.TaskID)
	if err != nil || task == nil {
		return
	}
	ch, err := et.orchestrator.Chat(ctx, task.ProjectID, syntheticMessage)
	if err != nil {
		slog.Debug("event trigger chat failed", "session_id", sessionID, "error", err)
		return
	}
	et.emitEvent(task.ProjectID, "orchestrator_trigger_started", map[string]any{"session_id": sessionID, "message": syntheticMessage})
	drain(ch)
	et.emitEvent(task.ProjectID, "orchestrator_trigger_completed", map[string]any{"session_id": sessionID})
}

func drain(ch <-chan StreamEvent) {
	for range ch {
	}
}

func (et *EventTrigger) emitEvent(projectID string, event string, data map[string]any) {
	if et == nil || et.onEvent == nil || strings.TrimSpace(projectID) == "" || strings.TrimSpace(event) == "" {
		return
	}
	et.onEvent(projectID, event, data)
}

func (et *EventTrigger) emitProjectPhaseEventForSession(sessionID string, phase string) {
	if et == nil || et.sessionRepo == nil || et.taskRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sess, err := et.sessionRepo.Get(ctx, sessionID)
	if err != nil || sess == nil {
		return
	}
	task, err := et.taskRepo.Get(ctx, sess.TaskID)
	if err != nil || task == nil {
		return
	}
	et.emitEvent(task.ProjectID, "project_phase_changed", map[string]any{"phase": phase, "session_id": sessionID, "task_id": task.ID})
}

func (et *EventTrigger) resolveReviewCommitHash(sess *db.Session) string {
	if sess == nil || et.taskRepo == nil {
		return "unknown"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	task, err := et.taskRepo.Get(ctx, sess.TaskID)
	if err != nil || task == nil {
		return "unknown"
	}
	repoPath := ""
	if strings.TrimSpace(task.WorktreeID) != "" && et.worktreeRepo != nil {
		wt, err := et.worktreeRepo.Get(ctx, task.WorktreeID)
		if err == nil && wt != nil {
			repoPath = strings.TrimSpace(wt.Path)
		}
	}
	if repoPath == "" && et.projectRepo != nil {
		project, err := et.projectRepo.Get(ctx, task.ProjectID)
		if err == nil && project != nil {
			repoPath = strings.TrimSpace(project.RepoPath)
		}
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return "unknown"
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--short=12", "HEAD")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "unknown"
	}
	return hash
}

func (et *EventTrigger) shouldNotifyOnBlocked(ctx context.Context, projectID string) bool {
	if et == nil || et.orchestrator == nil || et.orchestrator.projectOrchestratorRepo == nil || strings.TrimSpace(projectID) == "" {
		return true
	}
	profile, err := et.orchestrator.projectOrchestratorRepo.Get(ctx, projectID)
	if err != nil || profile == nil {
		return true
	}
	return profile.NotifyOnBlocked
}
