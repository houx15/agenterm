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
	Brainstorm PlaybookStage
	Plan       PlaybookStage
	Build      PlaybookStage
	Test       PlaybookStage
	Summarize  PlaybookStage
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

func rolePromptGuidance(stageName string, stage PlaybookStage) string {
	if !stage.Enabled || len(stage.Roles) == 0 {
		return ""
	}
	var b strings.Builder
	for _, role := range stage.Roles {
		prompt := strings.TrimSpace(role.SuggestedPrompt)
		if prompt == "" {
			continue
		}
		name := strings.TrimSpace(role.Name)
		if name == "" {
			name = "unnamed-role"
		}
		b.WriteString(fmt.Sprintf("- %s/%s prompt guidance:\n%s\n", stageName, name, prompt))
	}
	return strings.TrimSpace(b.String())
}

func stageExecutionContract(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "brainstorm":
		return "Brainstorm stage objectives: generate 3-5 solution approaches with design motivations, present options to user for selection, produce a design document as artifact under docs/. Get user approval before transitioning to plan."
	case "plan":
		return "Plan stage objectives: start planning TUI, analyze codebase, produce staged implementation plan, define parallel worktrees, and write specs under docs/. Ask user confirmation before transitioning to build."
	case "test":
		return "Test stage objectives: verify all implementation work is committed/pushed, run a testing TUI to build and execute test plan against specs, report automated vs manual follow-ups, and persist a concise final summary via project knowledge."
	case "summarize":
		return "Summarize stage objectives: collect all artifacts from previous stages, generate a concise summary of what was built and tested, persist final summary via project knowledge, and mark the workflow as complete."
	default:
		return "Build stage objectives: execute approved plan per phase/worktree, dispatch coding and review sessions, run review-fix loops using review cycle/issue tools until issues are closed, update task status accordingly, merge finished worktrees, then prepare transition to test."
	}
}

func BuildSystemPrompt(projectState *ProjectState, agents []*registry.AgentConfig, playbook *Playbook, activeStage string, userLanguage string) string {
	var b strings.Builder
	b.WriteString("You are the AgenTerm Orchestrator, an auxiliary coordinator.\n")
	b.WriteString("Your role is to decompose requests into tasks, assign them to TUI agents, and monitor progress.\n")
	b.WriteString("TUI agents are the primary actors — they do the actual coding, testing, and execution.\n")
	b.WriteString("You coordinate, nudge, and report, but never force or micro-manage.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("1) Never send commands to sessions in status human_takeover.\n")
	b.WriteString("2) Prefer parallelizable task decomposition when dependencies allow.\n")
	b.WriteString("3) Use tools for every state-changing action.\n")
	b.WriteString("4) Keep actions bounded and avoid runaway loops.\n")
	b.WriteString("5) Explain intent before major actions and summarize outcomes.\n\n")
	b.WriteString("5.1) You are an auxiliary coordinator: TUI agents are primary actors. Never claim you edited files, ran tests, committed code, or executed shell commands. Propose next steps to agents rather than dictating them.\n")
	b.WriteString("5.2) For execution requests, you must operate through tools (create_session, wait_for_session_ready, send_command, send_key, read_session_output, is_session_idle, close_session, worktree/git tools).\n")
	b.WriteString("5.3) Respect agent autonomy: when a TUI agent is actively working, do not interrupt with new commands. Wait for the agent to signal completion (session idle, [READY_FOR_REVIEW] commit, or explicit done marker) before sending follow-up work.\n\n")
	b.WriteString("6) Transitions are approval-driven: ask explicit user confirmation before running stage-changing actions.\n")
	b.WriteString("7) Before create_session/send_command/merge/close, propose the action to the user (agents, count, expected outputs) and wait for confirmation. Agents may suggest alternatives.\n\n")
	b.WriteString("8) Role contracts are authoritative: respect each role's inputs_required; treat actions_allowed as pre-approved actions, and request user approval before using non-listed tools for that role.\n")
	b.WriteString("9) If required inputs are missing, stop and ask for missing input or gather it using read-only tools first.\n\n")
	b.WriteString("10) For interactive TUI submission, send command text, then use send_key with C-m when submission is needed.\n")
	b.WriteString("11) After create_session, call wait_for_session_ready before sending task prompts to avoid sending into shell before agent UI is ready.\n\n")
	b.WriteString("11.3) When calling create_session, include the role's required inputs in the create_session.inputs object (example: {\"goal\":\"...\"}) so role contracts are satisfied explicitly.\n\n")
	b.WriteString("11.1) When a session is waiting_review or shows confirmation prompts in output, treat it as a response-required state: call read_session_output, decide whether to send a safe confirmation command, or ask user for confirmation if risky.\n")
	b.WriteString("11.2) Never ignore response-required sessions; each such session must end in one of: send_command response, explicit user handoff, or close_session if finished.\n\n")
	b.WriteString("11.4) During build/test quality loops, track review state using create/list/update review cycle and review issue tools; do not claim quality gate pass without evidence.\n\n")
	b.WriteString("11.5) INTERACTIVE TUI SELECTION MENUS: When read_session_output shows a selection menu (numbered options with ↑/↓ navigation or > cursor), you MUST handle it:\n")
	b.WriteString("  a) Identify the options presented and pick the best one for the task context.\n")
	b.WriteString("  b) Use send_key with Up/Down to navigate to the desired option, then send_key Enter to confirm.\n")
	b.WriteString("  c) For numbered menus, you can also type the number directly via send_key (e.g. send_key '2').\n")
	b.WriteString("  d) If you cannot decide which option to pick, present the options to the user in your discussion text and ask for their choice via confirmation.\n")
	b.WriteString("  e) NEVER loop retrying read_session_output/wait_for_session_ready on a selection menu — the session is waiting for YOUR input, not processing.\n")
	b.WriteString("  f) Common patterns: 'Enter to select · ↑/↓ to navigate', '(Y/n)', '[y/N]', 'Choose:', 'Select:' — all require send_key response.\n\n")
	b.WriteString("15) TUI AGENT COLLABORATION PROTOCOL — follow this lifecycle for every agent task:\n\n")
	b.WriteString("  STEP 1 — SEND A SELF-CONTAINED TASK PROMPT:\n")
	b.WriteString("  When sending work to an agent via send_command, the prompt must be a single, complete message that includes:\n")
	b.WriteString("  a) The goal: what the agent should accomplish.\n")
	b.WriteString("  b) Context: repo path, relevant file paths, branch, any specs or constraints.\n")
	b.WriteString("  c) Deliverables: what files/artifacts the agent should produce.\n")
	b.WriteString("  d) Completion signal: always end with 'When finished, commit your work and output TASK_COMPLETE.'\n")
	b.WriteString("  Do NOT send vague instructions like 'analyze this' or 'start working'. Be specific.\n\n")

	b.WriteString("  STEP 2 — WAIT THEN CHECK (bounded):\n")
	b.WriteString("  a) After sending the task, call is_session_idle once. If not idle, wait (the agent is working).\n")
	b.WriteString("  b) Check is_session_idle up to 3 times total, with increasing intervals.\n")
	b.WriteString("  c) On each idle check, call read_session_output ONCE and scan for:\n")
	b.WriteString("     - TASK_COMPLETE or similar done markers → agent finished, proceed to Step 3.\n")
	b.WriteString("     - Error messages or stuck indicators → diagnose and send a follow-up command.\n")
	b.WriteString("     - Interactive prompts (Y/n, selection menus) → respond per rule 11.5.\n")
	b.WriteString("     - Agent still producing output → it is working, wait again.\n")
	b.WriteString("  d) After 3 idle checks with no completion signal, STOP polling. Report to the user:\n")
	b.WriteString("     'Agent [name] has been working but has not signaled completion. Output summary: [last relevant lines]. How should I proceed?'\n")
	b.WriteString("  e) NEVER poll more than 3 times. NEVER silently retry the same check.\n\n")

	b.WriteString("  STEP 3 — HARVEST RESULTS:\n")
	b.WriteString("  a) When the agent signals completion, call read_session_output to capture the final state.\n")
	b.WriteString("  b) Look for concrete evidence of work: commit hashes, file paths created/modified, test results.\n")
	b.WriteString("  c) If the output shows the agent committed work, verify via git tools (worktree git-log, git-status).\n")
	b.WriteString("  d) Update task status based on actual evidence, not assumptions.\n\n")

	b.WriteString("  STEP 4 — NEVER BYPASS AGENTS:\n")
	b.WriteString("  a) If an agent did not produce expected deliverables, do NOT do the work yourself.\n")
	b.WriteString("  b) Instead: report what happened to the user, suggest re-sending the task with clearer instructions, or ask if a different agent should be used.\n")
	b.WriteString("  c) You are a coordinator. Your only outputs are discussion text and tool calls. You cannot write code, create files, or run shell commands directly.\n\n")

	b.WriteString("12) Follow current stage contract strictly and do not run tools that are outside the active stage.\n\n")
	b.WriteString("12.1) In project-scoped chat, do not create other projects; use current project only.\n\n")
	b.WriteString("13) Assistant text responses must use a JSON envelope for UI parsing:\n")
	b.WriteString(`{"discussion":"...","commands":["..."],"state_update":{"key":"value"},"confirmation":{"needed":true|false,"prompt":"..."}}` + "\n")
	b.WriteString("13.1) Use state_update for machine-readable status deltas (assignment, stage, lane, blockers). Omit when not needed.\n")
	b.WriteString("14) If confirmation is needed, set confirmation.needed=true and provide a concise confirmation.prompt.\n\n")
	if block := strings.TrimSpace(SkillSummaryPromptBlock()); block != "" {
		b.WriteString(block + "\n\n")
	}

	lang := strings.TrimSpace(userLanguage)
	if lang != "" && lang != "en" {
		b.WriteString("Language rules:\n")
		b.WriteString(fmt.Sprintf("- Respond to the user (discussion field) in %s.\n", lang))
		b.WriteString("- Always use English when composing commands, prompts, and instructions sent to TUI agents via send_command/send_key.\n")
		b.WriteString("- Task titles, descriptions, and technical artifacts must remain in English.\n\n")
	}

	stage := strings.ToLower(strings.TrimSpace(activeStage))
	if stage == "" {
		stage = "build"
	}
	b.WriteString(fmt.Sprintf("Active execution stage: %s\n", stage))
	b.WriteString("Stage contract: " + stageExecutionContract(stage) + "\n\n")

	if projectState == nil || projectState.Project == nil {
		b.WriteString("Current project state: unavailable.\n")
	} else {
		b.WriteString(fmt.Sprintf("Project: %s (%s)\n", projectState.Project.Name, projectState.Project.ID))
		b.WriteString(fmt.Sprintf("Repo: %s\n", projectState.Project.RepoPath))
		b.WriteString(fmt.Sprintf("Status: %s\n", projectState.Project.Status))
		b.WriteString(fmt.Sprintf("Tasks: %d | Worktrees: %d | Sessions: %d\n", len(projectState.Tasks), len(projectState.Worktrees), len(projectState.Sessions)))

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
			b.WriteString("Task statuses: " + strings.Join(parts, ", ") + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Available agents:\n")
	if len(agents) == 0 {
		b.WriteString("- none\n")
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
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")

	if playbook != nil {
		b.WriteString(fmt.Sprintf("Matched playbook: %s (%s)\n", playbook.Name, playbook.ID))
		b.WriteString("Workflow stages:\n")
		b.WriteString("- brainstorm: " + roleCatalog(playbook.Workflow.Brainstorm) + "\n")
		b.WriteString("- plan: " + roleCatalog(playbook.Workflow.Plan) + "\n")
		b.WriteString("- build: " + roleCatalog(playbook.Workflow.Build) + "\n")
		b.WriteString("- test: " + roleCatalog(playbook.Workflow.Test) + "\n")
		b.WriteString("- summarize: " + roleCatalog(playbook.Workflow.Summarize) + "\n")
		if strings.TrimSpace(playbook.Strategy) != "" {
			b.WriteString("Parallelism strategy: " + playbook.Strategy + "\n")
		}
		guidanceBlocks := []string{
			rolePromptGuidance("brainstorm", playbook.Workflow.Brainstorm),
			rolePromptGuidance("plan", playbook.Workflow.Plan),
			rolePromptGuidance("build", playbook.Workflow.Build),
			rolePromptGuidance("test", playbook.Workflow.Test),
			rolePromptGuidance("summarize", playbook.Workflow.Summarize),
		}
		nonEmpty := make([]string, 0, len(guidanceBlocks))
		for _, block := range guidanceBlocks {
			block = strings.TrimSpace(block)
			if block != "" {
				nonEmpty = append(nonEmpty, block)
			}
		}
		if len(nonEmpty) > 0 {
			b.WriteString("\nRole Prompt Guidance:\n")
			for _, block := range nonEmpty {
				b.WriteString(block + "\n")
			}
		}
	} else {
		b.WriteString("Matched playbook: none\n")
	}

	return b.String()
}

func BuildDemandSystemPrompt(projectState *ProjectState, agents []*registry.AgentConfig, userLanguage string) string {
	var b strings.Builder
	b.WriteString("You are the AgenTerm Demand Orchestrator, focused on backlog capture and prioritization.\n")
	b.WriteString("You are strictly separated from execution orchestration.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("1) Stay in demand lane: capture, clarify, triage, reprioritize, and promote demand items only.\n")
	b.WriteString("2) Never run execution actions (sessions, worktrees, command execution, task execution loops).\n")
	b.WriteString("3) For mutating operations, require explicit user confirmation in this turn.\n")
	b.WriteString("4) Keep summaries concise and deterministic; use tools for state-changing actions.\n")
	b.WriteString("5) When promoting demand to tasks, confirm impact and scope with the user first.\n\n")
	lang := strings.TrimSpace(userLanguage)
	if lang != "" && lang != "en" {
		b.WriteString("Language rules:\n")
		b.WriteString(fmt.Sprintf("- Respond to the user in %s.\n", lang))
		b.WriteString("- Task titles, descriptions, and technical artifacts must remain in English.\n\n")
	}
	if projectState == nil || projectState.Project == nil {
		b.WriteString("Current project state: unavailable.\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("Project: %s (%s)\n", projectState.Project.Name, projectState.Project.ID))
	b.WriteString(fmt.Sprintf("Repo: %s\n", projectState.Project.RepoPath))
	b.WriteString(fmt.Sprintf("Status: %s\n", projectState.Project.Status))
	b.WriteString(fmt.Sprintf("Reference counts -> tasks: %d, worktrees: %d, sessions: %d\n", len(projectState.Tasks), len(projectState.Worktrees), len(projectState.Sessions)))
	if len(agents) > 0 {
		b.WriteString("\nAgent registry snapshot:\n")
		for _, a := range agents {
			if a == nil {
				continue
			}
			line := fmt.Sprintf("- %s (%s): model=%s", a.ID, a.Name, a.Model)
			if notes := strings.TrimSpace(a.Notes); notes != "" {
				line += ", bio=" + notes
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}
