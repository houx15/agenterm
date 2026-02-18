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
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/playbook"
	"github.com/user/agenterm/internal/registry"
)

const (
	defaultModel             = "claude-sonnet-4-5"
	defaultAnthropicURL      = "https://api.anthropic.com/v1/messages"
	defaultMaxToolRounds     = 10
	defaultMaxHistory        = 50
	defaultGlobalMaxParallel = 32
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
	matchedPlaybook := o.loadProjectPlaybook(ctx, state.Project)
	if matchedPlaybook == nil {
		matchedPlaybook = o.loadWorkflowAsPlaybook(ctx, projectID)
	}
	systemPrompt := BuildSystemPrompt(state, agents, matchedPlaybook)
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
				Model:     o.model,
				MaxTokens: 1024,
				System:    systemPrompt,
				Tools:     o.toolset.JSONSchemas(),
				Messages:  messages,
			})
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
					result, err := o.toolset.Execute(ctx, block.Name, block.Input)
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
	phases := make([]PlaybookPhase, 0, len(workflow.Phases))
	for _, p := range workflow.Phases {
		if p == nil {
			continue
		}
		phases = append(phases, PlaybookPhase{
			Name:        p.PhaseType + ":" + p.Role,
			Agent:       "",
			Role:        p.Role,
			Description: fmt.Sprintf("entry=%s | exit=%s | max_parallel=%d", p.EntryRule, p.ExitRule, p.MaxParallel),
		})
	}
	return &Playbook{
		ID:       workflow.ID,
		Name:     workflow.Name,
		Phases:   phases,
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

	phases := make([]PlaybookPhase, 0, len(pb.Phases))
	for _, phase := range pb.Phases {
		phases = append(phases, PlaybookPhase{
			Name:        phase.Name,
			Agent:       phase.Agent,
			Role:        phase.Role,
			Description: phase.Description,
		})
	}
	return &Playbook{
		ID:       pb.ID,
		Name:     pb.Name,
		Phases:   phases,
		Strategy: pb.ParallelismStrategy,
	}
}

func toJSON(v any) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err.Error())
	}
	return string(buf)
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

func (o *Orchestrator) createMessage(ctx context.Context, req anthropicRequest) (*anthropicResponse, error) {
	buf, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.anthropicBaseURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(o.apiKey) != "" {
		httpReq.Header.Set("x-api-key", o.apiKey)
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
