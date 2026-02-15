package parser

import "time"

type MessageClass string

const (
	ClassNormal MessageClass = "normal"
	ClassPrompt MessageClass = "prompt"
	ClassError  MessageClass = "error"
	ClassCode   MessageClass = "code"
	ClassSystem MessageClass = "system"
)

type QuickAction struct {
	Label string
	Keys  string
}

type Message struct {
	ID        string
	WindowID  string
	Text      string
	RawText   string
	Class     MessageClass
	Actions   []QuickAction
	Timestamp time.Time
}

type WindowStatus string

const (
	StatusWorking WindowStatus = "working"
	StatusWaiting WindowStatus = "waiting"
	StatusIdle    WindowStatus = "idle"
)
