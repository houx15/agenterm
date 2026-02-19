package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type createDemandPoolItemRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Impact      int      `json:"impact"`
	Effort      int      `json:"effort"`
	Risk        int      `json:"risk"`
	Urgency     int      `json:"urgency"`
	Tags        []string `json:"tags"`
	Source      string   `json:"source"`
	CreatedBy   string   `json:"created_by"`
	Notes       string   `json:"notes"`
}

type updateDemandPoolItemRequest struct {
	Title          *string   `json:"title"`
	Description    *string   `json:"description"`
	Status         *string   `json:"status"`
	Priority       *int      `json:"priority"`
	Impact         *int      `json:"impact"`
	Effort         *int      `json:"effort"`
	Risk           *int      `json:"risk"`
	Urgency        *int      `json:"urgency"`
	Tags           *[]string `json:"tags"`
	Source         *string   `json:"source"`
	CreatedBy      *string   `json:"created_by"`
	Notes          *string   `json:"notes"`
	SelectedTaskID *string   `json:"selected_task_id"`
}

type reprioritizeDemandPoolRequest struct {
	Items []reprioritizeDemandPoolItem `json:"items"`
}

type reprioritizeDemandPoolItem struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}

type promoteDemandPoolItemRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	DependsOn   []string `json:"depends_on"`
}

func (h *handler) listDemandPoolItems(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("offset")))
	items, err := h.demandPoolRepo.List(r.Context(), db.DemandPoolFilter{
		ProjectID: projectID,
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		Tag:       strings.TrimSpace(r.URL.Query().Get("tag")),
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) createDemandPoolItem(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	var req createDemandPoolItemRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	status, err := normalizeDemandPoolStatus(req.Status)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	item := &db.DemandPoolItem{
		ProjectID:   projectID,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Status:      status,
		Priority:    req.Priority,
		Impact:      req.Impact,
		Effort:      req.Effort,
		Risk:        req.Risk,
		Urgency:     req.Urgency,
		Tags:        req.Tags,
		Source:      strings.TrimSpace(req.Source),
		CreatedBy:   strings.TrimSpace(req.CreatedBy),
		Notes:       strings.TrimSpace(req.Notes),
	}
	if item.Source == "" {
		item.Source = "user"
	}
	if err := h.demandPoolRepo.Create(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, item)
}

func (h *handler) getDemandPoolItem(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetDemandPoolItem(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (h *handler) updateDemandPoolItem(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetDemandPoolItem(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	var req updateDemandPoolItemRequest
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
	if req.Status != nil {
		status, err := normalizeDemandPoolStatus(*req.Status)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		item.Status = status
	}
	if req.Priority != nil {
		item.Priority = *req.Priority
	}
	if req.Impact != nil {
		item.Impact = *req.Impact
	}
	if req.Effort != nil {
		item.Effort = *req.Effort
	}
	if req.Risk != nil {
		item.Risk = *req.Risk
	}
	if req.Urgency != nil {
		item.Urgency = *req.Urgency
	}
	if req.Tags != nil {
		item.Tags = *req.Tags
	}
	if req.Source != nil {
		item.Source = strings.TrimSpace(*req.Source)
	}
	if req.CreatedBy != nil {
		item.CreatedBy = strings.TrimSpace(*req.CreatedBy)
	}
	if req.Notes != nil {
		item.Notes = strings.TrimSpace(*req.Notes)
	}
	if req.SelectedTaskID != nil {
		item.SelectedTaskID = strings.TrimSpace(*req.SelectedTaskID)
	}
	if strings.TrimSpace(item.Title) == "" {
		jsonError(w, http.StatusBadRequest, "title cannot be empty")
		return
	}
	if err := h.demandPoolRepo.Update(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (h *handler) deleteDemandPoolItem(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetDemandPoolItem(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.demandPoolRepo.Delete(r.Context(), item.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
}

func (h *handler) reprioritizeDemandPool(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	var req reprioritizeDemandPoolRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	updated := make([]*db.DemandPoolItem, 0, len(req.Items))
	for _, entry := range req.Items {
		item, ok := h.mustGetDemandPoolItem(w, r, entry.ID)
		if !ok {
			return
		}
		if item.ProjectID != projectID {
			jsonError(w, http.StatusBadRequest, "item does not belong to project")
			return
		}
		item.Priority = entry.Priority
		if err := h.demandPoolRepo.Update(r.Context(), item); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		updated = append(updated, item)
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (h *handler) promoteDemandPoolItem(w http.ResponseWriter, r *http.Request) {
	item, ok := h.mustGetDemandPoolItem(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if _, ok := h.mustGetProject(w, r, item.ProjectID); !ok {
		return
	}

	var req promoteDemandPoolItemRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = item.Title
	}
	if title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = item.Description
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "pending"
	}
	task := &db.Task{
		ProjectID:   item.ProjectID,
		Title:       title,
		Description: description,
		Status:      status,
		DependsOn:   req.DependsOn,
	}
	if err := h.taskRepo.Create(r.Context(), task); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	item.SelectedTaskID = task.ID
	item.Status = "scheduled"
	if err := h.demandPoolRepo.Update(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"demand_item": item,
		"task":        task,
	})
}

func normalizeDemandPoolStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "captured", nil
	}
	switch status {
	case "captured", "triaged", "shortlisted", "scheduled", "done", "rejected":
		return status, nil
	default:
		return "", errors.New("invalid status")
	}
}

func (h *handler) mustGetDemandPoolItem(w http.ResponseWriter, r *http.Request, id string) (*db.DemandPoolItem, bool) {
	item, err := h.demandPoolRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if item == nil {
		jsonError(w, http.StatusNotFound, "demand pool item not found")
		return nil, false
	}
	return item, true
}
