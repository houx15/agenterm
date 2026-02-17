# Task: orchestrator

## Context
The Orchestrator is the "AI Project Manager" — an LLM agent that decomposes user requirements into tasks, creates worktrees, dispatches agents, monitors progress, and generates reports. It operates by calling the Go Backend's REST API as its tool set (function calling). It's event-driven and stateless: triggered by user input or system events, reads current state, makes decisions, executes actions, exits.

## Objective
Implement the Orchestrator as a Go service that calls an LLM API (Claude/OpenAI) with function calling, using the AgenTerm REST API as its tools. Include the PM Chat WebSocket endpoint for streaming responses to the frontend.

## Dependencies
- Depends on: TASK-08 (rest-api), TASK-09 (agent-registry), TASK-11 (worktree-management), TASK-12 (session-lifecycle)
- Branch: feature/orchestrator
- Base: main (after all dependencies merge)

## Scope

### Files to Create
- `internal/orchestrator/orchestrator.go` — Core orchestrator: LLM API call with tools, conversation management
- `internal/orchestrator/tools.go` — Tool definitions mapping to REST API calls
- `internal/orchestrator/prompt.go` — System prompt construction (inject project state, agent registry, playbook)
- `internal/orchestrator/events.go` — Event triggers: session completion, timer, user input
- `internal/api/orchestrator.go` — API handler: POST /api/orchestrator/chat, GET /api/orchestrator/report

### Files to Modify
- `internal/hub/hub.go` — Add orchestrator WebSocket channel (ws/orchestrator)
- `internal/hub/protocol.go` — Add orchestrator message types
- `internal/server/server.go` — Add /ws/orchestrator endpoint
- `internal/config/config.go` — Add LLM API key config, model selection
- `cmd/agenterm/main.go` — Initialize orchestrator, wire event triggers

### Files NOT to Touch
- `internal/tmux/` — Orchestrator uses API, not tmux directly
- `internal/parser/` — No changes
- `internal/git/` — No changes

## Implementation Spec

### Step 1: Tool definitions
Map SPEC Section 4.2 tools to REST API calls:
```go
var tools = []Tool{
    {
        Name: "create_project",
        Description: "Create a new project with a name and repository path",
        Parameters: map[string]Param{
            "name":      {Type: "string", Required: true},
            "repo_path": {Type: "string", Required: true},
            "playbook":  {Type: "string", Required: false},
        },
        Execute: func(args map[string]any) (string, error) {
            // POST /api/projects
        },
    },
    // create_task, create_worktree, create_session, send_command,
    // read_session_output, is_session_idle, get_project_status,
    // write_task_spec, generate_progress_report, etc.
}
```

### Step 2: System prompt
```go
func BuildSystemPrompt(projectState *ProjectState, agents []*AgentConfig, playbook *Playbook) string {
    // Constructs the prompt from SPEC Section 4.4:
    // - Role description (PM AI)
    // - Current project/task/session state
    // - Available agents with capabilities
    // - Matched playbook phases
    // - Rules (don't touch human_takeover sessions, prefer parallelism, etc.)
}
```

### Step 3: LLM API integration
```go
type Orchestrator struct {
    apiKey      string
    model       string          // e.g., "claude-sonnet-4-5-20250929"
    tools       []Tool
    history     []Message       // Conversation history (per project)
    apiBaseURL  string          // AgenTerm REST API base URL
}

func (o *Orchestrator) Chat(ctx context.Context, userMessage string, projectID string) (<-chan string, error) {
    // 1. Build system prompt with current project state
    // 2. Append user message to history
    // 3. Call LLM API with tools
    // 4. Process tool calls in a loop until LLM returns final text
    // 5. Stream response tokens via channel
    // 6. Save conversation to DB
}
```

### Step 4: Tool execution loop
```
User message → LLM → tool_call → Execute tool → tool_result → LLM → tool_call → ... → final_text
```
- Max 10 tool call rounds per invocation
- Each tool call executes against the local REST API (or directly calls the repo/manager)
- Tool results are fed back as assistant messages

### Step 5: WebSocket streaming endpoint
```
ws://host/ws/orchestrator?project_id=<id>
```
- Client sends: `{"type": "chat", "message": "..."}`
- Server streams: `{"type": "token", "text": "..."}` for each LLM output token
- Server sends: `{"type": "tool_call", "name": "...", "args": {...}}` for visibility
- Server sends: `{"type": "tool_result", "name": "...", "result": "..."}` for visibility
- Server sends: `{"type": "done"}` when complete

### Step 6: Event-driven triggers
```go
type EventTrigger struct {
    orchestrator *Orchestrator
}

// Called when a session becomes idle
func (et *EventTrigger) OnSessionIdle(sessionID string)

// Called periodically (every 60s) to check project progress
func (et *EventTrigger) OnTimer(projectID string)

// Called when a commit with [READY_FOR_REVIEW] is detected
func (et *EventTrigger) OnReviewReady(sessionID string, commitHash string)
```
Each trigger constructs a synthetic "user message" describing the event, then calls `Chat()`.

### Step 7: REST API endpoint
```
POST /api/orchestrator/chat
Body: {"project_id": "...", "message": "..."}
Response: SSE stream or JSON with full response
```
For non-WebSocket clients (like curl or the orchestrator calling itself).

## Testing Requirements
- Test tool definitions generate valid LLM function schemas
- Test system prompt includes current state
- Test tool execution loop with mock LLM responses
- Test streaming WebSocket endpoint
- Test event triggers generate appropriate messages
- Test conversation history persistence

## Acceptance Criteria
- [ ] Orchestrator can receive natural language and decompose into tasks
- [ ] Tool calls execute against real API and affect system state
- [ ] Streaming responses visible in PM Chat UI
- [ ] Tool call visibility (user can see what the PM is doing)
- [ ] Event-driven triggers work (session idle → orchestrator reacts)
- [ ] Conversation history persisted per project
- [ ] Configurable LLM model and API key

## Notes
- Start with Claude API (Anthropic). Can add OpenAI/Gemini later.
- Use `github.com/anthropics/anthropic-sdk-go` for Claude API
- The orchestrator should NEVER directly access tmux — always through API/managers
- Keep conversation history bounded (last 50 messages + summarize older ones)
- Consider rate limiting orchestrator actions to prevent runaway loops
- The system prompt should emphasize: don't send commands to human_takeover sessions
