package api

import (
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
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
	jsonResponse(w, http.StatusOK, issue)
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
