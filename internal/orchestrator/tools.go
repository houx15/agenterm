package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Param struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]Param
	Execute     func(ctx context.Context, args map[string]any) (any, error)
}

type ToolCallEvent struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type ToolResultEvent struct {
	Name   string `json:"name"`
	Result any    `json:"result"`
}

type RESTToolClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c *RESTToolClient) doJSON(ctx context.Context, method string, path string, query url.Values, reqBody any, out any) error {
	if c == nil {
		return fmt.Errorf("rest client is required")
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "http://127.0.0.1:8765"
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var body io.Reader
	if reqBody != nil {
		buf, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}

	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("api %s %s failed: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type Toolset struct {
	client *RESTToolClient
	tools  map[string]Tool
}

func NewToolset(client *RESTToolClient) *Toolset {
	return NewExecutionToolset(client)
}

func NewExecutionToolset(client *RESTToolClient) *Toolset {
	return newToolset(client, defaultTools(client))
}

func NewDemandToolset(client *RESTToolClient) *Toolset {
	readOnly := map[string]struct{}{
		"list_skills":        {},
		"get_skill_details":  {},
		"get_project_status": {},
	}
	base := defaultTools(client)
	tools := make([]Tool, 0, len(readOnly)+8)
	for _, tool := range base {
		if _, ok := readOnly[tool.Name]; ok {
			tools = append(tools, tool)
		}
	}
	tools = append(tools, demandPoolTools(client)...)
	return newToolset(client, tools)
}

func newToolset(client *RESTToolClient, tools []Tool) *Toolset {
	ts := &Toolset{client: client, tools: make(map[string]Tool)}
	for _, t := range tools {
		ts.tools[t.Name] = t
	}
	return ts
}

func (ts *Toolset) Definitions() []Tool {
	defs := make([]Tool, 0, len(ts.tools))
	for _, tool := range ts.tools {
		defs = append(defs, Tool{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters})
	}
	return defs
}

func (ts *Toolset) Execute(ctx context.Context, name string, args map[string]any) (any, error) {
	tool, ok := ts.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, args)
}

func (ts *Toolset) JSONSchemas() []map[string]any {
	defs := ts.Definitions()
	schemas := make([]map[string]any, 0, len(defs))
	for _, t := range defs {
		required := make([]string, 0)
		properties := make(map[string]any, len(t.Parameters))
		for key, p := range t.Parameters {
			properties[key] = map[string]any{
				"type":        p.Type,
				"description": p.Description,
			}
			if p.Required {
				required = append(required, key)
			}
		}
		schemas = append(schemas, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"input_schema": map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		})
	}
	return schemas
}

func defaultTools(client *RESTToolClient) []Tool {
	if client == nil {
		client = &RESTToolClient{}
	}
	return []Tool{
		{
			Name:        "list_skills",
			Description: "List orchestrator skills with short summaries",
			Parameters:  map[string]Param{},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				return map[string]any{"skills": SkillSummaries()}, nil
			},
		},
		{
			Name:        "get_skill_details",
			Description: "Fetch full description for one orchestrator skill",
			Parameters: map[string]Param{
				"skill_id": {Type: "string", Description: "Skill id returned by list_skills", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				skillID, err := requiredString(args, "skill_id")
				if err != nil {
					return nil, err
				}
				spec, ok := SkillDetailsByID(skillID)
				if !ok {
					return nil, fmt.Errorf("unknown skill_id: %s", strings.TrimSpace(skillID))
				}
				return map[string]any{
					"id":          spec.ID,
					"name":        spec.Name,
					"description": spec.Description,
					"details":     spec.Details,
					"path":        spec.Path,
				}, nil
			},
		},
		{
			Name:        "install_online_skill",
			Description: "Install a skill package from an online GitHub skill URL into local skills/",
			Parameters: map[string]Param{
				"url":       {Type: "string", Description: "GitHub tree URL or raw SKILL.md URL", Required: true},
				"overwrite": {Type: "boolean", Description: "Replace existing local skill if true"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				rawURL, err := requiredString(args, "url")
				if err != nil {
					return nil, err
				}
				overwrite, err := optionalBool(args, "overwrite")
				if err != nil {
					return nil, err
				}
				spec, err := InstallSkillFromURL(ctx, rawURL, overwrite)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"id":          spec.ID,
					"name":        spec.Name,
					"description": spec.Description,
					"path":        spec.Path,
				}, nil
			},
		},
		{
			Name:        "create_project",
			Description: "Create a new project",
			Parameters: map[string]Param{
				"name":      {Type: "string", Description: "Project name", Required: true},
				"repo_path": {Type: "string", Description: "Repository path", Required: true},
				"playbook":  {Type: "string", Description: "Optional playbook id"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				name, err := requiredString(args, "name")
				if err != nil {
					return nil, err
				}
				repoPath, err := requiredString(args, "repo_path")
				if err != nil {
					return nil, err
				}
				playbook, _ := optionalString(args, "playbook")
				body := map[string]any{"name": name, "repo_path": repoPath}
				if playbook != "" {
					body["playbook"] = playbook
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodPost, "/api/projects", nil, body, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "create_task",
			Description: "Create a task in a project",
			Parameters: map[string]Param{
				"project_id":  {Type: "string", Description: "Project id", Required: true},
				"title":       {Type: "string", Description: "Task title", Required: true},
				"description": {Type: "string", Description: "Task description"},
				"depends_on":  {Type: "array", Description: "Task dependency ids"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				title, err := requiredString(args, "title")
				if err != nil {
					return nil, err
				}
				description, _ := optionalString(args, "description")
				dependsOn, err := optionalStringSlice(args, "depends_on")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/projects/"+projectID+"/tasks", nil, map[string]any{
					"title":       title,
					"description": description,
					"depends_on":  dependsOn,
				}, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "create_worktree",
			Description: "Create worktree for a project/task",
			Parameters: map[string]Param{
				"project_id":  {Type: "string", Description: "Project id", Required: true},
				"task_id":     {Type: "string", Description: "Task id"},
				"branch_name": {Type: "string", Description: "Branch name"},
				"path":        {Type: "string", Description: "Worktree path"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				taskID, _ := optionalString(args, "task_id")
				branchName, _ := optionalString(args, "branch_name")
				path, _ := optionalString(args, "path")
				body := map[string]any{}
				if taskID != "" {
					body["task_id"] = taskID
				}
				if branchName != "" {
					body["branch_name"] = branchName
				}
				if path != "" {
					body["path"] = path
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/projects/"+projectID+"/worktrees", nil, body, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "merge_worktree",
			Description: "Merge a worktree branch into the project's default or specified target branch",
			Parameters: map[string]Param{
				"worktree_id":   {Type: "string", Description: "Worktree id", Required: true},
				"target_branch": {Type: "string", Description: "Optional explicit target branch"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				worktreeID, err := requiredString(args, "worktree_id")
				if err != nil {
					return nil, err
				}
				targetBranch, _ := optionalString(args, "target_branch")
				body := map[string]any{}
				if strings.TrimSpace(targetBranch) != "" {
					body["target_branch"] = targetBranch
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodPost, "/api/worktrees/"+worktreeID+"/merge", nil, body, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "resolve_merge_conflict",
			Description: "Request coder conflict resolution workflow for a worktree merge conflict",
			Parameters: map[string]Param{
				"worktree_id": {Type: "string", Description: "Worktree id", Required: true},
				"session_id":  {Type: "string", Description: "Optional coder session id"},
				"message":     {Type: "string", Description: "Optional instruction to send to coder"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				worktreeID, err := requiredString(args, "worktree_id")
				if err != nil {
					return nil, err
				}
				sessionID, _ := optionalString(args, "session_id")
				message, _ := optionalString(args, "message")
				body := map[string]any{}
				if strings.TrimSpace(sessionID) != "" {
					body["session_id"] = sessionID
				}
				if strings.TrimSpace(message) != "" {
					body["message"] = message
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodPost, "/api/worktrees/"+worktreeID+"/resolve-conflict", nil, body, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "create_session",
			Description: "Create a coding session for task",
			Parameters: map[string]Param{
				"task_id":    {Type: "string", Description: "Task id", Required: true},
				"agent_type": {Type: "string", Description: "Agent type", Required: true},
				"role":       {Type: "string", Description: "Session role", Required: true},
				"inputs":     {Type: "object", Description: "Optional role input payload; include required role inputs (e.g. goal/spec_path/worktree_id) for contract checks"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				taskID, err := requiredString(args, "task_id")
				if err != nil {
					return nil, err
				}
				agentType, err := requiredString(args, "agent_type")
				if err != nil {
					return nil, err
				}
				role, err := requiredString(args, "role")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/tasks/"+taskID+"/sessions", nil, map[string]any{
					"agent_type": agentType,
					"role":       role,
				}, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "send_command",
			Description: "Send command to session",
			Parameters: map[string]Param{
				"session_id": {Type: "string", Description: "Session id", Required: true},
				"text":       {Type: "string", Description: "Command text", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				text, err := requiredString(args, "text")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/sessions/"+sessionID+"/commands", nil, map[string]any{
					"op":   "send_text",
					"text": text,
				}, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "send_key",
			Description: "Send a control key to session (e.g. C-m/C-c/Escape/Tab)",
			Parameters: map[string]Param{
				"session_id": {Type: "string", Description: "Session id", Required: true},
				"key":        {Type: "string", Description: "Control key name", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				key, err := requiredString(args, "key")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/sessions/"+sessionID+"/commands", nil, map[string]any{
					"op":  "send_key",
					"key": key,
				}, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "read_session_output",
			Description: "Read latest output lines from session",
			Parameters: map[string]Param{
				"session_id": {Type: "string", Description: "Session id", Required: true},
				"lines":      {Type: "number", Description: "Number of lines"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				lines, _ := optionalInt(args, "lines")
				if lines <= 0 {
					lines = 200
				}
				query := url.Values{}
				query.Set("lines", fmt.Sprintf("%d", lines))
				var out []map[string]any
				err = client.doJSON(ctx, http.MethodGet, "/api/sessions/"+sessionID+"/output", query, nil, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "is_session_idle",
			Description: "Check whether a session is idle",
			Parameters: map[string]Param{
				"session_id": {Type: "string", Description: "Session id", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodGet, "/api/sessions/"+sessionID+"/idle", nil, nil, &out)
				if err != nil {
					return nil, err
				}
				waitingReview, _ := out["waiting_review"].(bool)
				humanTakeover, _ := out["human_takeover"].(bool)
				if waitingReview || humanTakeover {
					out["needs_response"] = true
					if humanTakeover {
						out["response_mode"] = "human_takeover"
						out["next_step_hint"] = "human intervention is required; ask user or hand over session"
					} else {
						out["response_mode"] = "waiting_review"
						out["next_step_hint"] = "read_session_output, then respond via send_command or request user confirmation"
					}
				} else {
					out["needs_response"] = false
				}
				return out, nil
			},
		},
		{
			Name:        "wait_for_session_ready",
			Description: "Wait until a session appears ready to accept interactive prompts",
			Parameters: map[string]Param{
				"session_id":      {Type: "string", Description: "Session id", Required: true},
				"timeout_seconds": {Type: "number", Description: "Max seconds to wait before timeout"},
				"poll_ms":         {Type: "number", Description: "Polling interval in milliseconds"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				timeoutSeconds, _ := optionalInt(args, "timeout_seconds")
				if timeoutSeconds <= 0 {
					timeoutSeconds = 45
				}
				pollMS, _ := optionalInt(args, "poll_ms")
				if pollMS <= 0 {
					pollMS = 500
				}
				deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
				for {
					var state map[string]any
					if err := client.doJSON(ctx, http.MethodGet, "/api/sessions/"+sessionID+"/ready", nil, nil, &state); err != nil {
						return nil, err
					}
					ready, _ := state["ready"].(bool)
					if ready {
						state["waited_ms"] = timeoutSeconds*1000 - int(time.Until(deadline)/time.Millisecond)
						return state, nil
					}
					if time.Now().After(deadline) {
						state["timeout"] = true
						state["waited_ms"] = timeoutSeconds * 1000
						return state, nil
					}
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(time.Duration(pollMS) * time.Millisecond):
					}
				}
			},
		},
		{
			Name:        "close_session",
			Description: "Close and end a session",
			Parameters: map[string]Param{
				"session_id": {Type: "string", Description: "Session id", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				if err := client.doJSON(ctx, http.MethodDelete, "/api/sessions/"+sessionID, nil, nil, nil); err != nil {
					return nil, err
				}
				return map[string]any{"status": "closed", "session_id": sessionID}, nil
			},
		},
		{
			Name:        "can_close_session",
			Description: "Check whether a session can be safely closed according to review gate",
			Parameters: map[string]Param{
				"session_id": {Type: "string", Description: "Session id", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				sessionID, err := requiredString(args, "session_id")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodGet, "/api/sessions/"+sessionID+"/close-check", nil, nil, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "get_project_status",
			Description: "Fetch project with tasks/worktrees/sessions",
			Parameters: map[string]Param{
				"project_id": {Type: "string", Description: "Project id", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodGet, "/api/projects/"+projectID, nil, nil, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "preview_assignments",
			Description: "Preview role-to-agent assignments for the active playbook roles",
			Parameters: map[string]Param{
				"project_id": {Type: "string", Description: "Project id", Required: true},
				"stage":      {Type: "string", Description: "Optional stage filter: plan|build|test"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				stage, _ := optionalString(args, "stage")
				body := map[string]any{}
				if strings.TrimSpace(stage) != "" {
					body["stage"] = strings.ToLower(strings.TrimSpace(stage))
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/projects/"+projectID+"/orchestrator/assignments/preview", nil, body, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "confirm_assignments",
			Description: "Confirm role-to-agent assignments and persist scheduler constraints",
			Parameters: map[string]Param{
				"project_id":  {Type: "string", Description: "Project id", Required: true},
				"assignments": {Type: "array", Description: "Assignment items: [{stage, role, agent_type, max_parallel}]", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				assignments, ok := args["assignments"]
				if !ok {
					return nil, fmt.Errorf("assignments is required")
				}
				var out map[string]any
				err = client.doJSON(ctx, http.MethodPost, "/api/projects/"+projectID+"/orchestrator/assignments/confirm", nil, map[string]any{
					"assignments": assignments,
				}, &out)
				if err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "list_assignments",
			Description: "List persisted role-to-agent assignments for a project",
			Parameters: map[string]Param{
				"project_id": {Type: "string", Description: "Project id", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				var out []map[string]any
				err = client.doJSON(ctx, http.MethodGet, "/api/projects/"+projectID+"/orchestrator/assignments", nil, nil, &out)
				if err != nil {
					return nil, err
				}
				return map[string]any{"assignments": out}, nil
			},
		},
		{
			Name:        "write_task_spec",
			Description: "Write a markdown task spec inside project repository",
			Parameters: map[string]Param{
				"project_id":    {Type: "string", Description: "Project id", Required: true},
				"relative_path": {Type: "string", Description: "Relative path inside repository", Required: true},
				"content":       {Type: "string", Description: "File content", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				relPath, err := requiredString(args, "relative_path")
				if err != nil {
					return nil, err
				}
				content, err := requiredString(args, "content")
				if err != nil {
					return nil, err
				}
				var status map[string]any
				if err := client.doJSON(ctx, http.MethodGet, "/api/projects/"+projectID, nil, nil, &status); err != nil {
					return nil, err
				}
				projectRaw, ok := status["project"].(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid project payload")
				}
				repoPath, _ := projectRaw["repo_path"].(string)
				if strings.TrimSpace(repoPath) == "" {
					return nil, fmt.Errorf("project repo_path missing")
				}
				cleanRel := filepath.Clean(relPath)
				if strings.HasPrefix(cleanRel, "..") {
					return nil, fmt.Errorf("relative_path must stay within repo")
				}
				target := filepath.Join(repoPath, cleanRel)
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return nil, err
				}
				if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
					return nil, err
				}
				return map[string]any{"written": target}, nil
			},
		},
		{
			Name:        "generate_progress_report",
			Description: "Generate a concise project progress report",
			Parameters: map[string]Param{
				"project_id": {Type: "string", Description: "Project id", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				var status map[string]any
				if err := client.doJSON(ctx, http.MethodGet, "/api/projects/"+projectID, nil, nil, &status); err != nil {
					return nil, err
				}
				return summarizeProjectStatus(status), nil
			},
		},
	}
}

func demandPoolTools(client *RESTToolClient) []Tool {
	if client == nil {
		client = &RESTToolClient{}
	}
	return []Tool{
		{
			Name:        "list_demand_pool",
			Description: "List demand pool items for one project",
			Parameters: map[string]Param{
				"project_id": {Type: "string", Description: "Project id", Required: true},
				"status":     {Type: "string", Description: "Optional status filter"},
				"q":          {Type: "string", Description: "Optional full-text query"},
				"limit":      {Type: "number", Description: "Optional page size"},
				"offset":     {Type: "number", Description: "Optional page offset"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				status, _ := optionalString(args, "status")
				queryText, _ := optionalString(args, "q")
				limit, err := optionalInt(args, "limit")
				if err != nil {
					return nil, err
				}
				offset, err := optionalInt(args, "offset")
				if err != nil {
					return nil, err
				}
				query := url.Values{}
				if strings.TrimSpace(status) != "" {
					query.Set("status", strings.TrimSpace(status))
				}
				if strings.TrimSpace(queryText) != "" {
					query.Set("q", strings.TrimSpace(queryText))
				}
				if limit > 0 {
					query.Set("limit", fmt.Sprintf("%d", limit))
				}
				if offset > 0 {
					query.Set("offset", fmt.Sprintf("%d", offset))
				}
				var out []map[string]any
				if err := client.doJSON(ctx, http.MethodGet, "/api/projects/"+projectID+"/demand-pool", query, nil, &out); err != nil {
					return nil, err
				}
				return map[string]any{"items": out, "count": len(out)}, nil
			},
		},
		{
			Name:        "create_demand_item",
			Description: "Create a demand pool item in a project",
			Parameters: map[string]Param{
				"project_id":  {Type: "string", Description: "Project id", Required: true},
				"title":       {Type: "string", Description: "Demand title", Required: true},
				"description": {Type: "string", Description: "Demand description"},
				"status":      {Type: "string", Description: "Demand status"},
				"priority":    {Type: "number", Description: "Priority score"},
				"impact":      {Type: "number", Description: "Impact score 1-5"},
				"effort":      {Type: "number", Description: "Effort score 1-5"},
				"risk":        {Type: "number", Description: "Risk score 1-5"},
				"urgency":     {Type: "number", Description: "Urgency score 1-5"},
				"notes":       {Type: "string", Description: "Optional notes"},
				"tags":        {Type: "array", Description: "Optional tag list"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				title, err := requiredString(args, "title")
				if err != nil {
					return nil, err
				}
				description, _ := optionalString(args, "description")
				status, _ := optionalString(args, "status")
				priority, err := optionalInt(args, "priority")
				if err != nil {
					return nil, err
				}
				impact, err := optionalInt(args, "impact")
				if err != nil {
					return nil, err
				}
				effort, err := optionalInt(args, "effort")
				if err != nil {
					return nil, err
				}
				risk, err := optionalInt(args, "risk")
				if err != nil {
					return nil, err
				}
				urgency, err := optionalInt(args, "urgency")
				if err != nil {
					return nil, err
				}
				notes, _ := optionalString(args, "notes")
				tags, err := optionalStringSlice(args, "tags")
				if err != nil {
					return nil, err
				}
				body := map[string]any{
					"title":       title,
					"description": description,
					"status":      status,
					"priority":    priority,
					"impact":      impact,
					"effort":      effort,
					"risk":        risk,
					"urgency":     urgency,
					"notes":       notes,
					"tags":        tags,
					"source":      "orchestrator",
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodPost, "/api/projects/"+projectID+"/demand-pool", nil, body, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "update_demand_item",
			Description: "Update an existing demand pool item",
			Parameters: map[string]Param{
				"item_id":          {Type: "string", Description: "Demand item id", Required: true},
				"title":            {Type: "string", Description: "Demand title"},
				"description":      {Type: "string", Description: "Demand description"},
				"status":           {Type: "string", Description: "Demand status"},
				"priority":         {Type: "number", Description: "Priority score"},
				"impact":           {Type: "number", Description: "Impact score"},
				"effort":           {Type: "number", Description: "Effort score"},
				"risk":             {Type: "number", Description: "Risk score"},
				"urgency":          {Type: "number", Description: "Urgency score"},
				"notes":            {Type: "string", Description: "Notes"},
				"selected_task_id": {Type: "string", Description: "Linked task id"},
				"tags":             {Type: "array", Description: "Optional tag list"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				itemID, err := requiredString(args, "item_id")
				if err != nil {
					return nil, err
				}
				body := map[string]any{}
				if title, _ := optionalString(args, "title"); strings.TrimSpace(title) != "" {
					body["title"] = strings.TrimSpace(title)
				}
				if description, _ := optionalString(args, "description"); strings.TrimSpace(description) != "" {
					body["description"] = strings.TrimSpace(description)
				}
				if status, _ := optionalString(args, "status"); strings.TrimSpace(status) != "" {
					body["status"] = strings.TrimSpace(status)
				}
				if notes, _ := optionalString(args, "notes"); strings.TrimSpace(notes) != "" {
					body["notes"] = strings.TrimSpace(notes)
				}
				if selectedTaskID, _ := optionalString(args, "selected_task_id"); strings.TrimSpace(selectedTaskID) != "" {
					body["selected_task_id"] = strings.TrimSpace(selectedTaskID)
				}
				if priority, err := optionalInt(args, "priority"); err != nil {
					return nil, err
				} else if _, ok := args["priority"]; ok {
					body["priority"] = priority
				}
				if impact, err := optionalInt(args, "impact"); err != nil {
					return nil, err
				} else if _, ok := args["impact"]; ok {
					body["impact"] = impact
				}
				if effort, err := optionalInt(args, "effort"); err != nil {
					return nil, err
				} else if _, ok := args["effort"]; ok {
					body["effort"] = effort
				}
				if risk, err := optionalInt(args, "risk"); err != nil {
					return nil, err
				} else if _, ok := args["risk"]; ok {
					body["risk"] = risk
				}
				if urgency, err := optionalInt(args, "urgency"); err != nil {
					return nil, err
				} else if _, ok := args["urgency"]; ok {
					body["urgency"] = urgency
				}
				if tags, err := optionalStringSlice(args, "tags"); err != nil {
					return nil, err
				} else if _, ok := args["tags"]; ok {
					body["tags"] = tags
				}
				if len(body) == 0 {
					return nil, fmt.Errorf("at least one field must be provided for update_demand_item")
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodPatch, "/api/demand-pool/"+itemID, nil, body, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
		{
			Name:        "reprioritize_demand_pool",
			Description: "Bulk reprioritize demand pool items by project",
			Parameters: map[string]Param{
				"project_id": {Type: "string", Description: "Project id", Required: true},
				"items":      {Type: "array", Description: "Array of {id, priority} entries", Required: true},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				projectID, err := requiredString(args, "project_id")
				if err != nil {
					return nil, err
				}
				items, ok := args["items"].([]any)
				if !ok || len(items) == 0 {
					return nil, fmt.Errorf("items is required and must be a non-empty array")
				}
				payloadItems := make([]map[string]any, 0, len(items))
				for _, item := range items {
					entry, ok := item.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("items must contain objects")
					}
					idRaw, ok := entry["id"].(string)
					if !ok || strings.TrimSpace(idRaw) == "" {
						return nil, fmt.Errorf("each reprioritize item must include id")
					}
					priorityRaw, ok := entry["priority"]
					if !ok {
						return nil, fmt.Errorf("each reprioritize item must include priority")
					}
					priority := 0
					switch v := priorityRaw.(type) {
					case float64:
						priority = int(v)
					case int:
						priority = v
					default:
						return nil, fmt.Errorf("priority must be numeric")
					}
					payloadItems = append(payloadItems, map[string]any{
						"id":       strings.TrimSpace(idRaw),
						"priority": priority,
					})
				}
				var out []map[string]any
				if err := client.doJSON(ctx, http.MethodPost, "/api/projects/"+projectID+"/demand-pool/reprioritize", nil, map[string]any{"items": payloadItems}, &out); err != nil {
					return nil, err
				}
				return map[string]any{"items": out, "count": len(out)}, nil
			},
		},
		{
			Name:        "promote_demand_item",
			Description: "Promote one demand item into a project task",
			Parameters: map[string]Param{
				"item_id":     {Type: "string", Description: "Demand item id", Required: true},
				"title":       {Type: "string", Description: "Task title override"},
				"description": {Type: "string", Description: "Task description override"},
				"status":      {Type: "string", Description: "Task status override"},
				"depends_on":  {Type: "array", Description: "Task dependency ids"},
			},
			Execute: func(ctx context.Context, args map[string]any) (any, error) {
				itemID, err := requiredString(args, "item_id")
				if err != nil {
					return nil, err
				}
				title, _ := optionalString(args, "title")
				description, _ := optionalString(args, "description")
				status, _ := optionalString(args, "status")
				dependsOn, err := optionalStringSlice(args, "depends_on")
				if err != nil {
					return nil, err
				}
				body := map[string]any{}
				if strings.TrimSpace(title) != "" {
					body["title"] = strings.TrimSpace(title)
				}
				if strings.TrimSpace(description) != "" {
					body["description"] = strings.TrimSpace(description)
				}
				if strings.TrimSpace(status) != "" {
					body["status"] = strings.TrimSpace(status)
				}
				if len(dependsOn) > 0 {
					body["depends_on"] = dependsOn
				}
				var out map[string]any
				if err := client.doJSON(ctx, http.MethodPost, "/api/demand-pool/"+itemID+"/promote", nil, body, &out); err != nil {
					return nil, err
				}
				return out, nil
			},
		},
	}
}

func summarizeProjectStatus(status map[string]any) map[string]any {
	project := map[string]any{}
	if p, ok := status["project"].(map[string]any); ok {
		project = p
	}
	tasks := readArray(status["tasks"])
	sessions := readArray(status["sessions"])
	worktrees := readArray(status["worktrees"])

	taskCounts := map[string]int{}
	for _, t := range tasks {
		st, _ := t["status"].(string)
		if st == "" {
			st = "unknown"
		}
		taskCounts[st]++
	}
	sessionCounts := map[string]int{}
	for _, s := range sessions {
		st, _ := s["status"].(string)
		if st == "" {
			st = "unknown"
		}
		sessionCounts[st]++
	}
	blockers := make([]any, 0, 4)
	if n := taskCounts["blocked"]; n > 0 {
		blockers = append(blockers, map[string]any{"type": "tasks_blocked", "count": n})
	}
	if n := taskCounts["failed"]; n > 0 {
		blockers = append(blockers, map[string]any{"type": "tasks_failed", "count": n})
	}
	if n := sessionCounts["failed"]; n > 0 {
		blockers = append(blockers, map[string]any{"type": "sessions_failed", "count": n})
	}
	queueDepth := 0
	for _, key := range []string{"pending", "queued", "ready", "todo"} {
		queueDepth += taskCounts[key]
	}
	phase := "planning"
	if len(blockers) > 0 {
		phase = "blocked"
	} else if sessionCounts["waiting_review"] > 0 {
		phase = "review"
	} else if sessionCounts["working"] > 0 || sessionCounts["running"] > 0 {
		phase = "implementation"
	} else if len(tasks) > 0 && taskCounts["done"] == len(tasks) {
		phase = "completed"
	}

	return map[string]any{
		"project":        project,
		"task_counts":    taskCounts,
		"session_counts": sessionCounts,
		"phase":          phase,
		"queue_depth":    queueDepth,
		"blockers":       blockers,
		"worktree_count": len(worktrees),
		"tasks_total":    len(tasks),
		"sessions_total": len(sessions),
	}
}

func readArray(v any) []map[string]any {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	items := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			items = append(items, m)
		}
	}
	return items
}

func requiredString(args map[string]any, key string) (string, error) {
	v, err := optionalString(args, key)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func optionalString(args map[string]any, key string) (string, error) {
	if args == nil {
		return "", nil
	}
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return s, nil
}

func optionalStringSlice(args map[string]any, key string) ([]string, error) {
	if args == nil {
		return nil, nil
	}
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must contain strings", key)
		}
		values = append(values, s)
	}
	return values, nil
}

func optionalInt(args map[string]any, key string) (int, error) {
	if args == nil {
		return 0, nil
	}
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func optionalBool(args map[string]any, key string) (bool, error) {
	if args == nil {
		return false, nil
	}
	v, ok := args[key]
	if !ok || v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}
