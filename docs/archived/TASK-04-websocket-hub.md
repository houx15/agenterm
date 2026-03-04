# Task: websocket-hub

## Context
Agenterm needs a WebSocket hub that fans out tmux events to multiple browser clients and routes client input back to tmux. This is the communication backbone between the Go backend and the browser frontend.

**Tech stack:** Go 1.22+, `golang.org/x/net/websocket` or `nhooyr.io/websocket` (preferred for modern API). This is the first feature that may introduce an external dependency.

## Objective
Implement a WebSocket hub that manages client connections, authenticates via token, broadcasts messages from tmux to all clients, and routes input from clients to specific tmux windows.

## Dependencies
- Depends on: feature/01-project-scaffold (server infrastructure)
- Depends on: feature/02-tmux-gateway (event types)
- Depends on: feature/03-output-parser (message types)
- Branch: feature/04-websocket-hub
- Base: main

## Scope

### Files to Create
- `internal/hub/hub.go` — Hub struct, client management, fan-out
- `internal/hub/client.go` — Individual WebSocket client handler
- `internal/hub/protocol.go` — JSON message types for WebSocket protocol

### Files to Modify
- `go.mod` — Add `nhooyr.io/websocket` dependency (or use `golang.org/x/net/websocket`)

### Files NOT to Touch
- `internal/tmux/` — consume only
- `internal/parser/` — consume only
- `web/` — frontend handles this in feature/05

## Implementation Spec

### Step 1: Protocol Types (`internal/hub/protocol.go`)

Define JSON-serializable message types:

```go
package hub

// Server → Client messages
type ServerMessage struct {
    Type string `json:"type"` // "output", "windows", "status", "error"
}

type OutputMessage struct {
    Type     string            `json:"type"`     // "output"
    Window   string            `json:"window"`   // "@0"
    Text     string            `json:"text"`
    Class    string            `json:"class"`    // "normal", "prompt", "error", "code"
    Actions  []ActionMessage   `json:"actions,omitempty"`
    ID       string            `json:"id"`
    Ts       int64             `json:"ts"`       // unix timestamp
}

type ActionMessage struct {
    Label string `json:"label"` // "Yes"
    Keys  string `json:"keys"`  // "y\n"
}

type WindowsMessage struct {
    Type string          `json:"type"` // "windows"
    List []WindowInfo    `json:"list"`
}

type WindowInfo struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"` // "working", "waiting", "idle"
}

type StatusMessage struct {
    Type   string `json:"type"`   // "status"
    Window string `json:"window"`
    Status string `json:"status"`
}

// Client → Server messages
type ClientMessage struct {
    Type   string `json:"type"`   // "input", "subscribe"
    Window string `json:"window"` // target window ID
    Keys   string `json:"keys"`   // for "input" type
}
```

### Step 2: Client Handler (`internal/hub/client.go`)

```go
type Client struct {
    id     string
    conn   *websocket.Conn // or net.Conn depending on library
    send   chan []byte      // outbound messages
    hub    *Hub
}

func (c *Client) readPump()   // goroutine: read from WebSocket, dispatch to hub
func (c *Client) writePump()  // goroutine: write from send channel to WebSocket
```

**readPump:**
- Read JSON messages from WebSocket
- Parse as `ClientMessage`
- For `type: "input"`: forward to hub's input handler
- On error/disconnect: unregister from hub

**writePump:**
- Read from `send` channel
- Write to WebSocket as text messages
- Implement ping/pong for keepalive (30s interval)
- On write error: close connection

### Step 3: Hub (`internal/hub/hub.go`)

```go
type Hub struct {
    clients    map[string]*Client
    register   chan *Client
    unregister chan *Client
    broadcast  chan []byte     // send to all clients
    onInput    func(windowID string, keys string) // callback to tmux gateway
    token      string
    mu         sync.RWMutex
}

func New(token string, onInput func(string, string)) *Hub
func (h *Hub) Run(ctx context.Context)                    // main loop
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) // HTTP handler
func (h *Hub) BroadcastOutput(msg OutputMessage)          // send output to all clients
func (h *Hub) BroadcastWindows(windows []WindowInfo)      // send window list
func (h *Hub) BroadcastStatus(windowID string, status string)
func (h *Hub) ClientCount() int
```

**HandleWebSocket:**
1. Extract token from query parameter `?token=`
2. Compare with `h.token` — reject with 401 if mismatch
3. Upgrade HTTP connection to WebSocket
4. Create Client, register with hub
5. Start readPump and writePump goroutines
6. Send initial `WindowsMessage` with current window list

**Run() main loop:**
- Select on register, unregister, broadcast channels
- On register: add client to map
- On unregister: remove client, close send channel
- On broadcast: iterate clients, send to each client's send channel (non-blocking, drop if full)

**BroadcastOutput:**
- Marshal `OutputMessage` to JSON
- Send to broadcast channel

### Step 4: Rate Limiting / Batching
- Implement a simple rate limiter per window: max 1 broadcast per 100ms
- If multiple outputs arrive within 100ms, concatenate them into one message
- This prevents flooding mobile browsers with rapid terminal output

## Testing Requirements
- Unit test protocol marshal/unmarshal
- Unit test token authentication (accept valid, reject invalid, reject missing)
- Unit test client lifecycle (register, send message, unregister)
- Unit test broadcast fan-out (2 clients receive same message)
- Unit test rate limiting (rapid outputs are batched)
- Integration test with real WebSocket connection (use `httptest.Server`)

## Acceptance Criteria
- [ ] WebSocket endpoint accepts connections with valid token
- [ ] WebSocket endpoint rejects connections with invalid/missing token (HTTP 401)
- [ ] Messages are broadcast to all connected clients
- [ ] Client input is routed to the onInput callback with correct windowID
- [ ] Disconnected clients are cleaned up (no goroutine leaks)
- [ ] Rapid outputs are rate-limited to 1 per 100ms per window
- [ ] Ping/pong keepalive prevents stale connections

## Notes
- Use `nhooyr.io/websocket` if adding a dependency is OK — it has a cleaner API than `gorilla/websocket` (which is archived). Alternatively, `golang.org/x/net/websocket` works but is lower-level.
- The `send` channel per client should be buffered (e.g., 256). If a client can't keep up (channel full), drop the message rather than blocking.
- Log client connect/disconnect events with client count for debugging.
- The `onInput` callback will be wired to `tmux.Gateway.SendKeys()` in feature/06-integration.
