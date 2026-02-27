package pty

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

const captureBufferSize = 256 * 1024

// Backend wraps a Manager to satisfy session.TerminalBackend.
// It captures output in per-session ring buffers and re-broadcasts
// session events so that external consumers (e.g. main.go / hub)
// can read them without competing with the internal capture loop.
type Backend struct {
	manager       *Manager
	mu            sync.RWMutex
	outputBuffers map[string]*ringBuf
	eventChans    map[string]chan Event // broadcast channels for external consumers
}

// NewBackend creates a Backend with a fresh Manager.
func NewBackend() *Backend {
	return &Backend{
		manager:       NewManager(),
		outputBuffers: make(map[string]*ringBuf),
		eventChans:    make(map[string]chan Event),
	}
}

// CreateSession spawns a new PTY session and starts a goroutine that
// reads from the session's Events channel, writes output to a ring buffer,
// and forwards all events to a broadcast channel returned by Events().
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
	broadcast := make(chan Event, 1024)

	b.mu.Lock()
	b.outputBuffers[id] = rb
	b.eventChans[id] = broadcast
	b.mu.Unlock()

	// Fan-out: read from session events, write to ringBuf + broadcast channel.
	go func() {
		for evt := range sess.Events() {
			if evt.Type == EventOutput {
				rb.Write([]byte(evt.Data))
			}
			// Non-blocking send to broadcast channel.
			select {
			case broadcast <- evt:
			default:
				// Drop if consumer is slow.
			}
		}
		close(broadcast)
	}()

	return id, nil
}

// Events returns the broadcast event channel for a session.
// External consumers (e.g. main.go) read from this to forward
// terminal data to the hub.
func (b *Backend) Events(id string) <-chan Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.eventChans[id]
}

// DestroySession removes the session's ring buffer and broadcast channel,
// then delegates to the underlying Manager.
func (b *Backend) DestroySession(_ context.Context, id string) error {
	b.mu.Lock()
	delete(b.outputBuffers, id)
	delete(b.eventChans, id)
	b.mu.Unlock()
	return b.manager.DestroySession(id)
}

// SendInput writes raw bytes (user keystrokes) to the terminal.
func (b *Backend) SendInput(_ context.Context, id, data string) error {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return err
	}
	_, err = sess.Write([]byte(data))
	return err
}

// SendKey translates a named key (e.g. "Enter", "C-c") to its escape
// sequence and writes it to the terminal.
func (b *Backend) SendKey(_ context.Context, id, key string) error {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return err
	}
	mapped := mapNamedKey(key)
	_, err = sess.Write([]byte(mapped))
	return err
}

// Resize changes the PTY dimensions.
func (b *Backend) Resize(_ context.Context, id string, cols, rows int) error {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return err
	}
	return sess.Resize(uint16(cols), uint16(rows))
}

// CaptureOutput returns the last N lines of terminal output from the
// session's ring buffer.
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

// SessionExists returns true if the session is still alive.
func (b *Backend) SessionExists(_ context.Context, id string) bool {
	sess, err := b.manager.GetSession(id)
	if err != nil {
		return false
	}
	return !sess.IsClosed()
}

// Manager returns the underlying PTY Manager for direct access if needed.
func (b *Backend) Manager() *Manager {
	return b.manager
}

// Close terminates all sessions managed by this backend.
func (b *Backend) Close() {
	b.manager.Close()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mapNamedKey translates a human-readable key name to its terminal byte
// sequence. Unknown names are returned as-is.
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

// parseCommand splits a shell command string into argv. If the command
// contains shell metacharacters it is wrapped with "sh -c ...".
func parseCommand(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	if strings.ContainsAny(command, "\n|&;$`") {
		return []string{"sh", "-c", command}
	}
	return strings.Fields(command)
}

// ---------------------------------------------------------------------------
// ringBuf — fixed-size circular buffer for byte data
// ---------------------------------------------------------------------------

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

// Write appends p to the ring buffer, overwriting oldest data when full.
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

// Bytes returns a copy of the buffered data in chronological order.
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
