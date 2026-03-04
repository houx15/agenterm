package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type createRequirementRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
}

type updateRequirementRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *int    `json:"priority"`
	Status      *string `json:"status"`
}

type reorderRequirementsRequest struct {
	IDs []string `json:"ids"`
}

func (h *handler) createRequirement(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	var req createRequirementRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	status, err := normalizeRequirementStatus(req.Status)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	item := &db.Requirement{
		ProjectID:   projectID,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Priority:    req.Priority,
		Status:      status,
	}
	if err := h.requirementRepo.Create(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, item)
}

func (h *handler) listRequirements(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	items, err := h.requirementRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) getRequirement(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetRequirement(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (h *handler) updateRequirement(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetRequirement(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	var req updateRequirementRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Title != nil {
		item.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		item.Description = strings.TrimSpace(*req.Description)
	}
	if req.Priority != nil {
		item.Priority = *req.Priority
	}
	if req.Status != nil {
		status, err := normalizeRequirementStatus(*req.Status)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		item.Status = status
	}
	if strings.TrimSpace(item.Title) == "" {
		jsonError(w, http.StatusBadRequest, "title cannot be empty")
		return
	}
	if err := h.requirementRepo.Update(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (h *handler) deleteRequirement(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetRequirement(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.requirementRepo.Delete(r.Context(), item.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
}

func (h *handler) reorderRequirements(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	var req reorderRequirementsRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		jsonError(w, http.StatusBadRequest, "ids are required")
		return
	}
	if err := h.requirementRepo.Reorder(r.Context(), projectID, req.IDs); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := h.requirementRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func normalizeRequirementStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "draft", nil
	}
	switch status {
	case "draft", "planning", "ready", "building", "reviewing", "done":
		return status, nil
	default:
		return "", errors.New("invalid status")
	}
}

func (h *handler) mustGetRequirement(w http.ResponseWriter, r *http.Request, id string) (*db.Requirement, bool) {
	item, err := h.requirementRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if item == nil {
		jsonError(w, http.StatusNotFound, "requirement not found")
		return nil, false
	}
	return item, true
}
