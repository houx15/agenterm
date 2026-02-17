package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
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
	onEvent      func(projectID string, event string, data map[string]any)

	mu         sync.Mutex
	lastStatus map[string]string
}

func NewEventTrigger(o *Orchestrator, sessionRepo *db.SessionRepo, taskRepo *db.TaskRepo, projectRepo *db.ProjectRepo) *EventTrigger {
	return &EventTrigger{
		orchestrator: o,
		sessionRepo:  sessionRepo,
		taskRepo:     taskRepo,
		projectRepo:  projectRepo,
		lastStatus:   make(map[string]string),
	}
}

func (et *EventTrigger) SetOnEvent(fn func(projectID string, event string, data map[string]any)) {
	et.onEvent = fn
}

func (et *EventTrigger) OnSessionIdle(sessionID string) {
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
	et.emitEvent(projectID, "project_status_check_started", map[string]any{"source": "timer"})
	drain(ch)
	et.emitEvent(projectID, "project_status_check_completed", map[string]any{"source": "timer"})
}

func (et *EventTrigger) OnReviewReady(sessionID string, commitHash string) {
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
		go et.OnReviewReady(sess.ID, "latest")
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
