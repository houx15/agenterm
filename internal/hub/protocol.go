package hub

type ServerMessage struct {
	Type string `json:"type"`
}

type OutputMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Window    string          `json:"window"`
	Text      string          `json:"text"`
	Class     string          `json:"class"`
	Actions   []ActionMessage `json:"actions,omitempty"`
	ID        string          `json:"id"`
	Ts        int64           `json:"ts"`
}

type TerminalDataMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Window    string `json:"window"`
	Text      string `json:"text"`
}

type ActionMessage struct {
	Label string `json:"label"`
	Keys  string `json:"keys"`
}

type WindowsMessage struct {
	Type string       `json:"type"`
	List []WindowInfo `json:"list"`
}

type WindowInfo struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id,omitempty"`
	Name      string `json:"name"`
	Status    string `json:"status"`
}

type StatusMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Window    string `json:"window"`
	Status    string `json:"status"`
}

type ClientMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Window    string `json:"window"`
	Keys      string `json:"keys"`
	Name      string `json:"name,omitempty"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

type OrchestratorClientMessage struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type OrchestratorServerMessage struct {
	Type   string         `json:"type"`
	Text   string         `json:"text,omitempty"`
	Name   string         `json:"name,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
	Result any            `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type ProjectEventMessage struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id"`
	Event     string `json:"event"`
	Data      any    `json:"data,omitempty"`
	Ts        int64  `json:"ts"`
}

type NewWindowMessage struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type hubBroadcast struct {
	data      []byte
	sessionID string
}

type ErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
