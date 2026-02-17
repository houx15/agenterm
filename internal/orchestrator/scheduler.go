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

	projectSessions, err := o.listSessionsByProject(ctx, projectID)
	if err != nil {
		return scheduleDecision{Allowed: false, Reason: err.Error()}
	}

	activeProject := 0
	activeRole := 0
	activeModel := 0
	model := o.resolveModelForSession(agentType)
	for _, sess := range projectSessions {
		if !isActiveSessionStatus(sess.Status) {
			continue
		}
		activeProject++
		if strings.EqualFold(strings.TrimSpace(sess.Role), strings.TrimSpace(role)) {
			activeRole++
		}
		if model != "" {
			sessModel := o.resolveModelForSession(sess.AgentType)
			if sessModel == model {
				activeModel++
			}
		}
	}

	if profile.MaxParallel > 0 && activeProject >= profile.MaxParallel {
		return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("project max_parallel limit reached (%d)", profile.MaxParallel)}
	}

	if o.roleBindingRepo != nil && strings.TrimSpace(role) != "" {
		bindings, err := o.roleBindingRepo.ListByProject(ctx, projectID)
		if err == nil {
			for _, b := range bindings {
				if b == nil {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(b.Role), strings.TrimSpace(role)) {
					if b.MaxParallel > 0 && activeRole >= b.MaxParallel {
						return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("role max_parallel limit reached for %s (%d)", role, b.MaxParallel)}
					}
					if strings.TrimSpace(b.Model) != "" && strings.TrimSpace(model) == strings.TrimSpace(b.Model) && b.MaxParallel > 0 && activeModel >= b.MaxParallel {
						return scheduleDecision{Allowed: false, Reason: fmt.Sprintf("model max_parallel limit reached for %s (%d)", b.Model, b.MaxParallel)}
					}
				}
			}
		}
	}

	if o.registry != nil && strings.TrimSpace(agentType) != "" {
		agent := o.registry.Get(agentType)
		if agent != nil && agent.MaxParallelAgents > 0 {
			activeAgentType := 0
			for _, sess := range projectSessions {
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
	case "completed", "failed", "human_takeover":
		return false
	default:
		return status != ""
	}
}
