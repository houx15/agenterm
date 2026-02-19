package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/playbook"
	"github.com/user/agenterm/internal/registry"
)

const (
	defaultModel             = "claude-sonnet-4-5"
	defaultOpenAIModel       = "gpt-4o-mini"
	defaultAnthropicURL      = "https://api.anthropic.com/v1/messages"
	defaultOpenAIURL         = "https://api.openai.com/v1/chat/completions"
	defaultMaxToolRounds     = 10
	defaultMaxHistory        = 50
	defaultGlobalMaxParallel = 32
	maxCommandLedgerEntries  = 500
)

type StreamEvent struct {
	Type   string         `json:"type"`
	Text   string         `json:"text,omitempty"`
	Name   string         `json:"name,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Result any            `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type Options struct {
	APIKey           string
	Model            string
	AnthropicBaseURL string
	APIToolBaseURL   string
	APIToken         string
	HTTPClient       *http.Client

	ProjectRepo             *db.ProjectRepo
	TaskRepo                *db.TaskRepo
	WorktreeRepo            *db.WorktreeRepo
	SessionRepo             *db.SessionRepo
	HistoryRepo             *db.OrchestratorHistoryRepo
	ProjectOrchestratorRepo *db.ProjectOrchestratorRepo
	WorkflowRepo            *db.WorkflowRepo
	KnowledgeRepo           *db.ProjectKnowledgeRepo
	RoleBindingRepo         *db.RoleBindingRepo
	Registry                *registry.Registry
	PlaybookRegistry        *playbook.Registry
	Toolset                 *Toolset

	MaxToolRounds     int
	MaxHistory        int
	GlobalMaxParallel int
}

type Orchestrator struct {
	apiKey           string
	model            string
	anthropicBaseURL string
	httpClient       *http.Client

	projectRepo             *db.ProjectRepo
	taskRepo                *db.TaskRepo
	worktreeRepo            *db.WorktreeRepo
	sessionRepo             *db.SessionRepo
	historyRepo             *db.OrchestratorHistoryRepo
	projectOrchestratorRepo *db.ProjectOrchestratorRepo
	workflowRepo            *db.WorkflowRepo
	knowledgeRepo           *db.ProjectKnowledgeRepo
	roleBindingRepo         *db.RoleBindingRepo
	registry                *registry.Registry
	playbookRegistry        *playbook.Registry
	toolset                 *Toolset

	maxToolRounds     int
	maxHistory        int
	globalMaxParallel int

	commandMu          sync.Mutex
	sessionCommandLock map[string]*sync.Mutex
	commandLedger      []CommandLedgerEntry
	nextCommandID      int64
}

type CommandLedgerEntry struct {
	ID            int64     `json:"id"`
	ToolName      string    `json:"tool_name"`
	SessionID     string    `json:"session_id"`
	Command       string    `json:"command,omitempty"`
	IssuedAt      time.Time `json:"issued_at"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	Status        string    `json:"status"`
	ResultSnippet string    `json:"result_snippet,omitempty"`
	Error         string    `json:"error,omitempty"`
}

type llmConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

func New(opts Options) *Orchestrator {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultModel
	}
	anthropicURL := strings.TrimSpace(opts.AnthropicBaseURL)
	if anthropicURL == "" {
		anthropicURL = defaultAnthropicURL
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	toolset := opts.Toolset
	if toolset == nil {
		toolset = NewToolset(&RESTToolClient{
			BaseURL:    opts.APIToolBaseURL,
			Token:      opts.APIToken,
			HTTPClient: httpClient,
		})
	}
	maxRounds := opts.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxToolRounds
	}
	maxHistory := opts.MaxHistory
	if maxHistory <= 0 {
		maxHistory = defaultMaxHistory
	}
	globalMaxParallel := opts.GlobalMaxParallel
	if globalMaxParallel <= 0 {
		globalMaxParallel = defaultGlobalMaxParallel
	}

	return &Orchestrator{
		apiKey:                  strings.TrimSpace(opts.APIKey),
		model:                   model,
		anthropicBaseURL:        anthropicURL,
		httpClient:              httpClient,
		projectRepo:             opts.ProjectRepo,
		taskRepo:                opts.TaskRepo,
		worktreeRepo:            opts.WorktreeRepo,
		sessionRepo:             opts.SessionRepo,
		historyRepo:             opts.HistoryRepo,
		projectOrchestratorRepo: opts.ProjectOrchestratorRepo,
		workflowRepo:            opts.WorkflowRepo,
		knowledgeRepo:           opts.KnowledgeRepo,
		roleBindingRepo:         opts.RoleBindingRepo,
		registry:                opts.Registry,
		playbookRegistry:        opts.PlaybookRegistry,
		toolset:                 toolset,
		maxToolRounds:           maxRounds,
		maxHistory:              maxHistory,
		globalMaxParallel:       globalMaxParallel,
		sessionCommandLock:      make(map[string]*sync.Mutex),
		commandLedger:           make([]CommandLedgerEntry, 0, 64),
	}
}

func (o *Orchestrator) Enabled() bool {
	return o != nil && strings.TrimSpace(o.apiKey) != ""
}

func (o *Orchestrator) Chat(ctx context.Context, projectID string, userMessage string) (<-chan StreamEvent, error) {
	if o == nil {
		return nil, fmt.Errorf("orchestrator unavailable")
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(userMessage) == "" {
		return nil, fmt.Errorf("message is required")
	}

	state, err := o.loadProjectState(ctx, projectID)
	if err != nil {
		return nil, err
	}
	agents := []*registry.AgentConfig{}
	if o.registry != nil {
		agents = o.registry.List()
	}
	llmCfg, err := o.resolveLLMConfig(ctx, projectID, agents)
	if err != nil {
		return nil, err
	}
	approval := evaluateApprovalGate(userMessage)
	matchedPlaybook := o.loadProjectPlaybook(ctx, state.Project)
	if matchedPlaybook == nil {
		matchedPlaybook = o.loadWorkflowAsPlaybook(ctx, projectID)
	}
	systemPrompt := BuildSystemPrompt(state, agents, matchedPlaybook)
	systemPrompt += "\n\nApproval gate:\n"
	if approval.Confirmed {
		systemPrompt += "- User message includes explicit approval for execution in this turn.\n"
	} else {
		systemPrompt += "- User has NOT provided explicit approval for execution in this turn.\n"
		systemPrompt += "- You may analyze, ask questions, and propose plans.\n"
		systemPrompt += "- Do NOT execute mutating actions until user confirms.\n"
	}
	if o.knowledgeRepo != nil {
		knowledge, err := o.knowledgeRepo.ListByProject(ctx, projectID)
		if err == nil && len(knowledge) > 0 {
			systemPrompt += "\n\nProject Knowledge Highlights:\n"
			limit := len(knowledge)
			if limit > 8 {
				limit = 8
			}
			for i := 0; i < limit; i++ {
				k := knowledge[i]
				systemPrompt += fmt.Sprintf("- [%s] %s: %s\n", k.Kind, k.Title, truncate(k.Content, 180))
			}
		}
	}
	history := o.loadHistory(ctx, projectID)
	messages := append(history, anthropicMessage{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: userMessage}}})

	if o.historyRepo != nil {
		_ = o.historyRepo.Create(ctx, &db.OrchestratorMessage{ProjectID: projectID, Role: "user", Content: userMessage})
	}

	ch := make(chan StreamEvent, 32)
	go func() {
		defer close(ch)
		finalTexts := make([]string, 0, 4)

		for round := 0; round < o.maxToolRounds; round++ {
			resp, err := o.createMessage(ctx, anthropicRequest{
				Model:     llmCfg.Model,
				MaxTokens: 1024,
				System:    systemPrompt,
				Tools:     o.toolset.JSONSchemas(),
				Messages:  messages,
			}, llmCfg)
			if err != nil {
				ch <- StreamEvent{Type: "error", Error: err.Error()}
				return
			}

			assistantMsg := anthropicMessage{Role: "assistant", Content: resp.Content}
			messages = append(messages, assistantMsg)

			toolUsed := false
			for _, block := range resp.Content {
				switch block.Type {
				case "text":
					text := strings.TrimSpace(block.Text)
					if text != "" {
						finalTexts = append(finalTexts, text)
						ch <- StreamEvent{Type: "token", Text: text}
					}
				case "tool_use":
					toolUsed = true
					ch <- StreamEvent{Type: "tool_call", Name: block.Name, Args: block.Input}
					if isMutatingTool(block.Name) && !approval.Confirmed {
						result := map[string]any{
							"error":  "approval_required",
							"reason": "explicit user confirmation required before executing mutating actions",
							"hint":   "ask user to reply with an explicit approval, then retry",
						}
						ch <- StreamEvent{Type: "tool_result", Name: block.Name, Result: result}
						messages = append(messages, anthropicMessage{
							Role: "user",
							Content: []anthropicContentBlock{{
								Type:      "tool_result",
								ToolUseID: block.ID,
								Content:   toJSON(result),
							}},
						})
						continue
					}
					if block.Name == "create_session" {
						decision := o.checkSessionCreationAllowed(ctx, block.Input)
						if !decision.Allowed {
							result := map[string]any{"error": "scheduler_blocked", "reason": decision.Reason}
							ch <- StreamEvent{Type: "tool_result", Name: block.Name, Result: result}
							messages = append(messages, anthropicMessage{
								Role: "user",
								Content: []anthropicContentBlock{{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   toJSON(result),
								}},
							})
							continue
						}
					}
					result, err := o.executeTool(ctx, block.Name, block.Input)
					if err != nil {
						result = map[string]any{"error": err.Error()}
					}
					ch <- StreamEvent{Type: "tool_result", Name: block.Name, Result: result}
					resultJSON := toJSON(result)
					messages = append(messages, anthropicMessage{
						Role: "user",
						Content: []anthropicContentBlock{{
							Type:      "tool_result",
							ToolUseID: block.ID,
							Content:   resultJSON,
						}},
					})
				}
			}

			if !toolUsed {
				final := strings.TrimSpace(strings.Join(finalTexts, "\n"))
				if final != "" && o.historyRepo != nil {
					_ = o.historyRepo.Create(ctx, &db.OrchestratorMessage{ProjectID: projectID, Role: "assistant", Content: final})
					_ = o.historyRepo.TrimProject(ctx, projectID, o.maxHistory)
				}
				ch <- StreamEvent{Type: "done"}
				return
			}
		}

		ch <- StreamEvent{Type: "error", Error: "max tool call rounds reached"}
	}()

	return ch, nil
}

type approvalGate struct {
	Confirmed bool
}

func evaluateApprovalGate(message string) approvalGate {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return approvalGate{Confirmed: false}
	}
	// Keep this strict: only explicit approve/confirm intent unlocks execution.
	tokens := []string{
		"confirm",
		"approved",
		"approve",
		"go ahead",
		"proceed",
		"start now",
		"run it",
		"execute",
		"continue",
	}
	for _, token := range tokens {
		if strings.Contains(text, token) {
			return approvalGate{Confirmed: true}
		}
	}
	return approvalGate{Confirmed: false}
}

func isMutatingTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "create_project", "create_task", "create_worktree", "merge_worktree", "resolve_merge_conflict",
		"create_session", "send_command", "close_session", "write_task_spec":
		return true
	default:
		return false
	}
}

func (o *Orchestrator) GenerateProgressReport(ctx context.Context, projectID string) (map[string]any, error) {
	if o == nil || o.toolset == nil {
		return nil, fmt.Errorf("orchestrator unavailable")
	}
	raw, err := o.toolset.Execute(ctx, "generate_progress_report", map[string]any{"project_id": projectID})
	if err != nil {
		return nil, err
	}
	report, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected report payload type")
	}
	return report, nil
}

func (o *Orchestrator) RecentCommandLedger(limit int) []CommandLedgerEntry {
	if o == nil || limit == 0 {
		return nil
	}
	o.commandMu.Lock()
	defer o.commandMu.Unlock()
	if len(o.commandLedger) == 0 {
		return nil
	}
	if limit < 0 || limit > len(o.commandLedger) {
		limit = len(o.commandLedger)
	}
	start := len(o.commandLedger) - limit
	out := make([]CommandLedgerEntry, 0, limit)
	out = append(out, o.commandLedger[start:]...)
	return out
}

func (o *Orchestrator) loadProjectState(ctx context.Context, projectID string) (*ProjectState, error) {
	if o.projectRepo == nil || o.taskRepo == nil || o.worktreeRepo == nil || o.sessionRepo == nil {
		return nil, fmt.Errorf("orchestrator repositories unavailable")
	}
	project, err := o.projectRepo.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}
	tasks, err := o.taskRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	worktrees, err := o.worktreeRepo.ListByProject(ctx, projectID)
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
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})
	return &ProjectState{
		Project:   project,
		Tasks:     tasks,
		Worktrees: worktrees,
		Sessions:  sessions,
	}, nil
}

func (o *Orchestrator) loadHistory(ctx context.Context, projectID string) []anthropicMessage {
	if o.historyRepo == nil {
		return nil
	}
	items, err := o.historyRepo.ListByProject(ctx, projectID, o.maxHistory)
	if err != nil {
		return nil
	}
	messages := make([]anthropicMessage, 0, len(items))
	for _, item := range items {
		role := strings.TrimSpace(item.Role)
		if role != "assistant" {
			role = "user"
		}
		messages = append(messages, anthropicMessage{
			Role:    role,
			Content: []anthropicContentBlock{{Type: "text", Text: item.Content}},
		})
	}
	return messages
}

func (o *Orchestrator) loadWorkflowAsPlaybook(ctx context.Context, projectID string) *Playbook {
	if o.projectOrchestratorRepo == nil || o.workflowRepo == nil {
		return nil
	}
	profile, err := o.projectOrchestratorRepo.Get(ctx, projectID)
	if err != nil || profile == nil {
		return nil
	}
	workflow, err := o.workflowRepo.Get(ctx, profile.WorkflowID)
	if err != nil || workflow == nil {
		return nil
	}
	playbookWorkflow := PlaybookWorkflow{
		Plan:  PlaybookStage{Enabled: false, Roles: []PlaybookRole{}},
		Build: PlaybookStage{Enabled: false, Roles: []PlaybookRole{}},
		Test:  PlaybookStage{Enabled: false, Roles: []PlaybookRole{}},
	}
	for _, p := range workflow.Phases {
		if p == nil {
			continue
		}
		role := PlaybookRole{
			Name:             strings.TrimSpace(p.Role),
			Responsibilities: fmt.Sprintf("entry=%s | exit=%s | max_parallel=%d", p.EntryRule, p.ExitRule, p.MaxParallel),
			AllowedAgents:    []string{},
		}
		phaseType := strings.ToLower(strings.TrimSpace(p.PhaseType))
		switch {
		case strings.Contains(phaseType, "scan"), strings.Contains(phaseType, "plan"):
			playbookWorkflow.Plan.Enabled = true
			playbookWorkflow.Plan.Roles = append(playbookWorkflow.Plan.Roles, role)
		case strings.Contains(phaseType, "review"), strings.Contains(phaseType, "test"), strings.Contains(phaseType, "qa"):
			playbookWorkflow.Test.Enabled = true
			playbookWorkflow.Test.Roles = append(playbookWorkflow.Test.Roles, role)
		default:
			playbookWorkflow.Build.Enabled = true
			playbookWorkflow.Build.Roles = append(playbookWorkflow.Build.Roles, role)
		}
	}
	return &Playbook{
		ID:       workflow.ID,
		Name:     workflow.Name,
		Workflow: playbookWorkflow,
		Strategy: fmt.Sprintf("project_max_parallel=%d, review_policy=%s", profile.MaxParallel, profile.ReviewPolicy),
	}
}

func (o *Orchestrator) loadProjectPlaybook(ctx context.Context, project *db.Project) *Playbook {
	if o == nil || o.playbookRegistry == nil || project == nil {
		return nil
	}

	var pb *playbook.Playbook
	if overrideID := strings.TrimSpace(project.Playbook); overrideID != "" {
		pb = o.playbookRegistry.Get(overrideID)
	}
	if pb == nil {
		pb = o.playbookRegistry.MatchProject(project.RepoPath)
	}
	if pb == nil {
		return nil
	}

	return &Playbook{
		ID:   pb.ID,
		Name: pb.Name,
		Workflow: PlaybookWorkflow{
			Plan: PlaybookStage{
				Enabled: pb.Workflow.Plan.Enabled,
				Roles:   toPromptRoles(pb.Workflow.Plan.Roles),
			},
			Build: PlaybookStage{
				Enabled: pb.Workflow.Build.Enabled,
				Roles:   toPromptRoles(pb.Workflow.Build.Roles),
			},
			Test: PlaybookStage{
				Enabled: pb.Workflow.Test.Enabled,
				Roles:   toPromptRoles(pb.Workflow.Test.Roles),
			},
		},
	}
}

func toPromptRoles(roles []playbook.StageRole) []PlaybookRole {
	if len(roles) == 0 {
		return []PlaybookRole{}
	}
	out := make([]PlaybookRole, 0, len(roles))
	for _, role := range roles {
		out = append(out, PlaybookRole{
			Name:             strings.TrimSpace(role.Name),
			Mode:             strings.TrimSpace(role.Mode),
			Responsibilities: strings.TrimSpace(role.Responsibilities),
			AllowedAgents:    append([]string(nil), role.AllowedAgents...),
			InputsRequired:   append([]string(nil), role.InputsRequired...),
			ActionsAllowed:   append([]string(nil), role.ActionsAllowed...),
			SuggestedPrompt:  strings.TrimSpace(role.SuggestedPrompt),
		})
	}
	return out
}

func (o *Orchestrator) resolveLLMConfig(ctx context.Context, projectID string, agents []*registry.AgentConfig) (llmConfig, error) {
	provider := "anthropic"
	model := strings.TrimSpace(o.model)
	if model == "" {
		model = defaultModel
	}

	if o.projectOrchestratorRepo != nil && strings.TrimSpace(projectID) != "" {
		if profile, err := o.projectOrchestratorRepo.Get(ctx, projectID); err == nil && profile != nil {
			if p := strings.ToLower(strings.TrimSpace(profile.DefaultProvider)); p != "" {
				provider = p
			}
			if m := strings.TrimSpace(profile.DefaultModel); m != "" {
				model = m
			}
		}
	}

	candidates := make([]*registry.AgentConfig, 0)
	for _, agent := range agents {
		if agent != nil && agent.SupportsOrchestrator {
			candidates = append(candidates, agent)
		}
	}

	pick := func(list []*registry.AgentConfig) *registry.AgentConfig {
		for _, item := range list {
			if strings.EqualFold(strings.TrimSpace(item.Model), model) {
				return item
			}
		}
		if len(list) > 0 {
			return list[0]
		}
		return nil
	}

	filtered := make([]*registry.AgentConfig, 0, len(candidates))
	for _, item := range candidates {
		if p := strings.ToLower(strings.TrimSpace(item.OrchestratorProvider)); p == "" || p == provider {
			filtered = append(filtered, item)
		}
	}
	selected := pick(filtered)
	if selected == nil {
		selected = pick(candidates)
	}

	cfg := llmConfig{
		Provider: provider,
		Model:    model,
		APIKey:   strings.TrimSpace(o.apiKey),
		BaseURL:  strings.TrimSpace(o.anthropicBaseURL),
	}

	if selected != nil {
		if p := strings.ToLower(strings.TrimSpace(selected.OrchestratorProvider)); p != "" {
			cfg.Provider = p
		}
		if m := strings.TrimSpace(selected.Model); m != "" {
			cfg.Model = m
		}
		cfg.APIKey = strings.TrimSpace(selected.OrchestratorAPIKey)
		cfg.BaseURL = strings.TrimSpace(selected.OrchestratorAPIBase)
	}

	if cfg.Provider == "" {
		cfg.Provider = "anthropic"
	}
	if cfg.Provider == "openai" && strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = defaultOpenAIModel
	}
	if cfg.Provider != "openai" && strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = defaultModel
	}
	if cfg.BaseURL == "" {
		if cfg.Provider == "openai" {
			cfg.BaseURL = defaultOpenAIURL
		} else {
			cfg.BaseURL = defaultAnthropicURL
		}
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return llmConfig{}, fmt.Errorf("orchestrator credentials are not configured for provider %s", cfg.Provider)
	}
	return cfg, nil
}

func toJSON(v any) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err.Error())
	}
	return string(buf)
}

func (o *Orchestrator) executeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if o == nil || o.toolset == nil {
		return nil, fmt.Errorf("orchestrator tools unavailable")
	}
	if err := o.enforceRoleContractForTool(ctx, name, args); err != nil {
		return nil, err
	}
	if requiresExplicitSessionID(name) {
		if _, err := requiredString(args, "session_id"); err != nil {
			return nil, fmt.Errorf("%s requires explicit session_id: %w", name, err)
		}
	}
	if name == "send_command" {
		return o.executeQueuedSendCommand(ctx, args)
	}
	return o.toolset.Execute(ctx, name, args)
}

func requiresExplicitSessionID(name string) bool {
	switch strings.TrimSpace(name) {
	case "send_command", "read_session_output", "is_session_idle", "close_session", "can_close_session":
		return true
	default:
		return false
	}
}

func (o *Orchestrator) enforceRoleContractForTool(ctx context.Context, toolName string, args map[string]any) error {
	if o == nil || o.playbookRegistry == nil || o.projectRepo == nil || o.taskRepo == nil || o.sessionRepo == nil {
		return nil
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return nil
	}

	switch toolName {
	case "create_session":
		taskID, err := requiredString(args, "task_id")
		if err != nil {
			return nil
		}
		roleName, err := requiredString(args, "role")
		if err != nil {
			return nil
		}
		agentType, _ := optionalString(args, "agent_type")
		task, err := o.taskRepo.Get(ctx, taskID)
		if err != nil || task == nil {
			return nil
		}
		pb, err := o.loadProjectPlaybookDefinition(ctx, task.ProjectID)
		if err != nil || pb == nil {
			return nil
		}
		stage, role := findPlaybookRole(pb, roleName)
		if role == nil {
			return fmt.Errorf("role %q is not defined in playbook %q", roleName, pb.ID)
		}
		if strings.TrimSpace(agentType) != "" && len(role.AllowedAgents) > 0 && !containsFold(role.AllowedAgents, agentType) {
			return fmt.Errorf("agent %q is not allowed for role %q in stage %s", agentType, roleName, stage)
		}
		if !toolAllowedByRole(toolName, *role) {
			return fmt.Errorf("tool %q is not allowed for role %q (mode=%s)", toolName, roleName, role.Mode)
		}
		if missing := missingRoleInputs(*role, task, nil, args); len(missing) > 0 {
			return fmt.Errorf("missing required role inputs for %q: %s", roleName, strings.Join(missing, ", "))
		}
		return nil
	case "send_command", "read_session_output", "is_session_idle", "close_session", "can_close_session":
		sessionID, err := requiredString(args, "session_id")
		if err != nil {
			return nil
		}
		sess, err := o.sessionRepo.Get(ctx, sessionID)
		if err != nil || sess == nil {
			return nil
		}
		task, err := o.taskRepo.Get(ctx, sess.TaskID)
		if err != nil || task == nil {
			return nil
		}
		pb, err := o.loadProjectPlaybookDefinition(ctx, task.ProjectID)
		if err != nil || pb == nil {
			return nil
		}
		_, role := findPlaybookRole(pb, sess.Role)
		if role == nil {
			return nil
		}
		if !toolAllowedByRole(toolName, *role) {
			return fmt.Errorf("tool %q is not allowed for role %q (mode=%s)", toolName, sess.Role, role.Mode)
		}
		if missing := missingRoleInputs(*role, task, sess, args); len(missing) > 0 {
			return fmt.Errorf("missing required role inputs for %q: %s", sess.Role, strings.Join(missing, ", "))
		}
	}
	return nil
}

func (o *Orchestrator) loadProjectPlaybookDefinition(ctx context.Context, projectID string) (*playbook.Playbook, error) {
	if o == nil || o.projectRepo == nil || o.playbookRegistry == nil || strings.TrimSpace(projectID) == "" {
		return nil, nil
	}
	project, err := o.projectRepo.Get(ctx, projectID)
	if err != nil || project == nil {
		return nil, err
	}
	if overrideID := strings.TrimSpace(project.Playbook); overrideID != "" {
		if pb := o.playbookRegistry.Get(overrideID); pb != nil {
			return pb, nil
		}
	}
	return o.playbookRegistry.MatchProject(project.RepoPath), nil
}

func findPlaybookRole(pb *playbook.Playbook, roleName string) (string, *playbook.StageRole) {
	if pb == nil || strings.TrimSpace(roleName) == "" {
		return "", nil
	}
	stages := []struct {
		name  string
		stage playbook.Stage
	}{
		{name: "plan", stage: pb.Workflow.Plan},
		{name: "build", stage: pb.Workflow.Build},
		{name: "test", stage: pb.Workflow.Test},
	}
	for _, s := range stages {
		for i := range s.stage.Roles {
			if strings.EqualFold(strings.TrimSpace(s.stage.Roles[i].Name), strings.TrimSpace(roleName)) {
				return s.name, &s.stage.Roles[i]
			}
		}
	}
	return "", nil
}

func containsFold(values []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return true
		}
	}
	return false
}

func toolAllowedByRole(toolName string, role playbook.StageRole) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return true
	}
	if len(role.ActionsAllowed) > 0 {
		return containsFold(role.ActionsAllowed, toolName)
	}
	mode := strings.ToLower(strings.TrimSpace(role.Mode))
	switch mode {
	case "planner":
		return containsFold([]string{
			"create_task", "create_worktree", "write_task_spec", "create_session",
			"read_session_output", "is_session_idle", "get_project_status", "generate_progress_report",
		}, toolName)
	case "reviewer":
		return containsFold([]string{
			"create_session", "send_command", "read_session_output", "is_session_idle",
			"generate_progress_report", "can_close_session",
		}, toolName)
	case "tester":
		return containsFold([]string{
			"create_session", "send_command", "read_session_output", "is_session_idle",
			"generate_progress_report", "can_close_session", "close_session",
		}, toolName)
	default:
		return containsFold([]string{
			"create_session", "send_command", "read_session_output", "is_session_idle",
			"write_task_spec", "can_close_session", "close_session", "resolve_merge_conflict",
		}, toolName)
	}
}

func missingRoleInputs(role playbook.StageRole, task *db.Task, session *db.Session, args map[string]any) []string {
	if len(role.InputsRequired) == 0 {
		return nil
	}
	available := map[string]struct{}{}
	for key, value := range args {
		if value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				available[strings.ToLower(strings.TrimSpace(key))] = struct{}{}
			}
		default:
			available[strings.ToLower(strings.TrimSpace(key))] = struct{}{}
		}
	}
	if task != nil {
		if strings.TrimSpace(task.ID) != "" {
			available["task_id"] = struct{}{}
		}
		if strings.TrimSpace(task.ProjectID) != "" {
			available["project_id"] = struct{}{}
		}
		if strings.TrimSpace(task.WorktreeID) != "" {
			available["worktree_id"] = struct{}{}
		}
		if strings.TrimSpace(task.SpecPath) != "" {
			available["spec_path"] = struct{}{}
		}
	}
	if session != nil {
		if strings.TrimSpace(session.ID) != "" {
			available["session_id"] = struct{}{}
		}
		if strings.TrimSpace(session.AgentType) != "" {
			available["agent_type"] = struct{}{}
		}
		if strings.TrimSpace(session.Role) != "" {
			available["role"] = struct{}{}
		}
	}

	missing := make([]string, 0)
	for _, input := range role.InputsRequired {
		key := strings.ToLower(strings.TrimSpace(input))
		if key == "" {
			continue
		}
		if _, ok := available[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func (o *Orchestrator) executeQueuedSendCommand(ctx context.Context, args map[string]any) (any, error) {
	sessionID, err := requiredString(args, "session_id")
	if err != nil {
		return nil, err
	}
	commandText, _ := optionalString(args, "text")

	entryID := o.appendCommandLedgerEntry(CommandLedgerEntry{
		ToolName:  "send_command",
		SessionID: sessionID,
		Command:   commandText,
		IssuedAt:  time.Now().UTC(),
		Status:    "queued",
	})

	lock := o.getSessionCommandLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	o.updateCommandLedgerEntry(entryID, func(entry *CommandLedgerEntry) {
		entry.Status = "running"
		entry.StartedAt = time.Now().UTC()
	})

	result, execErr := o.toolset.Execute(ctx, "send_command", args)
	completedAt := time.Now().UTC()
	if execErr != nil {
		o.updateCommandLedgerEntry(entryID, func(entry *CommandLedgerEntry) {
			entry.Status = "failed"
			entry.CompletedAt = completedAt
			entry.Error = execErr.Error()
		})
		return nil, execErr
	}

	o.updateCommandLedgerEntry(entryID, func(entry *CommandLedgerEntry) {
		entry.Status = "succeeded"
		entry.CompletedAt = completedAt
		entry.ResultSnippet = truncate(strings.TrimSpace(toJSON(result)), 220)
	})
	return result, nil
}

func (o *Orchestrator) getSessionCommandLock(sessionID string) *sync.Mutex {
	o.commandMu.Lock()
	defer o.commandMu.Unlock()
	lock, ok := o.sessionCommandLock[sessionID]
	if ok {
		return lock
	}
	lock = &sync.Mutex{}
	o.sessionCommandLock[sessionID] = lock
	return lock
}

func (o *Orchestrator) appendCommandLedgerEntry(entry CommandLedgerEntry) int64 {
	o.commandMu.Lock()
	defer o.commandMu.Unlock()
	o.nextCommandID++
	entry.ID = o.nextCommandID
	o.commandLedger = append(o.commandLedger, entry)
	if len(o.commandLedger) > maxCommandLedgerEntries {
		o.commandLedger = o.commandLedger[len(o.commandLedger)-maxCommandLedgerEntries:]
	}
	return entry.ID
}

func (o *Orchestrator) updateCommandLedgerEntry(id int64, mutate func(entry *CommandLedgerEntry)) {
	if id <= 0 || mutate == nil {
		return
	}
	o.commandMu.Lock()
	defer o.commandMu.Unlock()
	for i := len(o.commandLedger) - 1; i >= 0; i-- {
		if o.commandLedger[i].ID == id {
			mutate(&o.commandLedger[i])
			return
		}
	}
}

func truncate(v string, max int) string {
	v = strings.TrimSpace(v)
	if max <= 0 || len(v) <= max {
		return v
	}
	if max <= 3 {
		return v[:max]
	}
	return v[:max-3] + "..."
}

type anthropicRequest struct {
	Model     string                 `json:"model"`
	System    string                 `json:"system,omitempty"`
	MaxTokens int                    `json:"max_tokens"`
	Tools     []map[string]any       `json:"tools,omitempty"`
	Messages  []anthropicMessage     `json:"messages"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

type openAIRequest struct {
	Model      string          `json:"model"`
	Messages   []openAIMessage `json:"messages"`
	Tools      []openAITool    `json:"tools,omitempty"`
	ToolChoice string          `json:"tool_choice,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

func (o *Orchestrator) createMessage(ctx context.Context, req anthropicRequest, cfg llmConfig) (*anthropicResponse, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "openai" {
		return o.createOpenAIMessage(ctx, req, cfg)
	}
	return o.createAnthropicMessage(ctx, req, cfg)
}

func (o *Orchestrator) createAnthropicMessage(ctx context.Context, req anthropicRequest, cfg llmConfig) (*anthropicResponse, error) {
	buf, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cfg.APIKey) != "" {
		httpReq.Header.Set("x-api-key", cfg.APIKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("anthropic api status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out anthropicResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (o *Orchestrator) createOpenAIMessage(ctx context.Context, req anthropicRequest, cfg llmConfig) (*anthropicResponse, error) {
	openReq := openAIRequest{
		Model:      cfg.Model,
		Messages:   toOpenAIMessages(req),
		Tools:      toOpenAITools(req.Tools),
		ToolChoice: "auto",
	}

	buf, err := json.Marshal(openReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cfg.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("openai api status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out openAIResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return &anthropicResponse{}, nil
	}

	msg := out.Choices[0].Message
	blocks := make([]anthropicContentBlock, 0, 1+len(msg.ToolCalls))
	if strings.TrimSpace(msg.Content) != "" {
		blocks = append(blocks, anthropicContentBlock{
			Type: "text",
			Text: msg.Content,
		})
	}
	for _, call := range msg.ToolCalls {
		input := map[string]any{}
		raw := strings.TrimSpace(call.Function.Arguments)
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &input)
		}
		blocks = append(blocks, anthropicContentBlock{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: input,
		})
	}
	return &anthropicResponse{Content: blocks}, nil
}

func toOpenAITools(input []map[string]any) []openAITool {
	tools := make([]openAITool, 0, len(input))
	for _, raw := range input {
		name, _ := raw["name"].(string)
		description, _ := raw["description"].(string)
		schema, _ := raw["input_schema"].(map[string]any)
		if strings.TrimSpace(name) == "" {
			continue
		}
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        name,
				Description: description,
				Parameters:  schema,
			},
		})
	}
	return tools
}

func toOpenAIMessages(req anthropicRequest) []openAIMessage {
	out := make([]openAIMessage, 0, len(req.Messages)+2)
	if strings.TrimSpace(req.System) != "" {
		out = append(out, openAIMessage{Role: "system", Content: req.System})
	}

	for _, msg := range req.Messages {
		textParts := make([]string, 0, 2)
		toolUses := make([]openAIToolCall, 0, 1)
		toolResults := make([]openAIMessage, 0, 1)

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if strings.TrimSpace(block.Text) != "" {
					textParts = append(textParts, block.Text)
				}
			case "tool_use":
				args, _ := json.Marshal(block.Input)
				toolUses = append(toolUses, openAIToolCall{
					ID:   block.ID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      block.Name,
						Arguments: string(args),
					},
				})
			case "tool_result":
				toolResults = append(toolResults, openAIMessage{
					Role:       "tool",
					ToolCallID: block.ToolUseID,
					Content:    block.Content,
				})
			}
		}

		content := strings.Join(textParts, "\n")
		if msg.Role == "assistant" {
			out = append(out, openAIMessage{
				Role:      "assistant",
				Content:   content,
				ToolCalls: toolUses,
			})
			continue
		}

		if strings.TrimSpace(content) != "" {
			out = append(out, openAIMessage{Role: "user", Content: content})
		}
		out = append(out, toolResults...)
	}

	return out
}
