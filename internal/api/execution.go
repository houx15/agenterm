package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/scaffold"
)

type launchExecutionResponse struct {
	Tasks     []*db.Task     `json:"tasks"`
	Worktrees []*db.Worktree  `json:"worktrees"`
	Run       *db.ProjectRun `json:"run"`
}

type transitionStageRequest struct {
	Transition string `json:"transition"`
}

type transitionStageResponse struct {
	Run    *db.ProjectRun  `json:"run"`
	Stages []*db.StageRun  `json:"stages"`
}

func (h *handler) launchExecution(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")
	requirement, ok := h.mustGetRequirement(w, r, requirementID)
	if !ok {
		return
	}
	if requirement.Status != "ready" {
		jsonError(w, http.StatusConflict, "requirement must be in 'ready' status to launch execution")
		return
	}

	project, ok := h.mustGetProject(w, r, requirement.ProjectID)
	if !ok {
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
	if ps.Blueprint == "" {
		jsonError(w, http.StatusConflict, "planning session has no blueprint")
		return
	}

	bp, err := scaffold.ParseBlueprint(ps.Blueprint)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid blueprint: %s", err.Error()))
		return
	}

	var tasks []*db.Task
	var worktrees []*db.Worktree

	for _, bpTask := range bp.Tasks {
		task := &db.Task{
			ProjectID:     project.ID,
			Title:         bpTask.Title,
			Description:   bpTask.Description,
			Status:        "pending",
			DependsOn:     bpTask.DependsOn,
			RequirementID: requirementID,
		}
		if err := h.taskRepo.Create(r.Context(), task); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		wt := &db.Worktree{
			ProjectID:  project.ID,
			BranchName: bpTask.WorktreeBranch,
			TaskID:     task.ID,
			Status:     "pending",
		}
		if err := h.worktreeRepo.Create(r.Context(), wt); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		task.WorktreeID = wt.ID
		if err := h.taskRepo.Update(r.Context(), task); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		tasks = append(tasks, task)
		worktrees = append(worktrees, wt)
	}

	run, err := h.runRepo.EnsureActive(r.Context(), project.ID, "build", "launch")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = h.runRepo.UpsertStageRun(r.Context(), run.ID, "build", "active", "")

	requirement.Status = "building"
	if err := h.requirementRepo.Update(r.Context(), requirement); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, launchExecutionResponse{
		Tasks:     tasks,
		Worktrees: worktrees,
		Run:       run,
	})
}

func (h *handler) transitionStage(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")
	requirement, ok := h.mustGetRequirement(w, r, requirementID)
	if !ok {
		return
	}

	var req transitionStageRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	transition := strings.ToLower(strings.TrimSpace(req.Transition))
	validTransitions := map[string]bool{
		"review": true,
		"merge":  true,
		"test":   true,
		"done":   true,
	}
	if !validTransitions[transition] {
		jsonError(w, http.StatusBadRequest, "transition must be one of: review, merge, test, done")
		return
	}

	run, err := h.runRepo.GetActiveByProject(r.Context(), requirement.ProjectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		jsonError(w, http.StatusNotFound, "no active run found")
		return
	}

	currentStage := strings.ToLower(strings.TrimSpace(run.CurrentStage))
	nextStage, err := resolveTransitionStage(currentStage, transition)
	if err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}

	// Complete the current stage.
	_ = h.runRepo.UpsertStageRun(r.Context(), run.ID, currentStage, "completed", "")

	if transition == "done" {
		run.Status = "completed"
		run.CurrentStage = currentStage
		if err := h.runRepo.UpdateStage(r.Context(), run.ID, currentStage, "completed"); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		requirement.Status = "done"
		if err := h.requirementRepo.Update(r.Context(), requirement); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		// Create a new stage run.
		_ = h.runRepo.UpsertStageRun(r.Context(), run.ID, nextStage, "active", "")
		if err := h.runRepo.UpdateStage(r.Context(), run.ID, nextStage, "active"); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	updated, err := h.runRepo.Get(r.Context(), run.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stages, err := h.runRepo.ListStageRuns(r.Context(), run.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, transitionStageResponse{
		Run:    updated,
		Stages: stages,
	})
}

// resolveTransitionStage maps a transition name to the target stage,
// validating it is legal from the current stage.
func resolveTransitionStage(currentStage string, transition string) (string, error) {
	// Transition map: what transitions are legal from which stages.
	//   build -> review (transition="review")
	//   build -> test   (transition="test")
	//   test  -> done   (transition="done")
	//   build -> done   (transition="done")
	//   any   -> done   (transition="done") — always legal to finish
	allowed := map[string]map[string]string{
		"build": {
			"review": "test",
			"test":   "test",
			"done":   "",
		},
		"test": {
			"merge": "test",
			"done":  "",
		},
		"plan": {
			"review": "build",
			"done":   "",
		},
	}

	stageTransitions, ok := allowed[currentStage]
	if !ok {
		if transition == "done" {
			return "", nil
		}
		return "", fmt.Errorf("invalid transition %q from stage %q", transition, currentStage)
	}

	nextStage, ok := stageTransitions[transition]
	if !ok {
		return "", fmt.Errorf("invalid transition %q from stage %q", transition, currentStage)
	}

	return nextStage, nil
}
