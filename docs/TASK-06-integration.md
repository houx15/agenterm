# Task: integration

## Context
All agenterm components have been built independently: tmux gateway, output parser, WebSocket hub, frontend UI, and project scaffold. This task wires them together into a working application.

**Tech stack:** Go 1.22+, all internal packages from previous features.

## Objective
Wire all components together in `cmd/agenterm/main.go` and `internal/server/server.go`, creating a fully functional agenterm that connects to tmux, parses output, and serves it to browsers via WebSocket.

## Dependencies
- Depends on: ALL previous features (01 through 05)
- Branch: feature/06-integration
- Base: main (after all previous features are merged)

## Scope

### Files to Modify
- `cmd/agenterm/main.go` — Wire all components, lifecycle management
- `internal/server/server.go` — Register WebSocket handler from hub, serve frontend

### Files to Create
- None (all packages exist, just need wiring)

### Files NOT to Touch
- `internal/tmux/` — already complete
- `internal/parser/` — already complete
- `internal/hub/` — already complete
- `web/index.html` — already complete

## Implementation Spec

### Step 1: Update `cmd/agenterm/main.go`

Wire the complete startup sequence:

```go
func main() {
    // 1. Parse config (already exists from feature/01)
    cfg := config.Load()

    // 2. Create tmux gateway
    gw := tmux.New(cfg.TmuxSession)

    // 3. Create output parser
    psr := parser.New()

    // 4. Create WebSocket hub with input callback
    h := hub.New(cfg.Token, func(windowID, keys string) {
        gw.SendKeys(windowID, keys)
    })

    // 5. Create HTTP server with hub's WebSocket handler
    srv := server.New(cfg, h)

    // 6. Start components in order
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Start tmux gateway
    if err := gw.Start(ctx); err != nil {
        log.Fatal("Failed to connect to tmux:", err)
    }

    // Start hub
    go h.Run(ctx)

    // Start event processing pipeline
    go processEvents(ctx, gw, psr, h)

    // Start HTTP server
    srv.Start(ctx)
}
```

### Step 2: Event Processing Pipeline

Create the goroutine that connects tmux events → parser → hub:

```go
func processEvents(ctx context.Context, gw *tmux.Gateway, psr *parser.Parser, h *hub.Hub) {
    // Goroutine 1: Feed tmux events into parser
    go func() {
        for event := range gw.Events() {
            switch event.Type {
            case tmux.EventOutput:
                psr.Feed(event.WindowID, event.Data)
            case tmux.EventWindowAdd:
                h.BroadcastWindows(convertWindows(gw.ListWindows()))
            case tmux.EventWindowClose:
                h.BroadcastWindows(convertWindows(gw.ListWindows()))
            case tmux.EventWindowRenamed:
                h.BroadcastWindows(convertWindows(gw.ListWindows()))
            }
        }
    }()

    // Goroutine 2: Read parsed messages and broadcast to clients
    go func() {
        for msg := range psr.Messages() {
            h.BroadcastOutput(hub.OutputMessage{
                Type:    "output",
                Window:  msg.WindowID,
                Text:    msg.Text,
                Class:   string(msg.Class),
                Actions: convertActions(msg.Actions),
                ID:      msg.ID,
                Ts:      msg.Timestamp.Unix(),
            })
        }
    }()

    // Goroutine 3: Periodically broadcast window status updates
    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            windows := gw.ListWindows()
            for _, w := range windows {
                status := psr.Status(w.ID)
                h.BroadcastStatus(w.ID, string(status))
            }
        }
    }
}
```

### Step 3: Update `internal/server/server.go`

Integrate the hub's WebSocket handler:

```go
func New(cfg *config.Config, h *hub.Hub) *Server {
    mux := http.NewServeMux()

    // Serve frontend
    mux.Handle("/", http.FileServer(http.FS(web.Assets)))

    // WebSocket endpoint (handled by hub)
    mux.HandleFunc("/ws", h.HandleWebSocket)

    return &Server{
        httpServer: &http.Server{
            Addr:    fmt.Sprintf(":%d", cfg.Port),
            Handler: mux,
        },
    }
}
```

### Step 4: Helper Conversions

Create helper functions to convert between internal types and hub protocol types:

```go
func convertWindows(windows []tmux.Window) []hub.WindowInfo { ... }
func convertActions(actions []parser.QuickAction) []hub.ActionMessage { ... }
```

### Step 5: Graceful Shutdown

Ensure clean shutdown order:
1. Cancel context (stops accepting new connections)
2. Stop HTTP server (with 5s timeout)
3. Stop parser (flush remaining messages)
4. Stop tmux gateway (close subprocess)
5. Log "agenterm stopped"

### Step 6: Startup Banner

Print a clear startup message:
```
agenterm v0.1.0
  tmux session: ai-coding
  listening on: http://0.0.0.0:8765
  access URL:   http://localhost:8765?token=abc123def456
  windows:      3 (claude-app-auth, kimi-web-tests, shell)

Ctrl+C to stop
```

## Testing Requirements
- End-to-end test:
  1. Create a tmux session with 2 windows
  2. Start agenterm pointing at that session
  3. Open browser to the URL
  4. Verify session list shows both windows
  5. Click a session, verify messages appear
  6. Type "echo hello" and send, verify it executes in tmux
  7. Click a quick action button on a Y/n prompt, verify it works
- Graceful shutdown: Ctrl+C cleanly stops all goroutines
- Reconnection: Kill and restart agenterm, verify browser reconnects

## Acceptance Criteria
- [ ] All components wire together without import cycles
- [ ] tmux output appears in the browser as chat messages
- [ ] User input from browser executes in tmux
- [ ] Quick action buttons work (send correct keys to tmux)
- [ ] Window list updates when tmux windows are created/closed/renamed
- [ ] Status indicators update correctly (working/waiting/idle)
- [ ] Multiple browser clients can connect simultaneously
- [ ] Graceful shutdown works cleanly
- [ ] Startup banner shows useful information

## Notes
- Watch for import cycles: `tmux` ← `parser` ← `hub` ← `server` is the dependency direction. No package should import a package that depends on it.
- The `processEvents` function is the heart of the application — it's the pipeline that connects everything. Keep it simple and linear.
- If tmux session doesn't exist at startup, print a helpful error: "No tmux session 'ai-coding' found. Create one with: tmux new-session -s ai-coding"
- Consider adding a `--version` flag that prints the build version.
