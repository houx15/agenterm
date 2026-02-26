package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type transitionProjectRunRequest struct {
	ToStage  string         `json:"to_stage"`
	Status   string         `json:"status,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

func (h *handler) getCurrentProjectRun(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.runRepo == nil || h.taskRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "run state unavailable")
		return
	}
	tasks, err := h.taskRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stage := deriveProjectStageFromTasks(tasks)
	run, err := h.runRepo.EnsureActive(r.Context(), projectID, stage, "auto")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if strings.TrimSpace(run.CurrentStage) == "" {
		run.CurrentStage = stage
	}
	_ = h.runRepo.UpsertStageRun(r.Context(), run.ID, run.CurrentStage, "active", "")
	stages, err := h.runRepo.ListStageRuns(r.Context(), run.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"run":    run,
		"stages": stages,
	})
}

func (h *handler) transitionProjectRun(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.runRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "run state unavailable")
		return
	}
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		jsonError(w, http.StatusBadRequest, "run_id is required")
		return
	}

	var req transitionProjectRunRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	stage := strings.ToLower(strings.TrimSpace(req.ToStage))
	if stage != "plan" && stage != "build" && stage != "test" {
		jsonError(w, http.StatusBadRequest, "to_stage must be one of: plan, build, test")
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "completed" && status != "failed" && status != "blocked" {
		jsonError(w, http.StatusBadRequest, "status must be one of: active, completed, failed, blocked")
		return
	}
	run, err := h.runRepo.Get(r.Context(), runID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil || run.ProjectID != projectID {
		jsonError(w, http.StatusNotFound, "run not found")
		return
	}

	evidenceJSON := ""
	if len(req.Evidence) > 0 {
		buf, err := json.Marshal(req.Evidence)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid evidence payload")
			return
		}
		evidenceJSON = string(buf)
	}

	if err := h.runRepo.UpdateStage(r.Context(), runID, stage, status); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.runRepo.UpsertStageRun(r.Context(), runID, stage, status, evidenceJSON); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.runRepo.Get(r.Context(), runID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stages, err := h.runRepo.ListStageRuns(r.Context(), runID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.hub != nil {
		h.hub.BroadcastProjectEvent(projectID, "stage_state", map[string]any{
			"run_id": runID,
			"stage":  stage,
			"status": status,
		})
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"run":    updated,
		"stages": stages,
	})
}

func deriveProjectStageFromTasks(tasks []*db.Task) string {
	if len(tasks) == 0 {
		return "plan"
	}
	allDone := true
	for _, task := range tasks {
		if task == nil {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if status != "done" && status != "completed" {
			allDone = false
			break
		}
	}
	if allDone {
		return "test"
	}
	return "build"
}
