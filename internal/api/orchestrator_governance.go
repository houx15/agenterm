package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/playbook"
)

type updateProjectOrchestratorRequest struct {
	WorkflowID      *string `json:"workflow_id"`
	DefaultProvider *string `json:"default_provider"`
	DefaultModel    *string `json:"default_model"`
	MaxParallel     *int    `json:"max_parallel"`
	ReviewPolicy    *string `json:"review_policy"`
	NotifyOnBlocked *bool   `json:"notify_on_blocked"`
}

type workflowPhaseRequest struct {
	Ordinal       int    `json:"ordinal"`
	PhaseType     string `json:"phase_type"`
	Role          string `json:"role"`
	EntryRule     string `json:"entry_rule"`
	ExitRule      string `json:"exit_rule"`
	MaxParallel   int    `json:"max_parallel"`
	AgentSelector string `json:"agent_selector"`
}

type workflowRequest struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Scope       string                 `json:"scope"`
	Version     int                    `json:"version"`
	Phases      []workflowPhaseRequest `json:"phases"`
}

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

type projectOrchestratorResponse struct {
	Orchestrator *db.ProjectOrchestrator `json:"orchestrator"`
	RoleBindings []*db.RoleBinding       `json:"role_bindings"`
	Workflow     *db.Workflow            `json:"workflow,omitempty"`
}

type replaceRoleBindingsRequest struct {
	Bindings []replaceRoleBindingItem `json:"bindings"`
}

type replaceRoleBindingItem struct {
	Role        string `json:"role"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	MaxParallel int    `json:"max_parallel"`
}

type previewProjectAssignmentsRequest struct {
	Stage string `json:"stage,omitempty"`
}

type confirmProjectAssignmentsRequest struct {
	Assignments []confirmProjectAssignmentItem `json:"assignments"`
}

type confirmProjectAssignmentItem struct {
	Stage       string `json:"stage,omitempty"`
	Role        string `json:"role"`
	AgentType   string `json:"agent_type"`
	MaxParallel int    `json:"max_parallel"`
}

type assignmentPreviewItem struct {
	Stage            string   `json:"stage"`
	Role             string   `json:"role"`
	Responsibilities string   `json:"responsibilities,omitempty"`
	AllowedAgents    []string `json:"allowed_agents"`
	Candidates       []string `json:"candidates"`
	SelectedAgent    string   `json:"selected_agent,omitempty"`
	SelectedModel    string   `json:"selected_model,omitempty"`
	MaxParallel      int      `json:"max_parallel"`
}

type stageRoleDef struct {
	Stage string
	Role  playbook.StageRole
}

func (h *handler) getProjectOrchestrator(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.projectOrchestratorRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "orchestrator profile unavailable")
		return
	}
	if err := h.projectOrchestratorRepo.EnsureDefaultForProject(r.Context(), projectID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	item, err := h.projectOrchestratorRepo.Get(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		jsonError(w, http.StatusNotFound, "project orchestrator not found")
		return
	}
	bindings, err := h.roleBindingRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var workflow *db.Workflow
	if h.workflowRepo != nil && item.WorkflowID != "" {
		workflow, err = h.workflowRepo.Get(r.Context(), item.WorkflowID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	jsonResponse(w, http.StatusOK, projectOrchestratorResponse{Orchestrator: item, RoleBindings: bindings, Workflow: workflow})
}

func (h *handler) updateProjectOrchestrator(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.projectOrchestratorRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "orchestrator profile unavailable")
		return
	}
	if err := h.projectOrchestratorRepo.EnsureDefaultForProject(r.Context(), projectID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	item, err := h.projectOrchestratorRepo.Get(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		jsonError(w, http.StatusNotFound, "project orchestrator not found")
		return
	}
	var req updateProjectOrchestratorRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.WorkflowID != nil {
		item.WorkflowID = strings.TrimSpace(*req.WorkflowID)
	}
	if req.DefaultProvider != nil {
		item.DefaultProvider = strings.TrimSpace(*req.DefaultProvider)
	}
	if req.DefaultModel != nil {
		item.DefaultModel = strings.TrimSpace(*req.DefaultModel)
	}
	if req.MaxParallel != nil {
		item.MaxParallel = *req.MaxParallel
	}
	if req.ReviewPolicy != nil {
		item.ReviewPolicy = strings.TrimSpace(*req.ReviewPolicy)
	}
	if req.NotifyOnBlocked != nil {
		item.NotifyOnBlocked = *req.NotifyOnBlocked
	}
	if item.WorkflowID == "" {
		jsonError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	if item.MaxParallel <= 0 || item.MaxParallel > 64 {
		jsonError(w, http.StatusBadRequest, "max_parallel must be between 1 and 64")
		return
	}
	if h.workflowRepo != nil {
		workflow, err := h.workflowRepo.Get(r.Context(), item.WorkflowID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if workflow == nil {
			jsonError(w, http.StatusBadRequest, "workflow not found")
			return
		}
	}
	if err := h.projectOrchestratorRepo.Update(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (h *handler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	if h.workflowRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
		return
	}
	items, err := h.workflowRepo.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) createWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.workflowRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
		return
	}
	var req workflowRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	item := &db.Workflow{
		ID:          strings.TrimSpace(req.ID),
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Scope:       strings.TrimSpace(req.Scope),
		Version:     req.Version,
		IsBuiltin:   false,
		Phases:      make([]*db.WorkflowPhase, 0, len(req.Phases)),
	}
	if item.Scope == "" {
		item.Scope = "project"
	}
	for _, phase := range req.Phases {
		item.Phases = append(item.Phases, &db.WorkflowPhase{
			Ordinal:       phase.Ordinal,
			PhaseType:     strings.TrimSpace(phase.PhaseType),
			Role:          strings.TrimSpace(phase.Role),
			EntryRule:     phase.EntryRule,
			ExitRule:      phase.ExitRule,
			MaxParallel:   phase.MaxParallel,
			AgentSelector: phase.AgentSelector,
		})
	}
	if err := validateWorkflowPhases(item.Phases); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.workflowRepo.Create(r.Context(), item); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, item)
}

func (h *handler) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.workflowRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
		return
	}
	id := r.PathValue("id")
	existing, err := h.workflowRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		jsonError(w, http.StatusNotFound, "workflow not found")
		return
	}
	if existing.IsBuiltin {
		jsonError(w, http.StatusBadRequest, "builtin workflow is immutable")
		return
	}
	var req workflowRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	existing.Description = req.Description
	if strings.TrimSpace(req.Scope) != "" {
		existing.Scope = strings.TrimSpace(req.Scope)
	}
	if req.Version > 0 {
		existing.Version = req.Version
	}
	existing.Phases = make([]*db.WorkflowPhase, 0, len(req.Phases))
	for _, phase := range req.Phases {
		existing.Phases = append(existing.Phases, &db.WorkflowPhase{
			Ordinal:       phase.Ordinal,
			PhaseType:     strings.TrimSpace(phase.PhaseType),
			Role:          strings.TrimSpace(phase.Role),
			EntryRule:     phase.EntryRule,
			ExitRule:      phase.ExitRule,
			MaxParallel:   phase.MaxParallel,
			AgentSelector: phase.AgentSelector,
		})
	}
	if err := validateWorkflowPhases(existing.Phases); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.workflowRepo.Update(r.Context(), existing); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, existing)
}

func (h *handler) deleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.workflowRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
		return
	}
	id := r.PathValue("id")
	if err := h.workflowRepo.Delete(r.Context(), id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
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

func validateWorkflowPhases(phases []*db.WorkflowPhase) error {
	if len(phases) == 0 {
		return nil
	}
	typeByOrdinal := make(map[int]struct{}, len(phases))
	allowedRoles := map[string]struct{}{
		"planner":    {},
		"coder":      {},
		"reviewer":   {},
		"researcher": {},
		"qa":         {},
	}
	allowedTypes := map[string]struct{}{
		"scan":           {},
		"planning":       {},
		"implementation": {},
		"review":         {},
		"testing":        {},
	}
	ordinals := make([]int, 0, len(phases))
	for _, phase := range phases {
		if phase == nil {
			return errBadRequest("workflow phase cannot be null")
		}
		if phase.Ordinal <= 0 {
			return errBadRequest("workflow phase ordinal must be >= 1")
		}
		if _, exists := typeByOrdinal[phase.Ordinal]; exists {
			return errBadRequest("workflow phase ordinals must be unique")
		}
		typeByOrdinal[phase.Ordinal] = struct{}{}
		ordinals = append(ordinals, phase.Ordinal)

		phaseType := strings.ToLower(strings.TrimSpace(phase.PhaseType))
		if phaseType == "" {
			return errBadRequest("workflow phase_type is required")
		}
		if _, ok := allowedTypes[phaseType]; !ok {
			return errBadRequest("workflow phase_type is invalid")
		}

		role := strings.ToLower(strings.TrimSpace(phase.Role))
		if role == "" {
			return errBadRequest("workflow role is required")
		}
		if _, ok := allowedRoles[role]; !ok {
			return errBadRequest("workflow role is invalid")
		}

		if phase.MaxParallel <= 0 || phase.MaxParallel > 64 {
			return errBadRequest("workflow phase max_parallel must be between 1 and 64")
		}
	}
	sort.Ints(ordinals)
	for i, ord := range ordinals {
		if ord != i+1 {
			return errBadRequest("workflow phase ordinals must be contiguous starting at 1")
		}
	}
	return nil
}

func errBadRequest(msg string) error {
	return &badRequestError{msg: msg}
}

type badRequestError struct {
	msg string
}

func (e *badRequestError) Error() string {
	return e.msg
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
		if h.shouldNotifyOnBlocked(r.Context(), projectID) {
			h.hub.BroadcastProjectEvent(projectID, "project_blocked", map[string]any{"reason": "review_changes_requested", "task_id": cycle.TaskID, "cycle_id": cycle.ID})
		}
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

func (h *handler) shouldNotifyOnBlocked(ctx context.Context, projectID string) bool {
	if h == nil || h.projectOrchestratorRepo == nil || strings.TrimSpace(projectID) == "" {
		return true
	}
	profile, err := h.projectOrchestratorRepo.Get(ctx, projectID)
	if err != nil || profile == nil {
		return true
	}
	return profile.NotifyOnBlocked
}

func (h *handler) listProjectRoleBindings(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.roleBindingRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "role binding repo unavailable")
		return
	}
	items, err := h.roleBindingRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) replaceProjectRoleBindings(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.roleBindingRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "role binding repo unavailable")
		return
	}
	var req replaceRoleBindingsRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	bindings := make([]*db.RoleBinding, 0, len(req.Bindings))
	for _, item := range req.Bindings {
		role := strings.TrimSpace(item.Role)
		provider := strings.TrimSpace(item.Provider)
		model := strings.TrimSpace(item.Model)
		if role == "" || provider == "" || model == "" {
			jsonError(w, http.StatusBadRequest, "role, provider, model are required")
			return
		}
		maxParallel := item.MaxParallel
		if maxParallel <= 0 || maxParallel > 64 {
			jsonError(w, http.StatusBadRequest, "max_parallel must be between 1 and 64")
			return
		}
		bindings = append(bindings, &db.RoleBinding{
			ProjectID:   projectID,
			Role:        role,
			Provider:    provider,
			Model:       model,
			MaxParallel: maxParallel,
		})
	}
	if err := h.roleBindingRepo.ReplaceForProject(r.Context(), projectID, bindings); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.roleBindingRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) listProjectAssignments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := h.mustGetProject(w, r, projectID); !ok {
		return
	}
	if h.roleAgentAssignRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "role agent assignment repo unavailable")
		return
	}
	items, err := h.roleAgentAssignRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (h *handler) previewProjectAssignments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	project, ok := h.mustGetProject(w, r, projectID)
	if !ok {
		return
	}
	if h.roleAgentAssignRepo == nil || h.registry == nil || h.playbookRegistry == nil {
		jsonError(w, http.StatusServiceUnavailable, "assignment preview unavailable")
		return
	}

	var req previewProjectAssignmentsRequest
	if err := decodeJSON(r, &req); err != nil {
		if !errors.Is(err, io.EOF) {
			jsonError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	stageFilter := strings.ToLower(strings.TrimSpace(req.Stage))
	if stageFilter != "" && stageFilter != "plan" && stageFilter != "build" && stageFilter != "test" {
		jsonError(w, http.StatusBadRequest, "stage must be one of: plan, build, test")
		return
	}

	pb := h.resolveProjectPlaybook(project)
	if pb == nil {
		jsonError(w, http.StatusBadRequest, "project playbook not found")
		return
	}
	roleDefs := collectPlaybookRoleDefs(pb, stageFilter)
	agents := h.registry.List()
	agentByID := make(map[string]string, len(agents))
	allAgentIDs := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		id := strings.TrimSpace(agent.ID)
		if id == "" {
			continue
		}
		agentByID[id] = strings.TrimSpace(agent.Model)
		allAgentIDs = append(allAgentIDs, id)
	}
	sort.Strings(allAgentIDs)

	current, err := h.roleAgentAssignRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	currentByRole := make(map[string]*db.RoleAgentAssignment, len(current))
	for _, item := range current {
		if item == nil {
			continue
		}
		currentByRole[strings.ToLower(strings.TrimSpace(item.Role))] = item
	}

	items := make([]assignmentPreviewItem, 0, len(roleDefs))
	for _, def := range roleDefs {
		allowed := append([]string(nil), def.Role.AllowedAgents...)
		if len(allowed) == 0 {
			allowed = append(allowed, allAgentIDs...)
		}
		candidates := make([]string, 0, len(allowed))
		for _, id := range allowed {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := agentByID[id]; ok {
				candidates = append(candidates, id)
			}
		}
		sort.Strings(candidates)
		maxParallel := 1
		selected := ""
		existing := currentByRole[strings.ToLower(strings.TrimSpace(def.Role.Name))]
		if existing != nil {
			if existing.MaxParallel > 0 {
				maxParallel = existing.MaxParallel
			}
			if containsFold(candidates, existing.AgentType) {
				selected = strings.TrimSpace(existing.AgentType)
			}
		}
		if selected == "" && len(candidates) > 0 {
			selected = candidates[0]
		}

		items = append(items, assignmentPreviewItem{
			Stage:            def.Stage,
			Role:             strings.TrimSpace(def.Role.Name),
			Responsibilities: strings.TrimSpace(def.Role.Responsibilities),
			AllowedAgents:    allowed,
			Candidates:       candidates,
			SelectedAgent:    selected,
			SelectedModel:    agentByID[selected],
			MaxParallel:      maxParallel,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"project_id":          projectID,
		"playbook_id":         project.Playbook,
		"stage_filter":        stageFilter,
		"current_assignments": current,
		"items":               items,
	})
}

func (h *handler) confirmProjectAssignments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	project, ok := h.mustGetProject(w, r, projectID)
	if !ok {
		return
	}
	if h.roleAgentAssignRepo == nil || h.registry == nil || h.roleBindingRepo == nil || h.projectOrchestratorRepo == nil {
		jsonError(w, http.StatusServiceUnavailable, "assignment confirmation unavailable")
		return
	}
	var req confirmProjectAssignmentsRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Assignments) == 0 {
		jsonError(w, http.StatusBadRequest, "assignments are required")
		return
	}
	if err := h.projectOrchestratorRepo.EnsureDefaultForProject(r.Context(), projectID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	enforceRoleContract := strings.TrimSpace(project.Playbook) != ""
	roleStageIndex := map[string]map[string]struct{}{}
	if enforceRoleContract {
		pb := h.resolveProjectPlaybook(project)
		if pb == nil {
			jsonError(w, http.StatusBadRequest, "project playbook not found")
			return
		}
		roleStageIndex = buildRoleStageIndex(collectPlaybookRoleDefs(pb, ""))
	}

	profile, err := h.projectOrchestratorRepo.Get(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == nil {
		jsonError(w, http.StatusNotFound, "project orchestrator not found")
		return
	}
	provider := strings.TrimSpace(profile.DefaultProvider)
	if provider == "" {
		provider = "anthropic"
	}

	seenRole := map[string]struct{}{}
	assignments := make([]*db.RoleAgentAssignment, 0, len(req.Assignments))
	bindings := make([]*db.RoleBinding, 0, len(req.Assignments))
	for _, item := range req.Assignments {
		role := strings.TrimSpace(item.Role)
		agentType := strings.TrimSpace(item.AgentType)
		stage := strings.ToLower(strings.TrimSpace(item.Stage))
		if role == "" || agentType == "" {
			jsonError(w, http.StatusBadRequest, "role and agent_type are required")
			return
		}
		roleKey := strings.ToLower(role)
		if enforceRoleContract {
			stagesForRole := roleStageIndex[roleKey]
			if len(stagesForRole) == 0 {
				jsonError(w, http.StatusBadRequest, fmt.Sprintf("role %q is not defined in playbook %q", role, project.Playbook))
				return
			}
			if stage == "" {
				if len(stagesForRole) != 1 {
					jsonError(w, http.StatusBadRequest, fmt.Sprintf("stage is required for role %q", role))
					return
				}
				stage = onlyStage(stagesForRole)
			}
			if _, ok := stagesForRole[stage]; !ok {
				jsonError(w, http.StatusBadRequest, fmt.Sprintf("role %q is not defined for stage %q in playbook %q", role, stage, project.Playbook))
				return
			}
		}
		if _, dup := seenRole[roleKey]; dup {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("duplicate role assignment: %s", role))
			return
		}
		seenRole[roleKey] = struct{}{}
		agent := h.registry.Get(agentType)
		if agent == nil {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown agent_type %q", agentType))
			return
		}
		maxParallel := item.MaxParallel
		if maxParallel <= 0 {
			maxParallel = 1
		}
		if maxParallel > 64 {
			jsonError(w, http.StatusBadRequest, "max_parallel must be between 1 and 64")
			return
		}
		model := strings.TrimSpace(agent.Model)
		if model == "" {
			model = "default"
		}
		assignments = append(assignments, &db.RoleAgentAssignment{
			ProjectID:   projectID,
			Stage:       stage,
			Role:        role,
			AgentType:   agentType,
			MaxParallel: maxParallel,
		})
		bindings = append(bindings, &db.RoleBinding{
			ProjectID:   projectID,
			Role:        role,
			Provider:    provider,
			Model:       model,
			MaxParallel: maxParallel,
		})
	}

	if err := h.roleAgentAssignRepo.ReplaceForProject(r.Context(), projectID, assignments); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.roleBindingRepo.ReplaceForProject(r.Context(), projectID, bindings); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := h.roleAgentAssignRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	savedBindings, err := h.roleBindingRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.runRepo != nil {
		run, err := h.runRepo.GetActiveByProject(r.Context(), projectID)
		if err == nil && run != nil {
			evidence, _ := json.Marshal(map[string]any{
				"assignment_count": len(saved),
				"event":            "assignment_confirmed",
			})
			_ = h.runRepo.UpsertStageRun(r.Context(), run.ID, run.CurrentStage, "active", string(evidence))
		}
	}
	if h.hub != nil {
		h.hub.BroadcastProjectEvent(projectID, "assignment_state", map[string]any{
			"count": len(saved),
		})
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"assignments":   saved,
		"role_bindings": savedBindings,
	})
}

func (h *handler) resolveProjectPlaybook(project *db.Project) *playbook.Playbook {
	if h == nil || h.playbookRegistry == nil || project == nil {
		return nil
	}
	if overrideID := strings.TrimSpace(project.Playbook); overrideID != "" {
		if pb := h.playbookRegistry.Get(overrideID); pb != nil {
			return pb
		}
	}
	return h.playbookRegistry.MatchProject(project.RepoPath)
}

func collectPlaybookRoleDefs(pb *playbook.Playbook, stageFilter string) []stageRoleDef {
	if pb == nil {
		return nil
	}
	defs := make([]stageRoleDef, 0)
	appendStage := func(stage string, def playbook.Stage) {
		if !def.Enabled {
			return
		}
		if stageFilter != "" && stageFilter != stage {
			return
		}
		for _, role := range def.Roles {
			name := strings.TrimSpace(role.Name)
			if name == "" {
				continue
			}
			defs = append(defs, stageRoleDef{Stage: stage, Role: role})
		}
	}
	appendStage("plan", pb.Workflow.Plan)
	appendStage("build", pb.Workflow.Build)
	appendStage("test", pb.Workflow.Test)
	return defs
}

func buildRoleStageIndex(defs []stageRoleDef) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{}, len(defs))
	for _, def := range defs {
		role := strings.ToLower(strings.TrimSpace(def.Role.Name))
		stage := strings.ToLower(strings.TrimSpace(def.Stage))
		if role == "" || stage == "" {
			continue
		}
		if _, ok := out[role]; !ok {
			out[role] = make(map[string]struct{}, 1)
		}
		out[role][stage] = struct{}{}
	}
	return out
}

func onlyStage(stages map[string]struct{}) string {
	for stage := range stages {
		return stage
	}
	return ""
}

func containsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
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
