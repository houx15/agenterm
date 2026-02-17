package api

import (
	"net/http"

	"github.com/user/agenterm/internal/db"
)

type createTaskRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on"`
	Status      string   `json:"status"`
}

type updateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type taskDetailResponse struct {
	Task     *db.Task      `json:"task"`
	Sessions []*db.Session `json:"sessions"`
}

func (h *handler) createTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	project, err := h.projectRepo.Get(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	var req createTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	status := req.Status
	if status == "" {
		status = "pending"
	}

	task := &db.Task{
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		DependsOn:   req.DependsOn,
	}
	if err := h.taskRepo.Create(r.Context(), task); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, task)
}

func (h *handler) listTasks(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	project, err := h.projectRepo.Get(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	tasks, err := h.taskRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, tasks)
}

func (h *handler) getTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.taskRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if task == nil {
		jsonError(w, http.StatusNotFound, "task not found")
		return
	}

	sessions, err := h.sessionRepo.ListByTask(r.Context(), task.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, taskDetailResponse{Task: task, Sessions: sessions})
}

func (h *handler) updateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.taskRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if task == nil {
		jsonError(w, http.StatusNotFound, "task not found")
		return
	}

	var req updateTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if task.Title == "" {
		jsonError(w, http.StatusBadRequest, "title cannot be empty")
		return
	}

	if err := h.taskRepo.Update(r.Context(), task); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, task)
}
