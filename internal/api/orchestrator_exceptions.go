package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/user/agenterm/internal/orchestrator"
)

type orchestratorExceptionItem struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Source    string         `json:"source"`
	Category  string         `json:"category"`
	Severity  string         `json:"severity"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	Status    string         `json:"status"`
}

type resolveProjectOrchestratorExceptionRequest struct {
	Status string `json:"status"`
}

func (h *handler) listProjectOrchestratorExceptions(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if statusFilter == "" {
		statusFilter = "open"
	}
	if statusFilter != "open" && statusFilter != "resolved" && statusFilter != "all" {
		jsonError(w, http.StatusBadRequest, "status must be one of: open, resolved, all")
		return
	}

	items, err := h.collectProjectOrchestratorExceptions(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range items {
		if h.isExceptionResolved(projectID, items[i].ID) {
			items[i].Status = "resolved"
		} else {
			items[i].Status = "open"
		}
	}

	filtered := make([]orchestratorExceptionItem, 0, len(items))
	for _, item := range items {
		if statusFilter == "all" || item.Status == statusFilter {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })

	openCount := 0
	resolvedCount := 0
	for _, item := range items {
		if item.Status == "resolved" {
			resolvedCount++
			continue
		}
		openCount++
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"project_id": projectID,
		"items":      filtered,
		"counts": map[string]int{
			"total":    len(items),
			"open":     openCount,
			"resolved": resolvedCount,
		},
	})
}

func (h *handler) resolveProjectOrchestratorException(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	exceptionID := strings.TrimSpace(r.PathValue("exception_id"))
	if exceptionID == "" {
		jsonError(w, http.StatusBadRequest, "exception_id is required")
		return
	}

	status := "resolved"
	if strings.TrimSpace(r.Header.Get("Content-Length")) != "" && r.ContentLength > 0 {
		var req resolveProjectOrchestratorExceptionRequest
		if err := decodeJSON(r, &req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Status) != "" {
			status = strings.ToLower(strings.TrimSpace(req.Status))
		}
	}
	if status != "resolved" && status != "open" {
		jsonError(w, http.StatusBadRequest, "status must be one of: resolved, open")
		return
	}

	h.exceptionMu.Lock()
	if _, ok := h.resolvedException[projectID]; !ok {
		h.resolvedException[projectID] = make(map[string]bool)
	}
	if status == "resolved" {
		h.resolvedException[projectID][exceptionID] = true
	} else {
		delete(h.resolvedException[projectID], exceptionID)
	}
	h.exceptionMu.Unlock()

	if h.hub != nil {
		h.hub.BroadcastProjectEvent(projectID, "confirmation_resolved", map[string]any{
			"exception_id": exceptionID,
			"status":       status,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"project_id":   projectID,
		"exception_id": exceptionID,
		"status":       status,
	})
}

func (h *handler) isExceptionResolved(projectID string, exceptionID string) bool {
	h.exceptionMu.Lock()
	defer h.exceptionMu.Unlock()
	projectResolved := h.resolvedException[projectID]
	if len(projectResolved) == 0 {
		return false
	}
	return projectResolved[exceptionID]
}

func (h *handler) collectProjectOrchestratorExceptions(ctx context.Context, projectID string) ([]orchestratorExceptionItem, error) {
	items := make([]orchestratorExceptionItem, 0, 16)
	seen := make(map[string]struct{})
	add := func(item orchestratorExceptionItem) {
		if item.ID == "" {
			return
		}
		if _, ok := seen[item.ID]; ok {
			return
		}
		seen[item.ID] = struct{}{}
		items = append(items, item)
	}

	if h.runRepo != nil {
		run, err := h.runRepo.GetActiveByProject(ctx, projectID)
		if err == nil && run == nil {
			run, err = h.runRepo.GetLatestByProject(ctx, projectID)
		}
		if err == nil && run != nil {
			stages, err := h.runRepo.ListStageRuns(ctx, run.ID)
			if err == nil {
				for _, stage := range stages {
					if stage == nil {
						continue
					}
					status := strings.ToLower(strings.TrimSpace(stage.Status))
					if status != "blocked" && status != "failed" {
						continue
					}
					add(orchestratorExceptionItem{
						ID:        fmt.Sprintf("stage-%s-%s", run.ID, stage.Stage),
						ProjectID: projectID,
						Source:    "stage_run",
						Category:  "stage_state",
						Severity:  "high",
						Message:   truncateExceptionMessage(fmt.Sprintf("Stage %s is %s. %s", stage.Stage, status, stage.EvidenceJSON), 320),
						CreatedAt: stage.UpdatedAt,
						Metadata: map[string]any{
							"run_id": run.ID,
							"stage":  stage.Stage,
							"status": status,
						},
					})
				}
			}
		}
	}

	if h.orchestrator != nil {
		ledger := h.orchestrator.RecentCommandLedger(200)
		for _, entry := range ledger {
			status := strings.ToLower(strings.TrimSpace(entry.Status))
			if status != "failed" && status != "timeout" {
				continue
			}
			if !h.commandLedgerEntryBelongsToProject(ctx, entry, projectID) {
				continue
			}
			createdAt := entry.CompletedAt
			if createdAt.IsZero() {
				createdAt = entry.IssuedAt
			}
			if createdAt.IsZero() {
				createdAt = time.Now().UTC()
			}
			add(orchestratorExceptionItem{
				ID:        fmt.Sprintf("ledger-%d", entry.ID),
				ProjectID: projectID,
				Source:    "command_ledger",
				Category:  "session_command",
				Severity:  "high",
				Message:   truncateExceptionMessage(firstNonEmpty(entry.Error, entry.ResultSnippet, "command dispatch failure"), 320),
				CreatedAt: createdAt,
				Metadata: map[string]any{
					"tool_name":  entry.ToolName,
					"session_id": entry.SessionID,
					"status":     entry.Status,
				},
			})
		}
	}

	if h.historyRepo != nil {
		history, err := h.historyRepo.ListByProjectAndRoles(ctx, projectID, 120, []string{"assistant"})
		if err == nil {
			for _, message := range history {
				if message == nil {
					continue
				}
				lower := strings.ToLower(strings.TrimSpace(message.Content))
				if lower == "" {
					continue
				}
				if !strings.Contains(lower, "approval needed before execution") &&
					!strings.Contains(lower, "execution policy blocked") &&
					!strings.Contains(lower, "approval_required") &&
					!strings.Contains(lower, "stage_tool_not_allowed") {
					continue
				}
				createdAt, err := time.Parse(time.RFC3339Nano, message.CreatedAt)
				if err != nil {
					createdAt = time.Now().UTC()
				}
				add(orchestratorExceptionItem{
					ID:        "history-" + message.ID,
					ProjectID: projectID,
					Source:    "history",
					Category:  "policy_gate",
					Severity:  "medium",
					Message:   truncateExceptionMessage(message.Content, 320),
					CreatedAt: createdAt,
					Metadata: map[string]any{
						"message_id": message.ID,
					},
				})
			}
		}
	}

	return items, nil
}

func (h *handler) commandLedgerEntryBelongsToProject(ctx context.Context, entry orchestrator.CommandLedgerEntry, projectID string) bool {
	sessionID := strings.TrimSpace(entry.SessionID)
	if sessionID == "" || h.sessionRepo == nil || h.taskRepo == nil {
		return false
	}
	session, err := h.sessionRepo.Get(ctx, sessionID)
	if err != nil || session == nil {
		return false
	}
	taskID := strings.TrimSpace(session.TaskID)
	if taskID == "" {
		return false
	}
	task, err := h.taskRepo.Get(ctx, taskID)
	if err != nil || task == nil {
		return false
	}
	return task.ProjectID == projectID
}

func truncateExceptionMessage(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
