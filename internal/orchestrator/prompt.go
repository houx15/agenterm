package orchestrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/registry"
)

type ProjectState struct {
	Project   *db.Project
	Tasks     []*db.Task
	Worktrees []*db.Worktree
	Sessions  []*db.Session
}

type Playbook struct {
	ID       string
	Name     string
	Workflow PlaybookWorkflow
	Strategy string
}

type PlaybookWorkflow struct {
	Plan  PlaybookStage
	Build PlaybookStage
	Test  PlaybookStage
}

type PlaybookStage struct {
	Enabled bool
	Roles   []PlaybookRole
}

type PlaybookRole struct {
	Name             string
	Mode             string
	Responsibilities string
	AllowedAgents    []string
	InputsRequired   []string
	ActionsAllowed   []string
	SuggestedPrompt  string
}

func roleCatalog(stage PlaybookStage) string {
	if !stage.Enabled {
		return "disabled"
	}
	if len(stage.Roles) == 0 {
		return "enabled (no roles configured)"
	}
	parts := make([]string, 0, len(stage.Roles))
	for _, role := range stage.Roles {
		desc := strings.TrimSpace(role.Name)
		if desc == "" {
			desc = "unnamed-role"
		}
		if len(role.AllowedAgents) > 0 {
			desc += fmt.Sprintf(" [agents: %s]", strings.Join(role.AllowedAgents, ", "))
		}
		if mode := strings.TrimSpace(role.Mode); mode != "" {
			desc += " [mode: " + mode + "]"
		}
		if len(role.InputsRequired) > 0 {
			desc += " [inputs: " + strings.Join(role.InputsRequired, ", ") + "]"
		}
		if len(role.ActionsAllowed) > 0 {
			desc += " [actions: " + strings.Join(role.ActionsAllowed, ", ") + "]"
		}
		if strings.TrimSpace(role.Responsibilities) != "" {
			desc += ": " + role.Responsibilities
		}
		parts = append(parts, desc)
	}
	return strings.Join(parts, " | ")
}

func BuildSystemPrompt(projectState *ProjectState, agents []*registry.AgentConfig, playbook *Playbook) string {
	var b strings.Builder
	b.WriteString("You are the AgenTerm Orchestrator, an AI project manager.\\n")
	b.WriteString("You decompose requests into actionable tasks, prefer safe parallel execution, and report clearly.\\n\\n")
	b.WriteString("Rules:\\n")
	b.WriteString("1) Never send commands to sessions in status human_takeover.\\n")
	b.WriteString("2) Prefer parallelizable task decomposition when dependencies allow.\\n")
	b.WriteString("3) Use tools for every state-changing action.\\n")
	b.WriteString("4) Keep actions bounded and avoid runaway loops.\\n")
	b.WriteString("5) Explain intent before major actions and summarize outcomes.\\n\\n")
	b.WriteString("6) Transitions are approval-driven: ask explicit user confirmation before running stage-changing actions.\\n")
	b.WriteString("7) Before create_session/send_command/merge/close, provide a brief execution proposal (agents, count, outputs) and wait for confirmation.\\n\\n")
	b.WriteString("8) Role contracts are authoritative: respect each role's inputs_required and actions_allowed.\\n")
	b.WriteString("9) If required inputs are missing, stop and ask for missing input or gather it using read-only tools first.\\n\\n")
	if block := strings.TrimSpace(SkillSummaryPromptBlock()); block != "" {
		b.WriteString(block + "\\n\\n")
	}

	if projectState == nil || projectState.Project == nil {
		b.WriteString("Current project state: unavailable.\\n")
	} else {
		b.WriteString(fmt.Sprintf("Project: %s (%s)\\n", projectState.Project.Name, projectState.Project.ID))
		b.WriteString(fmt.Sprintf("Repo: %s\\n", projectState.Project.RepoPath))
		b.WriteString(fmt.Sprintf("Status: %s\\n", projectState.Project.Status))
		b.WriteString(fmt.Sprintf("Tasks: %d | Worktrees: %d | Sessions: %d\\n", len(projectState.Tasks), len(projectState.Worktrees), len(projectState.Sessions)))

		statusCounts := map[string]int{}
		for _, t := range projectState.Tasks {
			statusCounts[t.Status]++
		}
		if len(statusCounts) > 0 {
			keys := make([]string, 0, len(statusCounts))
			for k := range statusCounts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%s=%d", k, statusCounts[k]))
			}
			b.WriteString("Task statuses: " + strings.Join(parts, ", ") + "\\n")
		}
		b.WriteString("\\n")
	}

	b.WriteString("Available agents:\\n")
	if len(agents) == 0 {
		b.WriteString("- none\\n")
	} else {
		for _, a := range agents {
			line := fmt.Sprintf("- %s (%s): model=%s, max_parallel=%d, speed=%s, cost=%s",
				a.ID, a.Name, a.Model, a.MaxParallelAgents, a.SpeedTier, a.CostTier)
			bio := strings.TrimSpace(a.Notes)
			if bio != "" {
				line += ", bio=" + bio
			}
			if len(a.Capabilities) > 0 {
				line += ", capabilities=" + strings.Join(a.Capabilities, "/")
			}
			if len(a.Languages) > 0 {
				line += ", languages=" + strings.Join(a.Languages, "/")
			}
			b.WriteString(line + "\\n")
		}
	}
	b.WriteString("\\n")

	if playbook != nil {
		b.WriteString(fmt.Sprintf("Matched playbook: %s (%s)\\n", playbook.Name, playbook.ID))
		b.WriteString("Workflow stages:\\n")
		b.WriteString("- plan: " + roleCatalog(playbook.Workflow.Plan) + "\\n")
		b.WriteString("- build: " + roleCatalog(playbook.Workflow.Build) + "\\n")
		b.WriteString("- test: " + roleCatalog(playbook.Workflow.Test) + "\\n")
		if strings.TrimSpace(playbook.Strategy) != "" {
			b.WriteString("Parallelism strategy: " + playbook.Strategy + "\\n")
		}
	} else {
		b.WriteString("Matched playbook: none\\n")
	}

	return b.String()
}
