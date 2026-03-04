package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type createProjectKnowledgeRequest struct {
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	SourceURI string `json:"source_uri"`
}

type createReviewCycleRequest struct {
	CommitHash string `json:"commit_hash"`
}

type updateReviewCycleRequest struct {
	Status string `json:"status"`
}

type createReviewIssueRequest struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
}

type updateReviewIssueRequest struct {
	Severity   *string `json:"severity"`
	Summary    *string `json:"summary"`
	Status     *string `json:"status"`
	Resolution *string `json:"resolution"`
}

func (h *handler) listProjectKnowledge(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.knowledgeRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "knowledge repo unavailable")
		return
	}
	items, err := h.knowledgeRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) createProjectKnowledge(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.knowledgeRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "knowledge repo unavailable")
		return
	}
	var req createProjectKnowledgeRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		jsonError(w, http.StatusBadRequest, "title and content are required")
		return
	}
	entry := &db.ProjectKnowledgeEntry{
		ProjectID: projectID,
		Kind:      strings.TrimSpace(req.Kind),
		Title:     strings.TrimSpace(req.Title),
		Content:   req.Content,
		SourceURI: strings.TrimSpace(req.SourceURI),
	}
	if entry.Kind == "" {
		entry.Kind = "note"
	}
	if err := h.knowledgeRepo.Create(r.Context(), entry); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, entry)
}

func (h *handler) listTaskReviewCycles(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if _, ok := h.mustGetTask(w, r, taskID); !ok {
		return
	}
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	items, err := h.reviewRepo.ListCyclesByTask(r.Context(), taskID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) getTaskReviewLoopStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if _, ok := h.mustGetTask(w, r, taskID); !ok {
		return
	}
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	cycles, err := h.reviewRepo.ListCyclesByTask(r.Context(), taskID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	openIssues, err := h.reviewRepo.CountOpenIssuesByTask(r.Context(), taskID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	latestStatus := "not_started"
	latestIteration := 0
	latestCycleID := ""
	if len(cycles) > 0 {
		latest := cycles[len(cycles)-1]
		latestStatus = strings.ToLower(strings.TrimSpace(latest.Status))
		latestIteration = latest.Iteration
		latestCycleID = latest.ID
	}
	passed := latestStatus == "review_passed" && openIssues == 0
	needsFix := latestStatus == "review_changes_requested" || openIssues > 0
	jsonResponse(w, http.StatusOK, map[string]any{
		"task_id":           taskID,
		"latest_cycle_id":   latestCycleID,
		"latest_iteration":  latestIteration,
		"latest_status":     latestStatus,
		"open_issues_total": openIssues,
		"passed":            passed,
		"needs_fix":         needsFix,
	})
}

func (h *handler) createTaskReviewCycle(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if _, ok := h.mustGetTask(w, r, taskID); !ok {
		return
	}
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	var req createReviewCycleRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cycle := &db.ReviewCycle{
		TaskID:     taskID,
		CommitHash: strings.TrimSpace(req.CommitHash),
		Status:     "review_pending",
	}
	if err := h.reviewRepo.CreateCycle(r.Context(), cycle); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, cycle)
}

func (h *handler) updateReviewCycle(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	cycleID := r.PathValue("id")
	cycle, err := h.reviewRepo.GetCycle(r.Context(), cycleID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cycle == nil {
		jsonError(w, http.StatusNotFound, "review cycle not found")
		return
	}
	var req updateReviewCycleRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		jsonError(w, http.StatusBadRequest, "status is required")
		return
	}
	if err := h.reviewRepo.UpdateCycleStatus(r.Context(), cycleID, req.Status); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.reviewRepo.GetCycle(r.Context(), cycleID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.emitReviewCycleProjectEvents(r, updated)
	jsonResponse(w, http.StatusOK, updated)
}

func (h *handler) listReviewCycleIssues(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	cycleID := r.PathValue("id")
	cycle, err := h.reviewRepo.GetCycle(r.Context(), cycleID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cycle == nil {
		jsonError(w, http.StatusNotFound, "review cycle not found")
		return
	}
	items, err := h.reviewRepo.ListIssuesByCycle(r.Context(), cycleID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) createReviewCycleIssue(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	cycleID := r.PathValue("id")
	cycle, err := h.reviewRepo.GetCycle(r.Context(), cycleID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cycle == nil {
		jsonError(w, http.StatusNotFound, "review cycle not found")
		return
	}
	var req createReviewIssueRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Summary) == "" {
		jsonError(w, http.StatusBadRequest, "summary is required")
		return
	}
	issue := &db.ReviewIssue{
		CycleID:  cycleID,
		Severity: strings.TrimSpace(req.Severity),
		Summary:  strings.TrimSpace(req.Summary),
		Status:   strings.TrimSpace(req.Status),
	}
	if err := h.reviewRepo.CreateIssue(r.Context(), issue); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.syncLatestCycleStatusAndEmit(r, cycle.TaskID)
	jsonResponse(w, http.StatusCreated, issue)
}

func (h *handler) updateReviewIssue(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "review repo unavailable")
		return
	}
	issueID := r.PathValue("id")
	issue, err := h.reviewRepo.GetIssue(r.Context(), issueID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if issue == nil {
		jsonError(w, http.StatusNotFound, "review issue not found")
		return
	}
	var req updateReviewIssueRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Severity != nil {
		issue.Severity = strings.TrimSpace(*req.Severity)
	}
	if req.Summary != nil {
		issue.Summary = strings.TrimSpace(*req.Summary)
	}
	if req.Status != nil {
		issue.Status = strings.TrimSpace(*req.Status)
	}
	if req.Resolution != nil {
		issue.Resolution = strings.TrimSpace(*req.Resolution)
	}
	if strings.TrimSpace(issue.Summary) == "" {
		jsonError(w, http.StatusBadRequest, "summary is required")
		return
	}
	if err := h.reviewRepo.UpdateIssue(r.Context(), issue); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if cycle, err := h.reviewRepo.GetCycle(r.Context(), issue.CycleID); err == nil && cycle != nil {
		h.syncLatestCycleStatusAndEmit(r, cycle.TaskID)
	}
	jsonResponse(w, http.StatusOK, issue)
}

func (h *handler) emitReviewCycleProjectEvents(r *http.Request, cycle *db.ReviewCycle) {
	if h == nil || h.hub == nil || h.taskRepo == nil || cycle == nil {
		return
	}
	task, err := h.taskRepo.Get(r.Context(), cycle.TaskID)
	if err != nil || task == nil {
		return
	}
	projectID := task.ProjectID
	status := strings.ToLower(strings.TrimSpace(cycle.Status))
	switch status {
	case "review_running":
		h.hub.BroadcastProjectEvent(projectID, "project_phase_changed", map[string]any{"phase": "review", "status": status})
	case "review_changes_requested":
		h.hub.BroadcastProjectEvent(projectID, "review_iteration_completed", map[string]any{"task_id": cycle.TaskID, "cycle_id": cycle.ID, "iteration": cycle.Iteration, "status": status})
	case "review_passed":
		h.hub.BroadcastProjectEvent(projectID, "review_iteration_completed", map[string]any{"task_id": cycle.TaskID, "cycle_id": cycle.ID, "iteration": cycle.Iteration, "status": status})
		h.hub.BroadcastProjectEvent(projectID, "review_loop_passed", map[string]any{"task_id": cycle.TaskID, "cycle_id": cycle.ID, "iteration": cycle.Iteration})
		h.hub.BroadcastProjectEvent(projectID, "project_phase_changed", map[string]any{"phase": "review", "status": status})
	}
}

func (h *handler) syncLatestCycleStatusAndEmit(r *http.Request, taskID string) {
	if h == nil || h.reviewRepo == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	changed, cycle, err := h.reviewRepo.SyncLatestCycleStatusByTaskOpenIssues(r.Context(), taskID)
	if err != nil || !changed || cycle == nil {
		return
	}
	h.emitReviewCycleProjectEvents(r, cycle)
}

func (h *handler) shouldNotifyOnBlocked(_ context.Context, _ string) bool {
	return true
}
