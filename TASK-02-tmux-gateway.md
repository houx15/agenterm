# Task: tmux-gateway

## Context
Agenterm is a Go-based terminal chat bridge. This feature implements the core tmux Control Mode integration — the component that spawns `tmux -C`, reads its structured protocol, and writes commands to it.

**Tech stack:** Go 1.22+, stdlib only. Uses `os/exec` and goroutines.

**Project structure:**
```
internal/
  tmux/
    gateway.go    — main gateway struct and lifecycle
    protocol.go   — control mode protocol parser
    types.go      — data types (Window, Event, etc.)
```

## Objective
Implement a `tmux.Gateway` that connects to a tmux session via Control Mode, parses all events (output, window lifecycle), and provides channels for reading events and sending commands.

## Dependencies
- Depends on: feature/01-project-scaffold (needs go.mod, project structure)
- Branch: feature/02-tmux-gateway
- `internal/tmux/types.go` — Data types
- `internal/tmux/protocol.go` — Control Mode protocol parser
- `internal/tmux/gateway.go` — Gateway struct, subprocess management, goroutines

### Files to Modify
- None directly (this is a library package, integrated in feature/06)

### Files NOT to Touch
- `cmd/agenterm/main.go` — integration happens in feature/06
- `internal/server/` — integration happens in feature/06

## Implementation Spec

### Step 1: Define Types (`internal/tmux/types.go`)

```go
package tmux

type EventType int

const (
    EventOutput       EventType = iota // %output
    EventWindowAdd                      // %window-add
    EventWindowClose                    // %window-close
    EventWindowRenamed                  // %window-renamed
    EventLayoutChange                   // %layout-change
    EventBegin                          // %begin
    EventEnd                            // %end
    EventError                          // %error
)

type Event struct {
    Type     EventType
    WindowID string // e.g., "@0"
    PaneID   string // e.g., "%0"
    Data     string // decoded output data or event payload
    Raw      string // original line from tmux
}

type Window struct {
    ID     string // e.g., "@0"
    Name   string
    Active bool
}
```

### Step 2: Protocol Parser (`internal/tmux/protocol.go`)

Implement `func ParseLine(line string) (Event, error)` that parses tmux Control Mode output lines:

- `%output %<pane_id> <octal-escaped-data>` → EventOutput
  - **Critical:** Decode octal escapes (`\012` → `\n`, `\033` → ESC, etc.)
  - Use a custom decoder: scan for `\` followed by 3 octal digits, replace with the byte value
- `%window-add @<id>` → EventWindowAdd
- `%window-close @<id>` → EventWindowClose
- `%window-renamed @<id> <new-name>` → EventWindowRenamed
- `%begin <timestamp> <flags> <cmd_number>` → EventBegin
- `%end <timestamp> <flags> <cmd_number>` → EventEnd
- `%error <timestamp> <flags> <cmd_number>` → EventError
- Lines not starting with `%` → treat as command response data (buffer between %begin/%end)

Also implement `func DecodeOctal(s string) string` — the octal escape decoder.

### Step 3: Gateway (`internal/tmux/gateway.go`)

```go
type Gateway struct {
    session  string
    process  *exec.Cmd
    stdin    io.WriteCloser
    events   chan Event
    done     chan struct{}
    windows  map[string]*Window
    mu       sync.RWMutex
}

func New(session string) *Gateway
func (g *Gateway) Start(ctx context.Context) error  // spawn tmux -C, start reader goroutine
func (g *Gateway) Stop() error                       // kill subprocess, cleanup
func (g *Gateway) Events() <-chan Event              // read-only channel of parsed events
func (g *Gateway) SendKeys(windowID string, keys string) error  // write send-keys command
func (g *Gateway) ListWindows() []Window             // snapshot of current windows
```

**Start() implementation:**
1. Spawn `tmux -C attach-session -t <session>` via `exec.CommandContext`
2. Pipe stdin for writing commands
3. Start a goroutine that reads stdout line by line using `bufio.Scanner`
4. Each line → `ParseLine()` → send to `events` channel
5. On `EventWindowAdd`: add to `windows` map
6. On `EventWindowClose`: remove from `windows` map
7. On `EventWindowRenamed`: update name in `windows` map
8. On process exit: close channels, set done

**SendKeys() implementation:**
- Write `send-keys -t <windowID> <keys>\n` to stdin
- Keys containing special chars must be properly escaped for tmux
- Common mappings: `\n` → `Enter`, `\x03` → `C-c`

**Initial window discovery:**
- After connecting, send `list-windows -F '#{window_id} #{window_name} #{window_active}'`
- Parse the response (lines between %begin/%end) to populate initial `windows` map

## Testing Requirements
- Unit test `ParseLine` with all event types
- Unit test `DecodeOctal` with various escape sequences:
  - `\012` → newline
  - `\033[31m` → ANSI red
  - Mixed text: `Hello\012World` → `Hello\nWorld`
  - No escapes: plain text passes through unchanged
- Integration test (requires tmux installed):
  - Create a test tmux session
  - Connect Gateway
  - Verify ListWindows returns the session's windows
  - SendKeys "echo hello" + Enter
  - Verify EventOutput received with "hello" in data
  - Clean up test session

## Acceptance Criteria
- [ ] Gateway connects to a running tmux session
- [ ] All window lifecycle events are parsed and tracked
- [ ] Output events are properly octal-decoded
- [ ] SendKeys successfully injects keystrokes into tmux windows
- [ ] Gateway cleanly shuts down when context is cancelled
- [ ] No goroutine leaks (all goroutines exit on Stop())

## Notes
- tmux Control Mode uses `%output %<pane_id>` (pane, not window). Multiple panes per window are possible but for v1 we assume 1 pane per window. Map pane IDs to their parent window IDs.
- The `-C` flag (single C) gives Control Mode. `-CC` is iTerm2-specific and should NOT be used.
- If tmux session doesn't exist, `Start()` should return a clear error message guiding the user to create it.
- The events channel should be buffered (e.g., 1000) to avoid blocking the reader goroutine on slow consumers.