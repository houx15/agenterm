package tmux

import (
	"testing"
)

func TestDecodeOctal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "newline",
			input:    "\\012",
			expected: "\n",
		},
		{
			name:     "ANSI red",
			input:    "\\033[31m",
			expected: "\x1b[31m",
		},
		{
			name:     "mixed text",
			input:    "Hello\\012World",
			expected: "Hello\nWorld",
		},
		{
			name:     "no escapes",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "multiple escapes",
			input:    "\\033[32mgreen\\033[0m\\012",
			expected: "\x1b[32mgreen\x1b[0m\n",
		},
		{
			name:     "backslash without octal",
			input:    "\\not_escape",
			expected: "\\not_escape",
		},
		{
			name:     "incomplete octal",
			input:    "\\01",
			expected: "\\01",
		},
		{
			name:     "carriage return",
			input:    "\\015",
			expected: "\r",
		},
		{
			name:     "tab",
			input:    "\\011",
			expected: "\t",
		},
		{
			name:     "null byte",
			input:    "\\000",
			expected: "\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeOctal(tt.input)
			if result != tt.expected {
				t.Errorf("DecodeOctal(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		eventType   EventType
		windowID    string
		paneID      string
		data        string
	}{
		{
			name:        "output event",
			input:       "%output %0 Hello\\012World",
			expectError: false,
			eventType:   EventOutput,
			paneID:      "%0",
			data:        "Hello\nWorld",
		},
		{
			name:        "window-add event",
			input:       "%window-add @0",
			expectError: false,
			eventType:   EventWindowAdd,
			windowID:    "@0",
		},
		{
			name:        "window-close event",
			input:       "%window-close @1",
			expectError: false,
			eventType:   EventWindowClose,
			windowID:    "@1",
		},
		{
			name:        "window-renamed event",
			input:       "%window-renamed @0 new-name",
			expectError: false,
			eventType:   EventWindowRenamed,
			windowID:    "@0",
			data:        "new-name",
		},
		{
			name:        "layout-change event",
			input:       "%layout-change @0 layout-info",
			expectError: false,
			eventType:   EventLayoutChange,
			windowID:    "@0",
			data:        "layout-info",
		},
		{
			name:        "begin event",
			input:       "%begin 1234567890 0 1",
			expectError: false,
			eventType:   EventBegin,
		},
		{
			name:        "end event",
			input:       "%end 1234567890 0 1",
			expectError: false,
			eventType:   EventEnd,
		},
		{
			name:        "error event",
			input:       "%error 1234567890 0 1",
			expectError: false,
			eventType:   EventError,
		},
		{
			name:        "plain line (command response)",
			input:       "@0 main 1",
			expectError: false,
			eventType:   EventOutput,
			data:        "@0 main 1",
		},
		{
			name:        "empty line",
			input:       "",
			expectError: true,
		},
		{
			name:        "unknown percent line",
			input:       "%unknown",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseLine(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if event.Type != tt.eventType {
				t.Errorf("event type = %v, want %v", event.Type, tt.eventType)
			}
			if event.WindowID != tt.windowID {
				t.Errorf("windowID = %q, want %q", event.WindowID, tt.windowID)
			}
			if event.PaneID != tt.paneID {
				t.Errorf("paneID = %q, want %q", event.PaneID, tt.paneID)
			}
			if tt.data != "" && event.Data != tt.data {
				t.Errorf("data = %q, want %q", event.Data, tt.data)
			}
		})
	}
}

func TestParseOutputDecoding(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple output",
			input:    "%output %0 hello",
			expected: "hello",
		},
		{
			name:     "output with newline",
			input:    "%output %0 hello\\012",
			expected: "hello\n",
		},
		{
			name:     "output with ANSI codes",
			input:    "%output %0 \\033[31mred\\033[0m",
			expected: "\x1b[31mred\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseLine(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if event.Data != tt.expected {
				t.Errorf("data = %q (len=%d), want %q (len=%d)", event.Data, len(event.Data), tt.expected, len(tt.expected))
			}
		})
	}
}

func TestParseWindowRenamedWithSpaces(t *testing.T) {
	input := "%window-renamed @0 my window name"
	event, err := ParseLine(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.WindowID != "@0" {
		t.Errorf("windowID = %q, want @0", event.WindowID)
	}
	if event.Data != "my window name" {
		t.Errorf("data = %q, want 'my window name'", event.Data)
	}
}
