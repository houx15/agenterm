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
	ts := &Toolset{client: client, tools: make(map[string]Tool)}
	for _, t := range defaultTools(client) {
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
			Name:        "create_session",
			Description: "Create a coding session for task",
			Parameters: map[string]Param{
				"task_id":    {Type: "string", Description: "Task id", Required: true},
				"agent_type": {Type: "string", Description: "Agent type", Required: true},
				"role":       {Type: "string", Description: "Session role", Required: true},
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
				err = client.doJSON(ctx, http.MethodPost, "/api/sessions/"+sessionID+"/send", nil, map[string]any{"text": text}, &out)
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
