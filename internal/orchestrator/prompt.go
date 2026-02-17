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
	Phases   []PlaybookPhase
	Strategy string
}

type PlaybookPhase struct {
	Name        string
	Description string
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
			b.WriteString(fmt.Sprintf("- %s (%s): capabilities=%s, languages=%s, speed=%s, cost=%s\\n",
				a.ID,
				a.Name,
				strings.Join(a.Capabilities, "/"),
				strings.Join(a.Languages, "/"),
				a.SpeedTier,
				a.CostTier,
			))
		}
	}
	b.WriteString("\\n")

	if playbook != nil {
		b.WriteString(fmt.Sprintf("Matched playbook: %s (%s)\\n", playbook.Name, playbook.ID))
		for _, phase := range playbook.Phases {
			b.WriteString(fmt.Sprintf("- %s: %s\\n", phase.Name, phase.Description))
		}
		if strings.TrimSpace(playbook.Strategy) != "" {
			b.WriteString("Parallelism strategy: " + playbook.Strategy + "\\n")
		}
	} else {
		b.WriteString("Matched playbook: none\\n")
	}

	return b.String()
}
