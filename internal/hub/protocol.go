package hub

type ServerMessage struct {
	Type string `json:"type"`
}

type OutputMessage struct {
	Type    string          `json:"type"`
	Window  string          `json:"window"`
	Text    string          `json:"text"`
	Class   string          `json:"class"`
	Actions []ActionMessage `json:"actions,omitempty"`
	ID      string          `json:"id"`
	Ts      int64           `json:"ts"`
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
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type StatusMessage struct {
	Type   string `json:"type"`
	Window string `json:"window"`
	Status string `json:"status"`
}

type ClientMessage struct {
	Type   string `json:"type"`
	Window string `json:"window"`
	Keys   string `json:"keys"`
	Name   string `json:"name,omitempty"`
}

type NewWindowMessage struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type ErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
