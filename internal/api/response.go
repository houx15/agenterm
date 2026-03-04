package api

import (
	"encoding/json"
	"net/http"

	"github.com/user/agenterm/internal/db"
)

type errorBody struct {
	Error string `json:"error"`
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if data == nil || status == http.StatusNoContent {
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, errorBody{Error: message})
}

func (h *handler) mustGetProject(w http.ResponseWriter, r *http.Request, id string) (*db.Project, bool) {
	project, err := h.projectRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return nil, false
	}
	return project, true
}

func (h *handler) mustGetTask(w http.ResponseWriter, r *http.Request, id string) (*db.Task, bool) {
	task, err := h.taskRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if task == nil {
		jsonError(w, http.StatusNotFound, "task not found")
		return nil, false
	}
	return task, true
}
