package api

import (
	"net/http"
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
			}
			report["review_cycles_total"] = totalCycles
			report["open_review_issues_total"] = openIssues
		}
	}
	jsonResponse(w, http.StatusOK, report)
}
