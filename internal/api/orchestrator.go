package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/orchestrator"
)

type orchestratorChatRequest struct {
	ProjectID string `json:"project_id"`
	Message   string `json:"message"`
}

type orchestratorChatResponse struct {
	Response string                     `json:"response"`
	Events   []orchestrator.StreamEvent `json:"events,omitempty"`
}

func (h *handler) chatOrchestrator(w http.ResponseWriter, r *http.Request) {
	h.chatWithOrchestrator(w, r, h.orchestrator)
}

func (h *handler) chatDemandOrchestrator(w http.ResponseWriter, r *http.Request) {
	h.chatWithOrchestrator(w, r, h.demandOrchestrator)
}

func (h *handler) chatWithOrchestrator(w http.ResponseWriter, r *http.Request, instance *orchestrator.Orchestrator) {
	if instance == nil {
		jsonError(w, http.StatusServiceUnavailable, "orchestrator unavailable")
		return
	}
	var req orchestratorChatRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.Message) == "" {
		jsonError(w, http.StatusBadRequest, "project_id and message are required")
		return
	}
	stream, err := instance.Chat(r.Context(), req.ProjectID, req.Message)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := orchestratorChatResponse{Events: make([]orchestrator.StreamEvent, 0, 16)}
	for evt := range stream {
		resp.Events = append(resp.Events, evt)
		if evt.Type == "token" {
			if resp.Response == "" {
				resp.Response = evt.Text
			} else {
				resp.Response += "\n" + evt.Text
			}
		}
		if evt.Type == "error" {
			jsonError(w, http.StatusBadGateway, evt.Error)
			return
		}
	}
	jsonResponse(w, http.StatusOK, resp)
}

func (h *handler) listOrchestratorHistory(w http.ResponseWriter, r *http.Request) {
	h.listOrchestratorHistoryWithInstance(w, r, h.orchestrator)
}

func (h *handler) listDemandOrchestratorHistory(w http.ResponseWriter, r *http.Request) {
	h.listOrchestratorHistoryWithInstance(w, r, h.demandOrchestrator)
}

func (h *handler) listOrchestratorHistoryWithInstance(w http.ResponseWriter, r *http.Request, instance *orchestrator.Orchestrator) {
	if instance == nil {
		jsonError(w, http.StatusServiceUnavailable, "orchestrator history unavailable")
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if projectID == "" {
		jsonError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			jsonError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	items, err := instance.ListHistory(r.Context(), projectID, limit)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) getOrchestratorReport(w http.ResponseWriter, r *http.Request) {
	if h.orchestrator == nil {
		jsonError(w, http.StatusServiceUnavailable, "orchestrator unavailable")
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if projectID == "" {
		jsonError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	report, err := h.orchestrator.GenerateProgressReport(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.projectOrchestratorRepo != nil {
		profile, err := h.projectOrchestratorRepo.Get(r.Context(), projectID)
		if err == nil && profile != nil {
			report["orchestrator_profile"] = profile
			if h.workflowRepo != nil && profile.WorkflowID != "" {
				workflow, err := h.workflowRepo.Get(r.Context(), profile.WorkflowID)
				if err == nil && workflow != nil {
					report["workflow"] = workflow
				}
			}
		}
	}
	if h.knowledgeRepo != nil {
		entries, err := h.knowledgeRepo.ListByProject(r.Context(), projectID)
		if err == nil {
			report["knowledge_entries"] = len(entries)
		}
	}
	if h.reviewRepo != nil && h.taskRepo != nil {
		tasks, err := h.taskRepo.ListByProject(r.Context(), projectID)
		if err == nil {
			totalCycles := 0
			openIssues := 0
			maxLatestIteration := 0
			reviewTaskSummaries := make([]map[string]any, 0)
			hasPending := false
			hasRunning := false
			hasChangesRequested := false
			hasPassed := false
			for _, t := range tasks {
				cycles, err := h.reviewRepo.ListCyclesByTask(r.Context(), t.ID)
				if err != nil {
					continue
				}
				totalCycles += len(cycles)
				count, err := h.reviewRepo.CountOpenIssuesByTask(r.Context(), t.ID)
				if err == nil {
					openIssues += count
				}
				if len(cycles) == 0 {
					continue
				}
				latest := cycles[len(cycles)-1]
				status := strings.ToLower(strings.TrimSpace(latest.Status))
				if latest.Iteration > maxLatestIteration {
					maxLatestIteration = latest.Iteration
				}
				reviewTaskSummaries = append(reviewTaskSummaries, map[string]any{
					"task_id":          t.ID,
					"cycle_id":         latest.ID,
					"latest_iteration": latest.Iteration,
					"latest_status":    status,
					"open_issues":      count,
				})
				switch status {
				case "review_pending":
					hasPending = true
				case "review_running":
					hasRunning = true
				case "review_changes_requested":
					hasChangesRequested = true
				case "review_passed":
					hasPassed = true
				}
			}
			sort.Slice(reviewTaskSummaries, func(i, j int) bool {
				left, _ := reviewTaskSummaries[i]["task_id"].(string)
				right, _ := reviewTaskSummaries[j]["task_id"].(string)
				return left < right
			})
			reviewState := "not_started"
			if len(reviewTaskSummaries) > 0 {
				switch {
				case hasChangesRequested || openIssues > 0:
					reviewState = "changes_requested"
				case hasRunning || hasPending:
					reviewState = "in_review"
				case hasPassed:
					reviewState = "passed"
				default:
					reviewState = "unknown"
				}
			}
			report["review_cycles_total"] = totalCycles
			report["open_review_issues_total"] = openIssues
			report["review_state"] = reviewState
			report["review_latest_iteration"] = maxLatestIteration
			report["review_tasks_in_loop"] = len(reviewTaskSummaries)
			report["review_task_summaries"] = reviewTaskSummaries
			report["review_verdict"] = map[string]any{
				"status":               normalizeReviewVerdict(reviewState),
				"open_issues_total":    openIssues,
				"latest_iteration":     maxLatestIteration,
				"tasks_in_review_loop": len(reviewTaskSummaries),
			}
			completedTasks := countTasksWithStatus(tasks, "done", "completed")
			totalTasks := len(tasks)
			noActiveExecution := noActiveSessionExecution(readStatusCounts(report["session_counts"]))
			requiredChecks := map[string]any{
				"review_verdict_passed":   reviewState == "passed",
				"open_review_issues_zero": openIssues == 0,
				"tasks_completed":         totalTasks > 0 && completedTasks == totalTasks,
				"no_active_execution":     noActiveExecution,
			}
			report["required_checks"] = requiredChecks
			report["finalize_ready"] = boolFromMap(requiredChecks, "review_verdict_passed") &&
				boolFromMap(requiredChecks, "open_review_issues_zero") &&
				boolFromMap(requiredChecks, "tasks_completed") &&
				boolFromMap(requiredChecks, "no_active_execution")
		}
	}
	if h.orchestrator != nil {
		if ledger := h.orchestrator.RecentCommandLedger(25); len(ledger) > 0 {
			report["command_ledger_recent"] = ledger
		}
	}
	jsonResponse(w, http.StatusOK, report)
}

func (h *handler) getDemandOrchestratorReport(w http.ResponseWriter, r *http.Request) {
	if h.demandOrchestrator == nil {
		jsonError(w, http.StatusServiceUnavailable, "orchestrator unavailable")
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if projectID == "" {
		jsonError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.demandPoolRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "demand pool unavailable")
		return
	}
	items, err := h.demandPoolRepo.List(r.Context(), db.DemandPoolFilter{ProjectID: projectID, Limit: 200})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	statusCounts := map[string]int{}
	topCandidates := make([]map[string]any, 0, 5)
	for _, item := range items {
		if item == nil {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "" {
			status = "captured"
		}
		statusCounts[status]++
		if len(topCandidates) < 5 && (status == "captured" || status == "triaged" || status == "shortlisted") {
			topCandidates = append(topCandidates, map[string]any{
				"id":       item.ID,
				"title":    item.Title,
				"status":   item.Status,
				"priority": item.Priority,
				"impact":   item.Impact,
				"effort":   item.Effort,
				"risk":     item.Risk,
				"urgency":  item.Urgency,
			})
		}
	}
	report := map[string]any{
		"project_id":                projectID,
		"demand_items_total":        len(items),
		"demand_status_counts":      statusCounts,
		"top_triage_candidates":     topCandidates,
		"awaiting_triage_total":     statusCounts["captured"],
		"awaiting_scheduling_total": statusCounts["shortlisted"],
	}
	jsonResponse(w, http.StatusOK, report)
}

func normalizeReviewVerdict(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "passed":
		return "pass"
	case "changes_requested":
		return "changes_requested"
	case "in_review":
		return "in_review"
	default:
		return "not_started"
	}
}

func countTasksWithStatus(tasks []*db.Task, statuses ...string) int {
	if len(tasks) == 0 || len(statuses) == 0 {
		return 0
	}
	allowed := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[strings.ToLower(strings.TrimSpace(status))] = struct{}{}
	}
	count := 0
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(task.Status))]; ok {
			count++
		}
	}
	return count
}

func readStatusCounts(raw any) map[string]int {
	result := map[string]int{}
	items, ok := raw.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range items {
		switch v := value.(type) {
		case int:
			result[strings.ToLower(strings.TrimSpace(key))] = v
		case int32:
			result[strings.ToLower(strings.TrimSpace(key))] = int(v)
		case int64:
			result[strings.ToLower(strings.TrimSpace(key))] = int(v)
		case float64:
			result[strings.ToLower(strings.TrimSpace(key))] = int(v)
		}
	}
	return result
}

func noActiveSessionExecution(counts map[string]int) bool {
	busyStatuses := []string{"running", "working", "queued", "starting", "waiting", "needs_input", "human_takeover"}
	totalBusy := 0
	for _, status := range busyStatuses {
		totalBusy += counts[strings.ToLower(status)]
	}
	return totalBusy == 0
}

func boolFromMap(items map[string]any, key string) bool {
	value, ok := items[key]
	if !ok {
		return false
	}
	v, ok := value.(bool)
	return ok && v
}
