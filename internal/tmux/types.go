package tmux

type EventType int

const (
	EventOutput EventType = iota
	EventWindowAdd
	EventWindowClose
	EventWindowRenamed
	EventLayoutChange
	EventBegin
	EventEnd
	EventError
)

type Event struct {
	Type     EventType
	WindowID string
	PaneID   string
	Data     string
	Raw      string
}

type Window struct {
	ID     string
	Name   string
	Active bool
}
