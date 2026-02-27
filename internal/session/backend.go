package session

import "context"

// TerminalBackend abstracts the terminal runtime (tmux or PTY).
type TerminalBackend interface {
	// CreateSession spawns a new terminal session.
	// command is the shell command string to execute.
	// Returns the session/terminal ID.
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
