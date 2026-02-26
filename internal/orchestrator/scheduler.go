package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type scheduleDecision struct {
	Allowed bool
	Reason  string
}

func (o *Orchestrator) checkSessionCreationAllowed(ctx context.Context, args map[string]any) scheduleDecision {
	if o == nil || o.sessionRepo == nil || o.taskRepo == nil || o.projectOrchestratorRepo == nil {
		return scheduleDecision{Allowed: true}
	}
	taskID, err := requiredString(args, "task_id")
	if err != nil {
		return scheduleDecision{Allowed: false, Reason: err.Error()}
	}
	role, _ := optionalString(args, "role")
	agentType, _ := optionalString(args, "agent_type")

	task, err := o.taskRepo.Get(ctx, taskID)
	if err != nil {
		return scheduleDecision{Allowed: false, Reason: err.Error()}
	}
	if task == nil {
		return scheduleDecision{Allowed: false, Reason: "task not found"}
	}
	projectID := task.ProjectID
	if strings.TrimSpace(projectID) == "" {
		return scheduleDecision{Allowed: false, Reason: "task has no project_id"}
	}

	profile, err := o.projectOrchestratorRepo.Get(ctx, projectID)
	if err != nil || profile == nil {
		return scheduleDecision{Allowed: true}
	}
	roleAssignment := o.getRoleAgentAssignment(ctx, projectID, strings.TrimSpace(role))
	if roleAssignment != nil {
		if strings.TrimSpace(agentType) == "" {
			return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("agent_type is required for assigned role %s", role)}
		}
		if !strings.EqualFold(strings.TrimSpace(roleAssignment.AgentType), strings.TrimSpace(agentType)) {
			return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("role %s is assigned to agent %s, got %s", role, roleAssignment.AgentType, agentType)}
		}
	}

	projectSessions, err := o.listSessionsByProject(ctx, projectID)
	if err != nil {
		return scheduleDecision{Allowed: false, Reason: err.Error()}
	}
	allSessions, err := o.sessionRepo.List(ctx, db.SessionFilter{})
	if err != nil {
		return scheduleDecision{Allowed: false, Reason: err.Error()}
	}

	activeProject := 0
	activeRole := 0
	activeModel := 0
	activeGlobal := 0
	workflowRoleLimit, workflowRoleKnown := o.resolveWorkflowRolePolicy(ctx, profile.WorkflowID, strings.TrimSpace(role))
	_ = workflowRoleKnown
	targetModel, roleBinding, modelMismatch := o.resolveTargetModel(ctx, projectID, strings.TrimSpace(role), strings.TrimSpace(agentType), profile)
	if modelMismatch != "" {
		return scheduleDecision{Allowed: false, Reason: modelMismatch}
	}
	for _, sess := range projectSessions {
		if !isActiveSessionStatus(sess.Status) {
			continue
		}
		activeProject++
		if strings.EqualFold(strings.TrimSpace(sess.Role), strings.TrimSpace(role)) {
			activeRole++
		}
	}
	taskCache := map[string]*db.Task{}
	bindingCache := map[string][]*db.RoleBinding{}
	for _, sess := range allSessions {
		if !isActiveSessionStatus(sess.Status) {
			continue
		}
		activeGlobal++
		if targetModel != "" {
			sessModel := o.resolveModelForExistingSession(ctx, sess, taskCache, bindingCache)
			if sessModel != "" && strings.EqualFold(strings.TrimSpace(sessModel), strings.TrimSpace(targetModel)) {
				activeModel++
			}
		}
	}

	if o.globalMaxParallel > 0 && activeGlobal >= o.globalMaxParallel {
		return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("global max_parallel limit reached (%d)", o.globalMaxParallel)}
	}
	if profile.MaxParallel > 0 && activeProject >= profile.MaxParallel {
		return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("project max_parallel limit reached (%d)", profile.MaxParallel)}
	}
	if workflowRoleLimit > 0 && activeRole >= workflowRoleLimit {
		return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("workflow phase max_parallel limit reached for role %s (%d)", role, workflowRoleLimit)}
	}

	if roleBinding != nil {
		if roleBinding.MaxParallel > 0 && activeRole >= roleBinding.MaxParallel {
			return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("role max_parallel limit reached for %s (%d)", role, roleBinding.MaxParallel)}
		}
		if targetModel != "" && roleBinding.MaxParallel > 0 && activeModel >= roleBinding.MaxParallel {
			return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("model max_parallel limit reached for %s (%d)", targetModel, roleBinding.MaxParallel)}
		}
	}
	if roleAssignment != nil && roleAssignment.MaxParallel > 0 && activeRole >= roleAssignment.MaxParallel {
		return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("assigned agent max_parallel limit reached for %s (%d)", role, roleAssignment.MaxParallel)}
	}

	if o.registry != nil && strings.TrimSpace(agentType) != "" {
		agent := o.registry.Get(agentType)
		if agent != nil && agent.MaxParallelAgents > 0 {
			activeAgentType := 0
			for _, sess := range allSessions {
				if isActiveSessionStatus(sess.Status) && strings.EqualFold(strings.TrimSpace(sess.AgentType), strings.TrimSpace(agentType)) {
					activeAgentType++
				}
			}
			if activeAgentType >= agent.MaxParallelAgents {
				return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("agent max_parallel limit reached for %s (%d)", agentType, agent.MaxParallelAgents)}
			}
		}
	}

	return scheduleDecision{Allowed: true}
}

func (o *Orchestrator) getRoleAgentAssignment(ctx context.Context, projectID string, role string) *db.RoleAgentAssignment {
	if o == nil || o.roleAgentAssignRepo == nil || strings.TrimSpace(projectID) == "" || strings.TrimSpace(role) == "" {
		return nil
	}
	items, err := o.roleAgentAssignRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Role), strings.TrimSpace(role)) {
			return item
		}
	}
	return nil
}

func (o *Orchestrator) resolveModelForSession(agentType string) string {
	if o == nil || o.registry == nil || strings.TrimSpace(agentType) == "" {
		return ""
	}
	agent := o.registry.Get(agentType)
	if agent == nil {
		return ""
	}
	return strings.TrimSpace(agent.Model)
}

func (o *Orchestrator) resolveTargetModel(ctx context.Context, projectID string, role string, agentType string, profile *db.ProjectOrchestrator) (string, *db.RoleBinding, string) {
	requestedModel := o.resolveModelForSession(agentType)
	binding := o.getRoleBinding(ctx, projectID, role)
	if binding != nil && strings.TrimSpace(binding.Model) != "" {
		boundModel := strings.TrimSpace(binding.Model)
		if requestedModel != "" && !strings.EqualFold(requestedModel, boundModel) {
			return "", binding, fmt.Sprintf("agent model mismatch for role %s: expected %s, got %s", role, boundModel, requestedModel)
		}
		return boundModel, binding, ""
	}
	if requestedModel != "" {
		return requestedModel, binding, ""
	}
	if profile != nil && strings.TrimSpace(profile.DefaultModel) != "" {
		return strings.TrimSpace(profile.DefaultModel), binding, ""
	}
	return "", binding, ""
}

func (o *Orchestrator) resolveWorkflowRolePolicy(ctx context.Context, workflowID string, role string) (int, bool) {
	workflowID = strings.TrimSpace(workflowID)
	role = strings.ToLower(strings.TrimSpace(role))
	if o == nil || o.workflowRepo == nil || workflowID == "" || role == "" {
		return 0, true
	}
	workflow, err := o.workflowRepo.Get(ctx, workflowID)
	if err != nil || workflow == nil {
		return 0, true
	}
	limit := 0
	seenRole := false
	for _, phase := range workflow.Phases {
		if phase == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(phase.Role), role) {
			seenRole = true
			if phase.MaxParallel > 0 && (limit == 0 || phase.MaxParallel < limit) {
				limit = phase.MaxParallel
			}
		}
	}
	if !seenRole {
		return 0, false
	}
	return limit, true
}

func (o *Orchestrator) getRoleBinding(ctx context.Context, projectID string, role string) *db.RoleBinding {
	if o == nil || o.roleBindingRepo == nil || strings.TrimSpace(projectID) == "" || strings.TrimSpace(role) == "" {
		return nil
	}
	bindings, err := o.roleBindingRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil
	}
	for _, b := range bindings {
		if b == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(b.Role), strings.TrimSpace(role)) {
			return b
		}
	}
	return nil
}

func (o *Orchestrator) resolveModelForExistingSession(
	ctx context.Context,
	sess *db.Session,
	taskCache map[string]*db.Task,
	bindingCache map[string][]*db.RoleBinding,
) string {
	if sess == nil {
		return ""
	}
	agentModel := strings.TrimSpace(o.resolveModelForSession(sess.AgentType))
	taskID := strings.TrimSpace(sess.TaskID)
	if taskID == "" || o.taskRepo == nil {
		return agentModel
	}
	task := taskCache[taskID]
	if task == nil {
		loaded, err := o.taskRepo.Get(ctx, taskID)
		if err != nil || loaded == nil {
			return agentModel
		}
		task = loaded
		taskCache[taskID] = task
	}
	projectID := strings.TrimSpace(task.ProjectID)
	if projectID == "" {
		return agentModel
	}
	role := strings.TrimSpace(sess.Role)
	if o.roleBindingRepo != nil && role != "" {
		bindings, ok := bindingCache[projectID]
		if !ok {
			loaded, err := o.roleBindingRepo.ListByProject(ctx, projectID)
			if err == nil {
				bindings = loaded
			}
			bindingCache[projectID] = bindings
		}
		for _, b := range bindings {
			if b == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(b.Role), role) && strings.TrimSpace(b.Model) != "" {
				return strings.TrimSpace(b.Model)
			}
		}
	}
	return agentModel
}

func (o *Orchestrator) listSessionsByProject(ctx context.Context, projectID string) ([]*db.Session, error) {
	tasks, err := o.taskRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	sessions := make([]*db.Session, 0)
	for _, t := range tasks {
		items, err := o.sessionRepo.ListByTask(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, items...)
	}
	return sessions, nil
}

func isActiveSessionStatus(status string) bool {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "", "idle", "disconnected", "sleeping", "paused", "completed", "failed", "stopped", "terminated", "closed", "dead":
		return false
	default:
		return true
	}
}
