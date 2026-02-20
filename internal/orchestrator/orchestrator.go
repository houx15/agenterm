package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	defaultMaxTokens         = 4096
	defaultMaxToolRounds     = 10
	defaultMaxIdlePollRounds = 240
	defaultMaxHistory        = 50
	defaultGlobalMaxParallel = 32
	maxCommandLedgerEntries  = 500
	executionLane            = "execution"
	demandLane               = "demand"
)

const (
	stagePlan  = "plan"
	stageBuild = "build"
	stageTest  = "test"
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
	RoleLoopAttemptRepo     *db.RoleLoopAttemptRepo
	Registry                *registry.Registry
	PlaybookRegistry        *playbook.Registry
	Toolset                 *Toolset

	MaxToolRounds     int
	MaxHistory        int
	GlobalMaxParallel int
	Lane              string
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
	roleLoopAttemptRepo     *db.RoleLoopAttemptRepo
	registry                *registry.Registry
	playbookRegistry        *playbook.Registry
	toolset                 *Toolset
	lane                    string

	maxToolRounds     int
	maxHistory        int
	globalMaxParallel int

	commandMu          sync.Mutex
	sessionCommandLock map[string]*sync.Mutex
	commandLedger      []CommandLedgerEntry
	nextCommandID      int64

	roleLoopMu       sync.Mutex
	roleLoopAttempts map[string]map[string]int
	roleLoopLoaded   map[string]bool

	projectChatMu    sync.Mutex
	projectChatLocks map[string]*sync.Mutex
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
	lane := strings.ToLower(strings.TrimSpace(opts.Lane))
	if lane == "" {
		lane = executionLane
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
		roleLoopAttemptRepo:     opts.RoleLoopAttemptRepo,
		registry:                opts.Registry,
		playbookRegistry:        opts.PlaybookRegistry,
		toolset:                 toolset,
		lane:                    lane,
		maxToolRounds:           maxRounds,
		maxHistory:              maxHistory,
		globalMaxParallel:       globalMaxParallel,
		sessionCommandLock:      make(map[string]*sync.Mutex),
		commandLedger:           make([]CommandLedgerEntry, 0, 64),
		roleLoopAttempts:        make(map[string]map[string]int),
		roleLoopLoaded:          make(map[string]bool),
		projectChatLocks:        make(map[string]*sync.Mutex),
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
	projectID = strings.TrimSpace(projectID)
	unlock := o.lockProjectChat(projectID)

	state, err := o.loadProjectState(ctx, projectID)
	if err != nil {
		unlock()
		return nil, err
	}
	agents := []*registry.AgentConfig{}
	if o.registry != nil {
		agents = o.registry.List()
	}
	llmCfg, err := o.resolveLLMConfig(ctx, projectID, agents)
	if err != nil {
		unlock()
		return nil, err
	}
	approval := evaluateApprovalGate(userMessage)
	var systemPrompt string
	if o.lane == demandLane {
		systemPrompt = BuildDemandSystemPrompt(state, agents)
	} else {
		matchedPlaybook := o.loadProjectPlaybook(ctx, state.Project)
		if matchedPlaybook == nil {
			matchedPlaybook = o.loadWorkflowAsPlaybook(ctx, projectID)
		}
		activeStage := deriveExecutionStage(state, matchedPlaybook)
		systemPrompt = BuildSystemPrompt(state, agents, matchedPlaybook, activeStage)
	}
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
	userMsg := anthropicMessage{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: userMessage}}}
	messages := append(history, userMsg)

	if o.historyRepo != nil {
		_ = o.persistHistoryMessage(ctx, projectID, userMsg)
	}

	ch := make(chan StreamEvent, 32)
	go func() {
		defer unlock()
		defer close(ch)
		actionRounds := 0
		idlePollRounds := 0
		sessionActionRounds := make(map[string]int)

		for {
			if actionRounds >= o.maxToolRounds {
				ch <- StreamEvent{Type: "error", Error: "max action rounds reached"}
				return
			}
			if idlePollRounds >= defaultMaxIdlePollRounds {
				ch <- StreamEvent{Type: "error", Error: "max idle poll rounds reached"}
				return
			}
			resp, err := o.createMessage(ctx, anthropicRequest{
				Model:     llmCfg.Model,
				MaxTokens: defaultMaxTokens,
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
			if o.historyRepo != nil {
				_ = o.persistHistoryMessage(ctx, projectID, assistantMsg)
			}

			toolUsed := false
			actionUsed := false
			idleOnlyRound := true
			for _, block := range resp.Content {
				switch block.Type {
				case "text":
					text := strings.TrimSpace(block.Text)
					if text != "" {
						ch <- StreamEvent{Type: "token", Text: text}
					}
				case "tool_use":
					toolUsed = true
					if strings.TrimSpace(block.Name) != "is_session_idle" {
						actionUsed = true
						idleOnlyRound = false
					}
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
						if o.historyRepo != nil {
							_ = o.persistHistoryMessage(ctx, projectID, anthropicMessage{
								Role: "user",
								Content: []anthropicContentBlock{{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   toJSON(result),
								}},
							})
						}
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
							if o.historyRepo != nil {
								_ = o.persistHistoryMessage(ctx, projectID, anthropicMessage{
									Role: "user",
									Content: []anthropicContentBlock{{
										Type:      "tool_result",
										ToolUseID: block.ID,
										Content:   toJSON(result),
									}},
								})
							}
							continue
						}
					}
					if strings.TrimSpace(block.Name) == "send_command" {
						sessionID, _ := optionalString(block.Input, "session_id")
						sessionID = strings.TrimSpace(sessionID)
						if sessionID != "" {
							sessionActionRounds[sessionID] = sessionActionRounds[sessionID] + 1
							if sessionActionRounds[sessionID] > o.maxToolRounds {
								result := map[string]any{
									"error":  "session_round_limit_reached",
									"reason": fmt.Sprintf("session %s reached max action rounds (%d) for this turn", sessionID, o.maxToolRounds),
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
					if o.historyRepo != nil {
						_ = o.persistHistoryMessage(ctx, projectID, anthropicMessage{
							Role: "user",
							Content: []anthropicContentBlock{{
								Type:      "tool_result",
								ToolUseID: block.ID,
								Content:   resultJSON,
							}},
						})
					}
				}
			}

			if !toolUsed {
				if o.lane == executionLane && approval.Confirmed && requiresExecutionToolUsage(userMessage) {
					ch <- StreamEvent{
						Type:  "error",
						Error: "execution_requires_tool_calls: orchestrator must create/use sessions and tools instead of claiming direct execution",
					}
					return
				}
				if o.historyRepo != nil {
					_ = o.historyRepo.TrimProjectAndRoles(ctx, projectID, o.maxHistory, o.storageRoles())
				}
				ch <- StreamEvent{Type: "done"}
				return
			}
			if actionUsed {
				actionRounds++
			}
			if idleOnlyRound {
				idlePollRounds++
			}
		}
	}()

	return ch, nil
}

func (o *Orchestrator) lockProjectChat(projectID string) func() {
	if o == nil {
		return func() {}
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return func() {}
	}
	o.projectChatMu.Lock()
	lock := o.projectChatLocks[projectID]
	if lock == nil {
		lock = &sync.Mutex{}
		o.projectChatLocks[projectID] = lock
	}
	o.projectChatMu.Unlock()
	lock.Lock()
	return func() {
		lock.Unlock()
	}
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

func requiresExecutionToolUsage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	keywords := []string{
		"implement", "fix", "write code", "code this", "build this", "start building",
		"run tests", "test this", "review code", "open session", "start session",
		"create worktree", "dispatch", "execute", "go ahead", "proceed with build",
		"send command", "apply changes", "commit", "merge",
	}
	for _, k := range keywords {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

func isMutatingTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "create_project", "create_task", "create_worktree", "merge_worktree", "resolve_merge_conflict",
		"create_session", "send_command", "close_session", "write_task_spec",
		"create_demand_item", "update_demand_item", "reprioritize_demand_pool", "promote_demand_item":
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
	if roleLoop, err := o.BuildRoleLoopState(ctx, projectID); err == nil {
		report["role_loop_state"] = roleLoop
	}
	return report, nil
}

func (o *Orchestrator) BuildRoleLoopState(ctx context.Context, projectID string) (map[string]any, error) {
	if o == nil || o.taskRepo == nil {
		return map[string]any{"tasks": []any{}}, nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return map[string]any{"tasks": []any{}}, nil
	}
	tasks, err := o.taskRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	playbookDef, _ := o.loadProjectPlaybookDefinition(ctx, projectID)
	stageRoles := map[string]playbook.StageRole{}
	if playbookDef != nil {
		for _, stage := range []playbook.Stage{playbookDef.Workflow.Plan, playbookDef.Workflow.Build, playbookDef.Workflow.Test} {
			for _, role := range stage.Roles {
				stageRoles[strings.ToLower(strings.TrimSpace(role.Name))] = role
			}
		}
	}
	taskStates := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		perTaskAttempts := o.roleAttemptsForTask(ctx, task.ID)
		roles := make([]map[string]any, 0, len(perTaskAttempts))
		escalations := make([]string, 0)
		for roleNameKey, count := range perTaskAttempts {
			roleDef, ok := stageRoles[roleNameKey]
			maxIterations := 0
			handoffTo := []string{}
			if ok {
				maxIterations = roleDef.RetryPolicy.MaxIterations
				handoffTo = append([]string(nil), roleDef.HandoffTo...)
			}
			remaining := -1
			exhausted := false
			if maxIterations > 0 {
				remaining = maxIterations - count
				exhausted = count >= maxIterations
				if exhausted {
					escalations = append(escalations, strings.TrimSpace(roleDef.Name))
				}
			}
			roles = append(roles, map[string]any{
				"role":           roleNameKey,
				"attempts":       count,
				"max_iterations": maxIterations,
				"remaining":      remaining,
				"exhausted":      exhausted,
				"handoff_to":     handoffTo,
			})
		}
		taskStates = append(taskStates, map[string]any{
			"task_id":       task.ID,
			"task_title":    task.Title,
			"roles":         roles,
			"escalations":   compactNonEmptyStrings(escalations),
			"tracked_roles": len(roles),
		})
	}
	return map[string]any{
		"tasks": taskStates,
	}, nil
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
	items, err := o.historyRepo.ListByProjectAndRoles(ctx, projectID, o.maxHistory, o.storageRoles())
	if err != nil {
		return nil
	}
	messages := make([]anthropicMessage, 0, len(items))
	for _, item := range items {
		role, ok := o.normalizeHistoryRole(item.Role)
		if !ok {
			continue
		}
		content := parseStoredMessageContent(item.MessageJSON, item.Content)
		messages = append(messages, anthropicMessage{
			Role:    role,
			Content: content,
		})
	}
	return messages
}

func parseStoredMessageContent(rawJSON string, fallbackText string) []anthropicContentBlock {
	rawJSON = strings.TrimSpace(rawJSON)
	if rawJSON != "" {
		var blocks []anthropicContentBlock
		if err := json.Unmarshal([]byte(rawJSON), &blocks); err == nil && len(blocks) > 0 {
			return blocks
		}
	}
	if strings.TrimSpace(fallbackText) == "" {
		return []anthropicContentBlock{{Type: "text", Text: "[empty]"}}
	}
	return []anthropicContentBlock{{Type: "text", Text: fallbackText}}
}

func (o *Orchestrator) persistHistoryMessage(ctx context.Context, projectID string, msg anthropicMessage) error {
	if o == nil || o.historyRepo == nil || strings.TrimSpace(projectID) == "" {
		return nil
	}
	raw, err := json.Marshal(msg.Content)
	if err != nil {
		return err
	}
	return o.historyRepo.Create(ctx, &db.OrchestratorMessage{
		ProjectID:   projectID,
		Role:        o.storageRole(msg.Role),
		Content:     summarizeHistoryContent(msg.Content),
		MessageJSON: string(raw),
	})
}

func summarizeHistoryContent(blocks []anthropicContentBlock) string {
	texts := make([]string, 0, 2)
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if text := strings.TrimSpace(block.Text); text != "" {
				texts = append(texts, text)
			}
		case "tool_use":
			name := strings.TrimSpace(block.Name)
			if name == "" {
				name = "unknown"
			}
			texts = append(texts, "[tool_use:"+name+"]")
		case "tool_result":
			snippet := truncate(strings.TrimSpace(block.Content), 120)
			if snippet == "" {
				snippet = "ok"
			}
			texts = append(texts, "[tool_result:"+snippet+"]")
		}
	}
	if len(texts) == 0 {
		return "[structured message]"
	}
	return strings.Join(texts, "\n")
}

func (o *Orchestrator) ListHistory(ctx context.Context, projectID string, limit int) ([]*db.OrchestratorMessage, error) {
	if o == nil || o.historyRepo == nil {
		return nil, fmt.Errorf("orchestrator history unavailable")
	}
	items, err := o.historyRepo.ListByProjectAndRoles(ctx, projectID, limit, o.storageRoles())
	if err != nil {
		return nil, err
	}
	out := make([]*db.OrchestratorMessage, 0, len(items))
	for _, item := range items {
		role, ok := o.normalizeHistoryRole(item.Role)
		if !ok {
			continue
		}
		cloned := *item
		cloned.Role = role
		out = append(out, &cloned)
	}
	return out, nil
}

func (o *Orchestrator) storageRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return "user"
	}
	if o != nil && o.lane == demandLane {
		return demandLane + "_" + role
	}
	return role
}

func (o *Orchestrator) storageRoles() []string {
	if o != nil && o.lane == demandLane {
		return []string{"demand_user", "demand_assistant"}
	}
	return []string{"user", "assistant", "execution_user", "execution_assistant"}
}

func (o *Orchestrator) normalizeHistoryRole(stored string) (string, bool) {
	stored = strings.ToLower(strings.TrimSpace(stored))
	switch stored {
	case "assistant", "execution_assistant", "demand_assistant":
		return "assistant", true
	case "user", "execution_user", "demand_user":
		return "user", true
	default:
		return "", false
	}
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
	if err := o.enforceStageToolGate(ctx, name, args); err != nil {
		return nil, err
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
	result, err := o.toolset.Execute(ctx, name, args)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "create_session" {
		taskID, _ := optionalString(args, "task_id")
		roleName, _ := optionalString(args, "role")
		o.incrementRoleAttempt(ctx, taskID, roleName)
	}
	return result, nil
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
		if blocked, reason := o.checkRoleHandoffAndRetries(ctx, taskID, *pb, roleName); blocked {
			return fmt.Errorf("role loop blocked for %q: %s", roleName, reason)
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

func (o *Orchestrator) checkRoleHandoffAndRetries(ctx context.Context, taskID string, pb playbook.Playbook, roleName string) (bool, string) {
	taskID = strings.TrimSpace(taskID)
	roleName = strings.TrimSpace(roleName)
	if o == nil || taskID == "" || roleName == "" {
		return false, ""
	}
	stageName, role := findPlaybookRole(&pb, roleName)
	if role == nil {
		return true, "role is not in playbook"
	}
	attempts := o.roleAttemptCount(ctx, taskID, roleName)
	if role.RetryPolicy.MaxIterations > 0 && attempts >= role.RetryPolicy.MaxIterations {
		reason := fmt.Sprintf("max_iterations reached (%d/%d)", attempts, role.RetryPolicy.MaxIterations)
		if len(role.RetryPolicy.EscalateOn) > 0 {
			reason += fmt.Sprintf("; escalate_on=%s", strings.Join(role.RetryPolicy.EscalateOn, ","))
		}
		return true, reason
	}

	predecessors := predecessorRoles(pb, roleName)
	if len(predecessors) == 0 {
		return false, ""
	}
	for _, predecessor := range predecessors {
		if o.roleAttemptCount(ctx, taskID, predecessor) > 0 {
			return false, ""
		}
	}
	return true, fmt.Sprintf("handoff not ready in stage %s; requires one of: %s", stageName, strings.Join(predecessors, ", "))
}

func predecessorRoles(pb playbook.Playbook, roleName string) []string {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return nil
	}
	stages := []playbook.Stage{pb.Workflow.Plan, pb.Workflow.Build, pb.Workflow.Test}
	predecessors := make([]string, 0)
	for _, stage := range stages {
		for _, role := range stage.Roles {
			if containsFold(role.HandoffTo, roleName) {
				predecessors = append(predecessors, strings.TrimSpace(role.Name))
			}
		}
	}
	return compactNonEmptyStrings(predecessors)
}

func compactNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (o *Orchestrator) roleAttemptCount(ctx context.Context, taskID string, roleName string) int {
	taskID = strings.TrimSpace(taskID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if o == nil || taskID == "" || roleName == "" {
		return 0
	}
	o.hydrateRoleAttemptsFromRepo(ctx, taskID)
	o.roleLoopMu.Lock()
	defer o.roleLoopMu.Unlock()
	perTask := o.roleLoopAttempts[taskID]
	if perTask == nil {
		return 0
	}
	return perTask[roleName]
}

func (o *Orchestrator) incrementRoleAttempt(ctx context.Context, taskID string, roleName string) {
	taskID = strings.TrimSpace(taskID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if o == nil || taskID == "" || roleName == "" {
		return
	}
	o.hydrateRoleAttemptsFromRepo(ctx, taskID)
	persistedCount := 0
	if o.roleLoopAttemptRepo != nil {
		if next, err := o.roleLoopAttemptRepo.Increment(ctx, taskID, roleName); err == nil {
			persistedCount = next
		}
	}
	o.roleLoopMu.Lock()
	defer o.roleLoopMu.Unlock()
	perTask := o.roleLoopAttempts[taskID]
	if perTask == nil {
		perTask = make(map[string]int)
		o.roleLoopAttempts[taskID] = perTask
	}
	if persistedCount > 0 {
		perTask[roleName] = persistedCount
		o.roleLoopLoaded[taskID] = true
		return
	}
	perTask[roleName] = perTask[roleName] + 1
}

func (o *Orchestrator) roleAttemptsForTask(ctx context.Context, taskID string) map[string]int {
	taskID = strings.TrimSpace(taskID)
	if o == nil || taskID == "" {
		return map[string]int{}
	}
	o.hydrateRoleAttemptsFromRepo(ctx, taskID)
	o.roleLoopMu.Lock()
	defer o.roleLoopMu.Unlock()
	perTask := o.roleLoopAttempts[taskID]
	if perTask == nil {
		return map[string]int{}
	}
	out := make(map[string]int, len(perTask))
	for roleName, attempts := range perTask {
		out[roleName] = attempts
	}
	return out
}

func (o *Orchestrator) hydrateRoleAttemptsFromRepo(ctx context.Context, taskID string) {
	taskID = strings.TrimSpace(taskID)
	if o == nil || o.roleLoopAttemptRepo == nil || taskID == "" {
		return
	}
	o.roleLoopMu.Lock()
	if o.roleLoopLoaded[taskID] {
		o.roleLoopMu.Unlock()
		return
	}
	o.roleLoopMu.Unlock()

	perTask, err := o.roleLoopAttemptRepo.GetTaskAttempts(ctx, taskID)
	if err != nil {
		return
	}

	o.roleLoopMu.Lock()
	defer o.roleLoopMu.Unlock()
	if o.roleLoopLoaded[taskID] {
		return
	}
	o.roleLoopAttempts[taskID] = perTask
	o.roleLoopLoaded[taskID] = true
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

func deriveExecutionStage(state *ProjectState, pb *Playbook) string {
	if state == nil || state.Project == nil {
		return firstEnabledStage(pb)
	}
	status := strings.ToLower(strings.TrimSpace(state.Project.Status))
	switch status {
	case "plan", "planning":
		return stagePlan
	case "test", "testing", "qa", "verifying":
		return stageTest
	case "build", "building", "developing":
		return stageBuild
	}

	if hasOpenExecutionTasks(state.Tasks) || hasActiveWorktrees(state.Worktrees) {
		return stageBuild
	}
	if hasCompletedExecutionTasks(state.Tasks) && !hasOpenExecutionTasks(state.Tasks) && !hasActiveWorktrees(state.Worktrees) {
		if pb != nil && pb.Workflow.Test.Enabled {
			return stageTest
		}
		return stageBuild
	}
	if len(state.Tasks) == 0 && len(state.Worktrees) == 0 {
		if pb != nil && pb.Workflow.Plan.Enabled {
			return stagePlan
		}
	}
	return firstEnabledStage(pb)
}

func firstEnabledStage(pb *Playbook) string {
	if pb != nil {
		if pb.Workflow.Plan.Enabled {
			return stagePlan
		}
		if pb.Workflow.Build.Enabled {
			return stageBuild
		}
		if pb.Workflow.Test.Enabled {
			return stageTest
		}
	}
	return stageBuild
}

func hasOpenExecutionTasks(tasks []*db.Task) bool {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(task.Status)) {
		case "", "pending", "planned", "planning", "ready", "todo", "queued", "running", "in_progress", "reviewing", "waiting_review", "blocked":
			return true
		}
	}
	return false
}

func hasCompletedExecutionTasks(tasks []*db.Task) bool {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(task.Status)) {
		case "done", "completed", "merged", "closed":
			return true
		}
	}
	return false
}

func hasActiveWorktrees(worktrees []*db.Worktree) bool {
	for _, wt := range worktrees {
		if wt == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(wt.Status)) {
		case "", "active", "created", "in_progress", "running", "open":
			return true
		}
	}
	return false
}

func stageToolAllowed(stage string, toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return true
	}
	allowedByStage := map[string]map[string]struct{}{
		stagePlan: {
			"list_skills":            {},
			"get_skill_details":      {},
			"get_project_status":     {},
			"create_task":            {},
			"create_worktree":        {},
			"write_task_spec":        {},
			"create_session":         {},
			"wait_for_session_ready": {},
			"send_command":           {},
			"read_session_output":    {},
			"is_session_idle":        {},
			"can_close_session":      {},
			"close_session":          {},
		},
		stageBuild: {
			"list_skills":            {},
			"get_skill_details":      {},
			"get_project_status":     {},
			"create_task":            {},
			"create_worktree":        {},
			"write_task_spec":        {},
			"merge_worktree":         {},
			"resolve_merge_conflict": {},
			"create_session":         {},
			"wait_for_session_ready": {},
			"send_command":           {},
			"read_session_output":    {},
			"is_session_idle":        {},
			"can_close_session":      {},
			"close_session":          {},
		},
		stageTest: {
			"list_skills":            {},
			"get_skill_details":      {},
			"get_project_status":     {},
			"write_task_spec":        {},
			"create_session":         {},
			"wait_for_session_ready": {},
			"send_command":           {},
			"read_session_output":    {},
			"is_session_idle":        {},
			"can_close_session":      {},
			"close_session":          {},
		},
	}
	stage = strings.ToLower(strings.TrimSpace(stage))
	allowed, ok := allowedByStage[stage]
	if !ok {
		allowed = allowedByStage[stageBuild]
	}
	_, ok = allowed[toolName]
	return ok
}

func (o *Orchestrator) enforceStageToolGate(ctx context.Context, toolName string, args map[string]any) error {
	if o == nil || o.lane != executionLane {
		return nil
	}
	projectID, err := o.projectIDForTool(ctx, toolName, args)
	if err != nil || strings.TrimSpace(projectID) == "" {
		return nil
	}
	state, err := o.loadProjectState(ctx, projectID)
	if err != nil {
		if o.projectRepo == nil {
			return nil
		}
		project, getErr := o.projectRepo.Get(ctx, projectID)
		if getErr != nil || project == nil {
			return nil
		}
		state = &ProjectState{Project: project}
	}
	matchedPlaybook := o.loadProjectPlaybook(ctx, state.Project)
	if matchedPlaybook == nil {
		matchedPlaybook = o.loadWorkflowAsPlaybook(ctx, projectID)
	}
	stage := deriveExecutionStage(state, matchedPlaybook)
	if stageToolAllowed(stage, toolName) {
		return nil
	}
	return fmt.Errorf("stage_tool_not_allowed: tool %q is not allowed during %s stage", strings.TrimSpace(toolName), stage)
}

func (o *Orchestrator) projectIDForTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	if o == nil {
		return "", nil
	}
	toolName = strings.TrimSpace(toolName)
	switch toolName {
	case "create_task", "create_worktree", "write_task_spec", "get_project_status":
		return optionalString(args, "project_id")
	case "create_session":
		taskID, err := optionalString(args, "task_id")
		if err != nil || strings.TrimSpace(taskID) == "" || o.taskRepo == nil {
			return "", err
		}
		task, err := o.taskRepo.Get(ctx, strings.TrimSpace(taskID))
		if err != nil || task == nil {
			return "", err
		}
		return strings.TrimSpace(task.ProjectID), nil
	case "send_command", "read_session_output", "is_session_idle", "close_session", "can_close_session", "wait_for_session_ready":
		sessionID, err := optionalString(args, "session_id")
		if err != nil || strings.TrimSpace(sessionID) == "" || o.sessionRepo == nil || o.taskRepo == nil {
			return "", err
		}
		sess, err := o.sessionRepo.Get(ctx, strings.TrimSpace(sessionID))
		if err != nil || sess == nil {
			return "", err
		}
		task, err := o.taskRepo.Get(ctx, sess.TaskID)
		if err != nil || task == nil {
			return "", err
		}
		return strings.TrimSpace(task.ProjectID), nil
	case "merge_worktree", "resolve_merge_conflict":
		worktreeID, err := optionalString(args, "worktree_id")
		if err != nil || strings.TrimSpace(worktreeID) == "" || o.worktreeRepo == nil {
			return "", err
		}
		wt, err := o.worktreeRepo.Get(ctx, strings.TrimSpace(worktreeID))
		if err != nil || wt == nil {
			return "", err
		}
		return strings.TrimSpace(wt.ProjectID), nil
	default:
		return "", nil
	}
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
			"read_session_output", "is_session_idle", "get_project_status",
		}, toolName)
	case "reviewer":
		return containsFold([]string{
			"create_session", "send_command", "read_session_output", "is_session_idle",
			"can_close_session",
		}, toolName)
	case "tester":
		return containsFold([]string{
			"create_session", "send_command", "read_session_output", "is_session_idle",
			"can_close_session", "close_session",
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

	endpoint := normalizeOpenAIEndpoint(cfg.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
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
		return nil, fmt.Errorf("openai api status=%d endpoint=%s body=%s", resp.StatusCode, endpoint, strings.TrimSpace(string(body)))
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

func normalizeOpenAIEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultOpenAIURL
	}
	if strings.HasSuffix(trimmed, "/chat/completions") || strings.HasSuffix(trimmed, "/responses") {
		return trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.HasSuffix(trimmed, "/v1") {
			return trimmed + "/chat/completions"
		}
		return strings.TrimRight(trimmed, "/") + "/v1/chat/completions"
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = "/v1/chat/completions"
	case path == "/v1":
		parsed.Path = "/v1/chat/completions"
	default:
		parsed.Path = path + "/chat/completions"
	}
	return parsed.String()
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
