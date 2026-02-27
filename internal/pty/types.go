package pty

import "time"

// EventType distinguishes the kind of event produced by a Session.
type EventType int

const (
	// EventOutput indicates that new data was read from the PTY.
	EventOutput EventType = iota
	// EventClosed indicates that the child process has exited.
	EventClosed
)

// Event is a single notification emitted by a Session.
type Event struct {
	Type EventType
	ID   string
	Data string
}

// SessionInfo is a read-only snapshot of session metadata returned by Manager.ListSessions.
type SessionInfo struct {
	ID        string
	Name      string
	Active    bool
	CreatedAt time.Time
}
