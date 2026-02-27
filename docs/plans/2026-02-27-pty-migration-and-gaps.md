# PTY Migration & Gap Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the tmux-based terminal runtime with native PTY sessions and fill all feature gaps identified in the gap analysis (workflow stages, demand pool CRUD, keyboard shortcuts, and orchestrator role adjustments).

**Architecture:** The backend swaps the `internal/tmux` package for a new `internal/pty` package using `creack/pty`. Each agent session spawns a child process with its own PTY fd. The existing `Gateway`/`Manager` interface contract is preserved so `session.Manager`, `hub`, and `main.go` require only targeted rewiring — not a full rewrite. The frontend changes are minimal since terminal data already flows as raw strings over WebSocket; only the ID scheme changes from tmux window IDs (`@0`) to PTY session UUIDs.

**Tech Stack:** Go 1.22, `github.com/creack/pty`, `nhooyr.io/websocket`, React 18, TypeScript, xterm.js, Vite

---

## Phase 1 — PTY Runtime (Backend)

### Task 1: Add `creack/pty` dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum` (auto-generated)

**Step 1: Add the dependency**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go get github.com/creack/pty@latest
```

**Step 2: Verify it appears in go.mod**

Run:
```bash
grep creack go.mod
```
Expected: `github.com/creack/pty v1.x.x`

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add creack/pty for native PTY runtime"
```

---

### Task 2: Create `internal/pty` package — types and interface

**Files:**
- Create: `internal/pty/types.go`
- Create: `internal/pty/session.go` (stub)
- Create: `internal/pty/manager.go` (stub)

**Step 1: Write `internal/pty/types.go`**

```go
package pty

import "time"

// EventType mirrors the events the rest of the app expects from a terminal backend.
type EventType int

const (
	EventOutput EventType = iota
	EventClosed
)

// Event is a single terminal event emitted by a PTY session.
type Event struct {
	Type EventType
	ID   string // session/window ID (UUID)
	Data string // raw terminal bytes (for EventOutput)
}

// SessionInfo is the metadata for one PTY session.
type SessionInfo struct {
	ID        string
	Name      string
	Active    bool
	CreatedAt time.Time
}
```

**Step 2: Write `internal/pty/session.go` — PTY session lifecycle (stub)**

```go
package pty

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Session wraps a single child process with a PTY file descriptor.
type Session struct {
	id      string
	name    string
	cmd     *exec.Cmd
	ptmx    *os.File
	events  chan Event
	cols    uint16
	rows    uint16

	mu       sync.Mutex
	closed   bool
	closeOnce sync.Once
}
```

**Step 3: Write `internal/pty/manager.go` — session pool (stub)**

```go
package pty

import "sync"

// Manager owns all PTY sessions and provides the same interface contract
// that session.Manager expects from its terminal backend.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates an empty PTY manager.
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}
```

**Step 4: Run `go build ./internal/pty/...` to verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./internal/pty/...
```
Expected: success, no errors

**Step 5: Commit**

```bash
git add internal/pty/
git commit -m "feat(pty): scaffold pty package with types, session, and manager stubs"
```

---

### Task 3: Implement `pty.Session` — spawn, read, write, resize, close

**Files:**
- Modify: `internal/pty/session.go`

**Step 1: Write the full Session implementation**

Replace `internal/pty/session.go` with:

```go
package pty

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	creackpty "github.com/creack/pty"
)

const (
	defaultCols     = 120
	defaultRows     = 30
	readBufferSize  = 4096
	eventBufferSize = 1024
)

// Session wraps a single child process with a PTY file descriptor.
type Session struct {
	id   string
	name string
	cmd  *exec.Cmd
	ptmx *os.File

	events chan Event
	cols   uint16
	rows   uint16

	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

// newSession spawns the command inside a PTY and starts the read pump.
// The caller receives events on Events().
func newSession(id, name string, argv []string, workDir string, env []string) (*Session, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("pty: empty command")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), env...)

	ptmx, err := creackpty.StartWithSize(cmd, &creackpty.Winsize{
		Cols: defaultCols,
		Rows: defaultRows,
	})
	if err != nil {
		return nil, fmt.Errorf("pty: start %q: %w", argv[0], err)
	}

	s := &Session{
		id:     id,
		name:   name,
		cmd:    cmd,
		ptmx:   ptmx,
		events: make(chan Event, eventBufferSize),
		cols:   defaultCols,
		rows:   defaultRows,
	}

	go s.readPump()
	go s.waitExit()

	return s, nil
}

// ID returns the session identifier.
func (s *Session) ID() string { return s.id }

// Name returns the human-readable session name.
func (s *Session) Name() string { return s.name }

// Events returns the channel that emits terminal output and lifecycle events.
func (s *Session) Events() <-chan Event { return s.events }

// Write sends raw bytes into the PTY (user keystrokes).
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, fmt.Errorf("pty: session closed")
	}
	return s.ptmx.Write(data)
}

// Resize changes the PTY window size.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("pty: session closed")
	}
	s.cols = cols
	s.rows = rows
	return creackpty.Setsize(s.ptmx, &creackpty.Winsize{Cols: cols, Rows: rows})
}

// Close terminates the PTY session. Safe to call multiple times.
func (s *Session) Close() error {
	var firstErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()

		// Signal the child process.
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGTERM)
		}
		if err := s.ptmx.Close(); err != nil {
			firstErr = err
		}
	})
	return firstErr
}

// readPump continuously reads from the PTY fd and emits EventOutput events.
func (s *Session) readPump() {
	buf := make([]byte, readBufferSize)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.events <- Event{
				Type: EventOutput,
				ID:   s.id,
				Data: string(buf[:n]),
			}
		}
		if err != nil {
			break // EOF or read error → child exited or PTY closed
		}
	}
}

// waitExit waits for the child process to finish and emits EventClosed.
func (s *Session) waitExit() {
	_ = s.cmd.Wait()
	s.events <- Event{
		Type: EventClosed,
		ID:   s.id,
	}
	close(s.events)
}
```

**Step 2: Verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./internal/pty/...
```
Expected: success

**Step 3: Commit**

```bash
git add internal/pty/session.go
git commit -m "feat(pty): implement Session spawn, readPump, write, resize, close"
```

---

### Task 4: Implement `pty.Manager` — CreateSession, GetSession, DestroySession, ListSessions

**Files:**
- Modify: `internal/pty/manager.go`

**Step 1: Write the full Manager implementation**

```go
package pty

import (
	"fmt"
	"sync"
)

// Manager owns all PTY sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates an empty PTY manager.
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

// CreateSession spawns a new PTY session.
// argv is the command + args (e.g., ["bash"] or ["claude", "--dangerously-skip-permissions"]).
// workDir is the initial working directory.
func (m *Manager) CreateSession(id, name string, argv []string, workDir string, env []string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; exists {
		return nil, fmt.Errorf("pty: session %q already exists", id)
	}

	sess, err := newSession(id, name, argv, workDir, env)
	if err != nil {
		return nil, err
	}

	m.sessions[id] = sess
	return sess, nil
}

// GetSession returns a running session by ID.
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("pty: session %q not found", id)
	}
	return sess, nil
}

// DestroySession terminates and removes a session.
func (m *Manager) DestroySession(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("pty: session %q not found", id)
	}
	return sess.Close()
}

// ListSessions returns metadata for all active sessions.
func (m *Manager) ListSessions() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]SessionInfo, 0, len(m.sessions))
	for _, sess := range m.sessions {
		result = append(result, SessionInfo{
			ID:     sess.id,
			Name:   sess.name,
			Active: !sess.closed,
		})
	}
	return result
}

// Close terminates all sessions.
func (m *Manager) Close() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, sess := range sessions {
		_ = sess.Close()
	}
}
```

**Step 2: Verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./internal/pty/...
```

**Step 3: Commit**

```bash
git add internal/pty/manager.go
git commit -m "feat(pty): implement Manager with create, get, destroy, list, close"
```

---

### Task 5: Write unit tests for `internal/pty`

**Files:**
- Create: `internal/pty/session_test.go`
- Create: `internal/pty/manager_test.go`

**Step 1: Write `session_test.go`**

```go
package pty

import (
	"strings"
	"testing"
	"time"
)

func TestSessionSpawnAndOutput(t *testing.T) {
	sess, err := newSession("test-1", "echo-test", []string{"echo", "hello-pty"}, "/tmp", nil)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer sess.Close()

	var output strings.Builder
	timeout := time.After(5 * time.Second)
	for {
		select {
		case evt, ok := <-sess.Events():
			if !ok {
				goto done
			}
			if evt.Type == EventOutput {
				output.WriteString(evt.Data)
			}
			if evt.Type == EventClosed {
				goto done
			}
		case <-timeout:
			t.Fatal("timeout waiting for session output")
		}
	}
done:
	if !strings.Contains(output.String(), "hello-pty") {
		t.Errorf("expected output to contain 'hello-pty', got: %q", output.String())
	}
}

func TestSessionResize(t *testing.T) {
	sess, err := newSession("test-resize", "resize", []string{"sleep", "10"}, "/tmp", nil)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer sess.Close()

	if err := sess.Resize(200, 50); err != nil {
		t.Errorf("Resize: %v", err)
	}
}

func TestSessionWriteAndClose(t *testing.T) {
	sess, err := newSession("test-write", "cat-test", []string{"cat"}, "/tmp", nil)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}

	if _, err := sess.Write([]byte("hello\n")); err != nil {
		t.Errorf("Write: %v", err)
	}

	if err := sess.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Double close should not panic.
	_ = sess.Close()
}
```

**Step 2: Write `manager_test.go`**

```go
package pty

import (
	"testing"
)

func TestManagerCreateAndDestroy(t *testing.T) {
	m := NewManager()
	defer m.Close()

	sess, err := m.CreateSession("s1", "test", []string{"sleep", "10"}, "/tmp", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID() != "s1" {
		t.Errorf("expected id 's1', got %q", sess.ID())
	}

	list := m.ListSessions()
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}

	if err := m.DestroySession("s1"); err != nil {
		t.Errorf("DestroySession: %v", err)
	}

	list = m.ListSessions()
	if len(list) != 0 {
		t.Errorf("expected 0 sessions after destroy, got %d", len(list))
	}
}

func TestManagerDuplicateSession(t *testing.T) {
	m := NewManager()
	defer m.Close()

	_, err := m.CreateSession("dup", "test", []string{"sleep", "10"}, "/tmp", nil)
	if err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}

	_, err = m.CreateSession("dup", "test", []string{"sleep", "10"}, "/tmp", nil)
	if err == nil {
		t.Error("expected error for duplicate session ID")
	}
}

func TestManagerGetSession(t *testing.T) {
	m := NewManager()
	defer m.Close()

	_, err := m.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}

	_, _ = m.CreateSession("exists", "test", []string{"sleep", "10"}, "/tmp", nil)
	sess, err := m.GetSession("exists")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.ID() != "exists" {
		t.Errorf("expected 'exists', got %q", sess.ID())
	}
}
```

**Step 3: Run tests**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go test ./internal/pty/... -v -count=1
```
Expected: all PASS

**Step 4: Commit**

```bash
git add internal/pty/session_test.go internal/pty/manager_test.go
git commit -m "test(pty): add session and manager unit tests"
```

---

### Task 6: Create `session.TerminalBackend` interface to decouple from tmux

**Files:**
- Create: `internal/session/backend.go`

The existing `session.Manager` references `tmux.Gateway` and `TmuxManager` interface directly. We introduce an abstraction layer so both tmux and PTY can satisfy the same contract.

**Step 1: Write `internal/session/backend.go`**

```go
package session

import "context"

// TerminalBackend abstracts the terminal runtime (tmux or PTY).
// session.Manager calls these methods instead of tmux.Gateway directly.
type TerminalBackend interface {
	// CreateSession spawns a new terminal session.
	// Returns the session ID (used as both session and "window" identifier in PTY mode).
	CreateSession(ctx context.Context, id string, name string, command string, workDir string) (string, error)

	// DestroySession kills the terminal session.
	DestroySession(ctx context.Context, id string) error

	// SendInput writes raw bytes to the terminal (user keystrokes).
	SendInput(ctx context.Context, id string, data string) error

	// SendKey sends a named key (Enter, C-c, etc.) to the terminal.
	SendKey(ctx context.Context, id string, key string) error

	// Resize changes the terminal dimensions.
	Resize(ctx context.Context, id string, cols, rows int) error

	// CaptureOutput returns the last N lines of terminal output.
	CaptureOutput(ctx context.Context, id string, lines int) ([]string, error)

	// SessionExists returns true if the session is still alive.
	SessionExists(ctx context.Context, id string) bool
}
```

**Step 2: Verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./internal/session/...
```

**Step 3: Commit**

```bash
git add internal/session/backend.go
git commit -m "feat(session): add TerminalBackend interface to decouple from tmux"
```

---

### Task 7: Implement PTY adapter for `TerminalBackend`

**Files:**
- Create: `internal/pty/backend.go`

**Step 1: Write `internal/pty/backend.go`**

This adapter wraps `pty.Manager` to satisfy `session.TerminalBackend`:

```go
package pty

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Backend adapts pty.Manager to satisfy session.TerminalBackend.
type Backend struct {
	manager *Manager

	// outputBuffers stores the last N bytes of output per session for CaptureOutput.
	mu            sync.RWMutex
	outputBuffers map[string]*ringBuf
}

const captureBufferSize = 256 * 1024 // 256 KB per session

// NewBackend creates a PTY backend.
func NewBackend() *Backend {
	return &Backend{
		manager:       NewManager(),
		outputBuffers: make(map[string]*ringBuf),
	}
}

func (b *Backend) CreateSession(_ context.Context, id, name, command, workDir string) (string, error) {
	argv := parseCommand(command)
	if len(argv) == 0 {
		return "", fmt.Errorf("pty backend: empty command")
	}

	sess, err := b.manager.CreateSession(id, name, argv, workDir, nil)
	if err != nil {
		return "", err
	}

	rb := newRingBuf(captureBufferSize)
	b.mu.Lock()
	b.outputBuffers[id] = rb
	b.mu.Unlock()

	// Start goroutine to buffer output for CaptureOutput.
	go func() {
		for evt := range sess.Events() {
			if evt.Type == EventOutput {
				rb.Write([]byte(evt.Data))
			}
		}
	}()

	return id, nil
}

func (b *Backend) DestroySession(_ context.Context, id string) error {
	b.mu.Lock()
	delete(b.outputBuffers, id)
	b.mu.Unlock()
	return b.manager.DestroySession(id)
}

func (b *Backend) SendInput(_ context.Context, id, data string) error {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return err
	}
	_, err = sess.Write([]byte(data))
	return err
}

func (b *Backend) SendKey(_ context.Context, id, key string) error {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return err
	}
	mapped := mapNamedKey(key)
	_, err = sess.Write([]byte(mapped))
	return err
}

func (b *Backend) Resize(_ context.Context, id string, cols, rows int) error {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return err
	}
	return sess.Resize(uint16(cols), uint16(rows))
}

func (b *Backend) CaptureOutput(_ context.Context, id string, lines int) ([]string, error) {
	b.mu.RLock()
	rb, ok := b.outputBuffers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("pty backend: session %q not found", id)
	}
	data := rb.Bytes()
	allLines := strings.Split(string(data), "\n")
	if lines > 0 && lines < len(allLines) {
		allLines = allLines[len(allLines)-lines:]
	}
	return allLines, nil
}

func (b *Backend) SessionExists(_ context.Context, id string) bool {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return false
	}
	return !sess.closed
}

// Manager exposes the underlying pty.Manager for event wiring.
func (b *Backend) Manager() *Manager {
	return b.manager
}

// Close terminates all sessions.
func (b *Backend) Close() {
	b.manager.Close()
}

// mapNamedKey converts named keys (Enter, C-c, etc.) to their byte sequences.
func mapNamedKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "enter":
		return "\r"
	case "c-c":
		return "\x03"
	case "c-d":
		return "\x04"
	case "c-z":
		return "\x1a"
	case "c-l":
		return "\x0c"
	case "escape", "esc":
		return "\x1b"
	case "tab":
		return "\t"
	case "backspace":
		return "\x7f"
	case "up":
		return "\x1b[A"
	case "down":
		return "\x1b[B"
	case "right":
		return "\x1b[C"
	case "left":
		return "\x1b[D"
	default:
		return key
	}
}

// parseCommand splits a shell command string into argv.
// Handles simple quoting but for complex cases uses sh -c.
func parseCommand(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	// If command contains newlines or pipes, wrap in shell.
	if strings.ContainsAny(command, "\n|&;$`") {
		return []string{"sh", "-c", command}
	}
	return strings.Fields(command)
}

// ringBuf is a simple fixed-size ring buffer for byte data.
type ringBuf struct {
	mu   sync.Mutex
	data []byte
	pos  int
	full bool
	cap  int
}

func newRingBuf(capacity int) *ringBuf {
	return &ringBuf{data: make([]byte, capacity), cap: capacity}
}

func (r *ringBuf) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range p {
		r.data[r.pos] = b
		r.pos = (r.pos + 1) % r.cap
		if r.pos == 0 {
			r.full = true
		}
	}
}

func (r *ringBuf) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		return append([]byte(nil), r.data[:r.pos]...)
	}
	result := make([]byte, r.cap)
	copy(result, r.data[r.pos:])
	copy(result[r.cap-r.pos:], r.data[:r.pos])
	return result
}
```

**Step 2: Verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./internal/pty/...
```

**Step 3: Commit**

```bash
git add internal/pty/backend.go
git commit -m "feat(pty): implement TerminalBackend adapter with ringBuf output capture"
```

---

### Task 8: Wire PTY backend into `main.go` — replace tmux

**Files:**
- Modify: `cmd/agenterm/main.go`

This is the critical switchover. We replace the `tmux.Manager` and `tmux.Gateway` usage with `pty.Backend`, and update `runtimeState` to use PTY sessions instead of tmux gateways.

**Step 1: Update imports — replace `tmux` with `pty`**

In `main.go`, change the import from:
```go
"github.com/user/agenterm/internal/tmux"
```
to:
```go
ptybackend "github.com/user/agenterm/internal/pty"
```

**Step 2: Replace `sessionRuntime` struct**

Change `sessionRuntime` to use PTY session:
```go
type sessionRuntime struct {
	session *ptybackend.Session
	parser  *parser.Parser
}
```

**Step 3: Replace `runtimeState` fields**

Change `runtimeState` to use PTY backend:
```go
type runtimeState struct {
	cfg       *config.Config
	backend   *ptybackend.Backend
	hub       *hub.Hub
	lifecycle *session.Manager
	mu        sync.RWMutex
	sessions  map[string]*sessionRuntime
}
```

**Step 4: Update `ensureSession` to use PTY**

The PTY backend creates sessions on-demand via `session.Manager.CreateSession`. The `ensureSession` in `runtimeState` now looks up from the PTY manager and starts event loops.

**Step 5: Update `startSessionLoops` for PTY events**

Replace the tmux gateway event loop with PTY session event loop:
```go
func (s *runtimeState) startSessionLoops(ctx context.Context, sessionID string, rt *sessionRuntime) {
	go func() {
		for evt := range rt.session.Events() {
			switch evt.Type {
			case ptybackend.EventOutput:
				s.hub.BroadcastTerminal(hub.TerminalDataMessage{
					Type:      "terminal_data",
					SessionID: sessionID,
					Window:    sessionID, // In PTY mode, window ID = session ID
					Text:      evt.Data,
				})
				rt.parser.Feed(sessionID, evt.Data)
			case ptybackend.EventClosed:
				s.hub.BroadcastStatusForSession(sessionID, sessionID, "failed")
				return
			}
		}
	}()

	go func() {
		for msg := range rt.parser.Messages() {
			if s.lifecycle != nil {
				s.lifecycle.ObserveParsedOutput(sessionID, sessionID, msg.Text, string(msg.Class), msg.Timestamp)
			}
			s.hub.BroadcastOutput(hub.OutputMessage{
				Type:      "output",
				SessionID: sessionID,
				Window:    sessionID,
				Text:      msg.Text,
				Class:     string(msg.Class),
				Actions:   convertActions(msg.Actions),
				ID:        msg.ID,
				Ts:        msg.Timestamp.Unix(),
			})
		}
	}()
}
```

**Step 6: Update input/resize handlers**

Replace `sendKeys`, `sendRaw`, `resizeWindow` to use PTY:
```go
func (s *runtimeState) sendKeys(ctx context.Context, sessionID, windowID, keys string) error {
	sess, err := s.backend.Manager().GetSession(sessionID)
	if err != nil {
		return err
	}
	_, err = sess.Write([]byte(keys))
	return err
}

func (s *runtimeState) sendRaw(ctx context.Context, sessionID, windowID, keys string) error {
	return s.sendKeys(ctx, sessionID, windowID, keys)
}

func (s *runtimeState) resizeWindow(ctx context.Context, sessionID, windowID string, cols, rows int) error {
	sess, err := s.backend.Manager().GetSession(sessionID)
	if err != nil {
		return err
	}
	return sess.Resize(uint16(cols), uint16(rows))
}
```

**Step 7: Remove tmux session bootstrap from main()**

Remove the block that creates/attaches the default tmux session (lines 373-388). PTY sessions are created on-demand by `session.Manager.CreateSession`.

**Step 8: Update `session.Manager` constructor call**

Change:
```go
lifecycleManager := session.NewManager(appDB.SQL(), manager, agentRegistry, h)
```
to pass the PTY backend (this requires Task 9 below to update the `session.Manager` to accept `TerminalBackend` instead of `TmuxManager`).

**Step 9: Verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./cmd/agenterm/...
```

**Step 10: Commit**

```bash
git add cmd/agenterm/main.go
git commit -m "feat(pty): wire PTY backend into main.go, replace tmux runtime"
```

---

### Task 9: Update `session.Manager` to use `TerminalBackend` instead of `TmuxManager`

**Files:**
- Modify: `internal/session/manager.go`
- Modify: `internal/session/monitor.go`

**Step 1: Replace `TmuxManager` interface with `TerminalBackend`**

In `manager.go`, change the `tmux` field from `TmuxManager` to `TerminalBackend`:
```go
type Manager struct {
	backend     TerminalBackend
	registry    *registry.Registry
	hub         *hub.Hub
	// ... rest unchanged
}
```

**Step 2: Update `NewManager` to accept `TerminalBackend`**

```go
func NewManager(conn *sql.DB, backend TerminalBackend, reg *registry.Registry, hubInst *hub.Hub) *Manager {
```

**Step 3: Update `CreateSession` to use backend**

Replace tmux session creation with:
```go
windowID, err := sm.backend.CreateSession(ctx, session.ID, agentConfig.Name, agentConfig.Command, workDir)
```

**Step 4: Update `DestroySession` to use backend**

Replace `sm.tmux.DestroySession(...)` with `sm.backend.DestroySession(ctx, sessionID)`.

**Step 5: Update `dispatchCommand` to use backend**

Replace tmux gateway calls with:
- `sm.backend.SendInput(ctx, sessionID, text)` for send_text
- `sm.backend.SendKey(ctx, sessionID, key)` for send_key
- `sm.backend.Resize(ctx, sessionID, cols, rows)` for resize

**Step 6: Update monitor to use `SessionExists` from backend**

In `monitor.go`, replace the tmux session existence check with `backend.SessionExists(ctx, id)`.

**Step 7: Remove tmux import from session package**

**Step 8: Verify compilation**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./...
```

**Step 9: Commit**

```bash
git add internal/session/manager.go internal/session/monitor.go internal/session/backend.go
git commit -m "refactor(session): use TerminalBackend interface, remove direct tmux dependency"
```

---

### Task 10: Update `db.Session` model — replace tmux fields with PTY fields

**Files:**
- Modify: `internal/db/models.go`
- Modify: `internal/db/sessions.go` (repository)
- Modify: DB migration / schema

**Step 1: Update Session model**

Replace `TmuxSessionName` and `TmuxWindowID` with a single `TerminalID` field:
```go
type Session struct {
	ID             string
	TaskID         string
	TerminalID     string    // PTY session ID (replaces TmuxSessionName + TmuxWindowID)
	AgentType      string
	Role           string
	Status         string
	HumanAttached  bool
	CreatedAt      time.Time
	LastActivityAt time.Time
}
```

Keep `TmuxSessionName` and `TmuxWindowID` as computed aliases for backward compatibility in the transition period — they both return `TerminalID`.

**Step 2: Add migration to add `terminal_id` column**

**Step 3: Update SessionRepo CRUD to use new field**

**Step 4: Verify compilation and tests**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go build ./... && go test ./internal/db/... -v -count=1
```

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "refactor(db): replace tmux session fields with terminal_id for PTY migration"
```

---

### Task 11: Update API session handlers for PTY

**Files:**
- Modify: `internal/api/sessions.go`

**Step 1: Update `createSession` to return terminal_id instead of tmux_window_id**

**Step 2: Update `getSessionOutput` to use backend.CaptureOutput**

**Step 3: Update JSON serialization to use `terminal_id` (keep `tmux_window_id` alias for backward compat)**

**Step 4: Verify compilation**

**Step 5: Commit**

```bash
git add internal/api/sessions.go
git commit -m "refactor(api): update session endpoints for PTY backend"
```

---

### Task 12: Update frontend types and WebSocket for PTY

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/components/Workspace.tsx`
- Modify: `frontend/src/hooks/useWebSocket.ts`

**Step 1: Update Session type**

Add `terminal_id` field alongside `tmux_window_id` for backward compat:
```typescript
export interface Session {
  id: string
  task_id: string
  terminal_id?: string       // PTY session ID (new)
  tmux_session_name?: string // deprecated
  tmux_window_id?: string    // deprecated — reads terminal_id
  agent_type: string
  role: string
  status: string
  human_attached: boolean
}
```

**Step 2: Add helper to get terminal window ID**

```typescript
export function getWindowID(session: Session): string {
  return session.terminal_id || session.tmux_window_id || ''
}
```

**Step 3: Update all references from `session.tmux_window_id` to `getWindowID(session)`**

Search and replace across:
- `Workspace.tsx` (refreshWindowSnapshot, projectSessionWindows, onSelectAgent)
- `TerminalGrid.tsx` (session filtering)
- `ProjectSidebar.tsx` (unread counts, agent selection)
- `OrchestratorPanel.tsx` (onOpenTaskSession)

**Step 4: Build frontend**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm/frontend && npm run build
```

**Step 5: Commit**

```bash
git add frontend/src/
git commit -m "refactor(frontend): support terminal_id from PTY backend alongside tmux_window_id"
```

---

### Task 13: Integration test — end-to-end PTY session

**Files:**
- Create: `internal/pty/integration_test.go`

**Step 1: Write integration test**

```go
package pty

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBackendIntegration(t *testing.T) {
	b := NewBackend()
	defer b.Close()

	ctx := context.Background()
	id, err := b.CreateSession(ctx, "int-test", "bash", "bash", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Wait for shell to start.
	time.Sleep(500 * time.Millisecond)

	if !b.SessionExists(ctx, id) {
		t.Fatal("session should exist")
	}

	// Send a command.
	if err := b.SendInput(ctx, id, "echo hello-integration\n"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}

	// Wait for output.
	time.Sleep(500 * time.Millisecond)

	lines, err := b.CaptureOutput(ctx, id, 50)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}

	found := false
	for _, line := range lines {
		if strings.Contains(line, "hello-integration") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected output containing 'hello-integration', got: %v", lines)
	}

	// Resize.
	if err := b.Resize(ctx, id, 200, 50); err != nil {
		t.Errorf("Resize: %v", err)
	}

	// Destroy.
	if err := b.DestroySession(ctx, id); err != nil {
		t.Errorf("DestroySession: %v", err)
	}

	if b.SessionExists(ctx, id) {
		t.Error("session should not exist after destroy")
	}
}
```

**Step 2: Run the test**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go test ./internal/pty/... -v -run Integration -count=1
```

**Step 3: Commit**

```bash
git add internal/pty/integration_test.go
git commit -m "test(pty): add end-to-end integration test for PTY backend"
```

---

### Task 14: Clean up — remove tmux dependency from session path (keep package for reference)

**Files:**
- Modify: `cmd/agenterm/main.go` (remove tmux import if not used)
- Modify: `internal/api/sessions.go` (remove tmux gateway parameter)

Do NOT delete `internal/tmux/` yet — keep as reference. Just ensure it is no longer imported in the active code path.

**Step 1: Verify no tmux imports remain in the session/API/main path**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && grep -r '"github.com/user/agenterm/internal/tmux"' cmd/ internal/session/ internal/api/
```
Expected: no results

**Step 2: Run full test suite**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go test ./... -count=1
```

**Step 3: Commit**

```bash
git add -A
git commit -m "refactor: remove tmux from active session path, PTY migration phase 1 complete"
```

---

## Phase 2 — Frontend Keyboard Shortcuts & Resizable Panes

### Task 15: Add global keyboard shortcut handler

**Files:**
- Create: `frontend/src/hooks/useKeyboardShortcuts.ts`
- Modify: `frontend/src/components/Workspace.tsx`

Minimum shortcuts per TAURI-PREFLIGHT-QUESTIONS.md Q6.4:
- `Cmd+1..9` — switch project
- `Cmd+Shift+A` — open approvals / orchestrator panel
- `Cmd+Shift+T` — jump to active terminal session
- `Cmd+K` — focus orchestrator chat input

**Step 1: Create `useKeyboardShortcuts.ts`**

```typescript
import { useEffect } from 'react'

interface ShortcutActions {
  switchProject: (index: number) => void
  togglePanel: () => void
  focusActiveTerminal: () => void
  focusChatInput: () => void
}

export function useKeyboardShortcuts(actions: ShortcutActions) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const meta = e.metaKey || e.ctrlKey

      // Cmd+1..9 — switch project
      if (meta && !e.shiftKey && e.key >= '1' && e.key <= '9') {
        e.preventDefault()
        actions.switchProject(parseInt(e.key, 10) - 1)
        return
      }

      // Cmd+Shift+A — toggle orchestrator panel
      if (meta && e.shiftKey && e.key.toLowerCase() === 'a') {
        e.preventDefault()
        actions.togglePanel()
        return
      }

      // Cmd+Shift+T — focus active terminal
      if (meta && e.shiftKey && e.key.toLowerCase() === 't') {
        e.preventDefault()
        actions.focusActiveTerminal()
        return
      }

      // Cmd+K — focus chat input
      if (meta && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        actions.focusChatInput()
        return
      }
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [actions])
}
```

**Step 2: Wire into Workspace.tsx**

Add the hook call with callbacks that set state / focus DOM elements.

**Step 3: Build frontend**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm/frontend && npm run build
```

**Step 4: Commit**

```bash
git add frontend/src/hooks/useKeyboardShortcuts.ts frontend/src/components/Workspace.tsx
git commit -m "feat(frontend): add keyboard shortcuts — Cmd+1..9, Cmd+Shift+A/T, Cmd+K"
```

---

### Task 16: Make workspace panes resizable with drag handles

**Files:**
- Modify: `frontend/src/components/Workspace.tsx`
- Modify: `frontend/src/styles/workspace.css`

The sidebar and orchestrator panel widths should be draggable. Use a simple mouse-drag handler (no external dependency).

**Step 1: Add resize state and handlers**

Store `sidebarWidth` and `panelWidth` in state, persist to localStorage. Add `onMouseDown` / `onMouseMove` / `onMouseUp` for drag handles.

**Step 2: Add drag handle elements between sidebar/main and main/panel**

```tsx
<div className="resize-handle resize-handle-left" onMouseDown={startSidebarResize} />
<div className="resize-handle resize-handle-right" onMouseDown={startPanelResize} />
```

**Step 3: Add CSS for drag handles**

```css
.resize-handle {
  width: 4px;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
  flex-shrink: 0;
}
.resize-handle:hover,
.resize-handle.active {
  background: var(--accent);
}
```

**Step 4: Build and verify**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm/frontend && npm run build
```

**Step 5: Commit**

```bash
git add frontend/src/components/Workspace.tsx frontend/src/styles/workspace.css
git commit -m "feat(frontend): add draggable resize handles for sidebar and panel"
```

---

## Phase 3 — Missing Workflow Stages & Orchestrator Adjustments

### Task 17: Add brainstorm stage to orchestrator toolset

**Files:**
- Modify: `internal/orchestrator/toolset.go` (or equivalent)
- Modify: `internal/orchestrator/system_prompt.go` (or equivalent)

The orchestrator currently supports `plan`, `build`, `test` stages. Add `brainstorm` as the first stage per DECISION-vnext.md. The brainstorm stage should:
1. Generate 3-5 solutions with design motivations
2. Present to user for selection
3. Produce a design document as artifact

**Step 1: Explore orchestrator toolset to find where stages are defined**

**Step 2: Add `brainstorm` stage with appropriate tools and prompts**

**Step 3: Verify compilation and run orchestrator tests**

**Step 4: Commit**

```bash
git add internal/orchestrator/
git commit -m "feat(orchestrator): add brainstorm stage with design doc generation"
```

---

### Task 18: Add summarize stage to orchestrator toolset

**Files:**
- Modify: `internal/orchestrator/toolset.go`

The `summarize` stage runs after test, per DECISION-vnext.md. It should:
1. Collect all artifacts from previous stages
2. Generate a summary of what was done
3. Mark the workflow as complete

**Step 1: Add `summarize` stage to orchestrator**

**Step 2: Verify compilation**

**Step 3: Commit**

```bash
git add internal/orchestrator/
git commit -m "feat(orchestrator): add summarize stage for workflow completion"
```

---

### Task 19: Update StagePipeline component for all 5 stages

**Files:**
- Modify: `frontend/src/components/StagePipeline.tsx`

The frontend `StagePipeline` already defines `const STAGES = ['brainstorm', 'plan', 'build', 'test', 'summarize']` but the backend may not emit all phases. Verify the pipeline renders correctly for all 5 stages.

**Step 1: Verify StagePipeline renders all 5 stages with correct active state**

**Step 2: Add per-stage description tooltips**

**Step 3: Build frontend**

**Step 4: Commit**

```bash
git add frontend/src/components/StagePipeline.tsx
git commit -m "feat(frontend): verify stage pipeline renders all 5 workflow stages"
```

---

### Task 20: Implement review loop with max 20 iterations and escalation

**Files:**
- Modify: `internal/orchestrator/` (review loop logic)

Per DECISION-vnext.md #8:
- Build-review loop: max 20 iterations
- On limit reached: auto-escalate to human with diagnostic bundle, block lane

**Step 1: Find where review loops are tracked in orchestrator**

**Step 2: Add iteration counter and escalation logic**

**Step 3: Add tests for loop termination**

**Step 4: Commit**

```bash
git add internal/orchestrator/
git commit -m "feat(orchestrator): enforce 20-iteration review loop limit with human escalation"
```

---

### Task 21: Implement escalation policy — orchestrator handles first, then human notice

**Files:**
- Modify: `internal/orchestrator/` (exception handling)

Per DECISION-vnext.md #7:
- First step: orchestrator auto-handles
- If still stuck: send notice and wait for human reply

**Step 1: Add escalation levels to exception handling**

**Step 2: Wire notification to hub for frontend display**

**Step 3: Commit**

```bash
git add internal/orchestrator/
git commit -m "feat(orchestrator): add two-tier escalation — auto-handle then human notice"
```

---

### Task 22: Adjust orchestrator role to auxiliary coordinator

**Files:**
- Modify: `internal/orchestrator/system_prompt.go`

Per user decision: orchestrator is 辅助角色 (auxiliary coordinator). TUI agents are primary actors. Orchestrator coordinates and pushes workflow, but TUI tells orchestrator when finished.

**Step 1: Update system prompt to reflect auxiliary role**

Key behavioral changes:
- Orchestrator proposes but does not force
- TUI agent signals completion (via `.orchestra/done` marker or `[READY_FOR_REVIEW]` commit)
- Orchestrator monitors and nudges, doesn't micro-manage

**Step 2: Verify no hardcoded "orchestrator drives everything" assumptions**

**Step 3: Commit**

```bash
git add internal/orchestrator/
git commit -m "refactor(orchestrator): adjust role to auxiliary coordinator per user decision"
```

---

## Phase 4 — Demand Pool CRUD, Project Delete & Polish

### Task 23: Add demand pool CRUD to frontend

**Files:**
- Modify: `frontend/src/components/OrchestratorPanel.tsx`

The API endpoints for demand pool already exist (`listDemandPoolItems`, `createDemandPoolItem`, `updateDemandPoolItem`, `deleteDemandPoolItem`, `promoteDemandPoolItem`). The frontend `OrchestratorPanel` has a read-only demand section. Add:
1. "Add Item" button → inline form (title, description, priority)
2. Edit item → inline edit
3. Delete item → confirm + delete
4. Promote to task → call `promoteDemandPoolItem`

**Step 1: Add demand pool CRUD UI to OrchestratorPanel**

**Step 2: Build frontend**

**Step 3: Commit**

```bash
git add frontend/src/components/OrchestratorPanel.tsx
git commit -m "feat(frontend): add demand pool CRUD — create, edit, delete, promote"
```

---

### Task 24: Add project delete to sidebar

**Files:**
- Modify: `frontend/src/components/ProjectSidebar.tsx`
- Modify: `frontend/src/components/Workspace.tsx`

The delete modal already exists in Workspace.tsx but there's no trigger in the sidebar.

**Step 1: Add context menu or delete button to project items in sidebar**

**Step 2: Wire to existing `setDeleteModalOpen(true)` in Workspace**

**Step 3: Build frontend**

**Step 4: Commit**

```bash
git add frontend/src/components/ProjectSidebar.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(frontend): add project delete trigger to sidebar"
```

---

### Task 25: Add playbook selector to CreateProjectFlow

**Files:**
- Modify: `frontend/src/components/CreateProjectFlow.tsx`

The API has `listPlaybooks`. Add a dropdown to the project creation wizard so users can select a playbook template.

**Step 1: Fetch playbooks on mount**

**Step 2: Add select dropdown to form**

**Step 3: Pass selected playbook ID to `createProject`**

**Step 4: Build frontend**

**Step 5: Commit**

```bash
git add frontend/src/components/CreateProjectFlow.tsx
git commit -m "feat(frontend): add playbook selector to project creation wizard"
```

---

### Task 26: Final integration test — start backend, verify PTY sessions from frontend

**Files:**
- No new files; manual verification

**Step 1: Start the Go backend**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go run ./cmd/agenterm --port 8765 --print-token
```

**Step 2: Start the frontend dev server**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm/frontend && npm run dev
```

**Step 3: Verify in browser:**
- Create a project
- Sessions spawn with PTY (no tmux errors in backend log)
- Terminal displays output correctly
- Input works (typing, Enter, Ctrl-C)
- Resize works
- Keyboard shortcuts work
- Stage pipeline displays all 5 stages
- Demand pool CRUD works
- Project delete works

**Step 4: Run full test suites**

Run:
```bash
cd /Users/houyuxin/08Coding/agenterm && go test ./... -count=1
cd /Users/houyuxin/08Coding/agenterm/frontend && npm run build
```

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix: integration test cleanup for PTY migration"
```

---

## Summary

| Phase | Tasks | Focus |
|-------|-------|-------|
| 1 | 1–14 | PTY runtime: `creack/pty` package, backend interface, `session.Manager` rewire, DB model update, frontend type update |
| 2 | 15–16 | Keyboard shortcuts (`Cmd+1..9`, `Cmd+Shift+A/T`, `Cmd+K`) and draggable resize handles |
| 3 | 17–22 | Brainstorm + summarize stages, review loop limit, escalation policy, orchestrator role adjustment |
| 4 | 23–26 | Demand pool CRUD, project delete from sidebar, playbook selector, integration test |
