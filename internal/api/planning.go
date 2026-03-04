package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type createPlanningSessionRequest struct{}

type updatePlanningSessionRequest struct {
	Status         *string `json:"status"`
	AgentSessionID *string `json:"agent_session_id"`
}

type saveBlueprintRequest struct {
	Blueprint json.RawMessage `json:"blueprint"`
}

func (h *handler) createPlanningSession(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")
	requirement, ok := h.mustGetRequirement(w, r, requirementID)
	if !ok {
		return
	}
	if _, ok := h.mustGetProject(w, r, requirement.ProjectID); !ok {
		return
	}

	ps := &db.PlanningSession{
		RequirementID: requirementID,
		Status:        "active",
	}
	if err := h.planningSessionRepo.Create(r.Context(), ps); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	requirement.Status = "planning"
	if err := h.requirementRepo.Update(r.Context(), requirement); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, ps)
}

func (h *handler) getPlanningSession(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")
	if _, ok := h.mustGetRequirement(w, r, requirementID); !ok {
		return
	}

	ps, err := h.planningSessionRepo.GetByRequirement(r.Context(), requirementID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ps == nil {
		jsonError(w, http.StatusNotFound, "planning session not found")
		return
	}

	jsonResponse(w, http.StatusOK, ps)
}

func (h *handler) updatePlanningSession(w http.ResponseWriter, r *http.Request) {
	psID := r.PathValue("id")
	ps, ok := h.mustGetPlanningSession(w, r, psID)
	if !ok {
		return
	}

	var req updatePlanningSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*req.Status))
		if status != "active" && status != "completed" && status != "failed" {
			jsonError(w, http.StatusBadRequest, "status must be one of: active, completed, failed")
			return
		}
		ps.Status = status
	}
	if req.AgentSessionID != nil {
		ps.AgentSessionID = strings.TrimSpace(*req.AgentSessionID)
	}

	if err := h.planningSessionRepo.Update(r.Context(), ps); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ps)
}

func (h *handler) saveBlueprint(w http.ResponseWriter, r *http.Request) {
	psID := r.PathValue("id")
	ps, ok := h.mustGetPlanningSession(w, r, psID)
	if !ok {
		return
	}

	var req saveBlueprintRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(req.Blueprint) == 0 {
		jsonError(w, http.StatusBadRequest, "blueprint is required")
		return
	}

	// Validate the blueprint is valid JSON.
	if !json.Valid(req.Blueprint) {
		jsonError(w, http.StatusBadRequest, "blueprint must be valid JSON")
		return
	}

	ps.Blueprint = string(req.Blueprint)
	ps.Status = "completed"
	if err := h.planningSessionRepo.Update(r.Context(), ps); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update requirement status to "ready".
	requirement, err := h.requirementRepo.Get(r.Context(), ps.RequirementID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if requirement != nil {
		requirement.Status = "ready"
		if err := h.requirementRepo.Update(r.Context(), requirement); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	jsonResponse(w, http.StatusOK, ps)
}

func (h *handler) mustGetPlanningSession(w http.ResponseWriter, r *http.Request, id string) (*db.PlanningSession, bool) {
	ps, err := h.planningSessionRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if ps == nil {
		jsonError(w, http.StatusNotFound, "planning session not found")
		return nil, false
	}
	return ps, true
}
