package api

import (
	"net/http"
	"sort"
	"strings"

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
	if h.orchestrator == nil {
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

	stream, err := h.orchestrator.Chat(r.Context(), req.ProjectID, req.Message)
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
		}
	}
	jsonResponse(w, http.StatusOK, report)
}
