package session

import (
	"path/filepath"
	"testing"

	"github.com/user/agenterm/internal/registry"
)

func TestIsComposeInputState(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "", want: false},
		{text: "normal shell prompt", want: false},
		{text: "ctrl+gtoeditinVim", want: true},
		{text: "Press ctrl+g to edit in vim", want: true},
	}
	for _, tc := range cases {
		got := isComposeInputState(tc.text)
		if got != tc.want {
			t.Fatalf("isComposeInputState(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}

func TestRequiresPromptReadyForClaude(t *testing.T) {
	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if err := reg.Save(&registry.AgentConfig{
		ID:    "claude-code",
		Name:  "Claude Code",
		Model: "sonnet",
		Command: "claude --model sonnet",
	}); err != nil {
		t.Fatalf("save claude agent: %v", err)
	}
	if err := reg.Save(&registry.AgentConfig{
		ID:    "codex",
		Name:  "Codex",
		Model: "codex",
		Command: "codex",
	}); err != nil {
		t.Fatalf("save codex agent: %v", err)
	}
	mgr := &Manager{registry: reg}
	if !mgr.requiresPromptReady("claude-code") {
		t.Fatalf("expected requiresPromptReady to be true for claude-code")
	}
	if mgr.requiresPromptReady("codex") {
		t.Fatalf("expected requiresPromptReady to be false for codex")
	}
}

func TestNormalizeSessionCommandText(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "ls", want: "ls"},
		{in: "ls\n", want: "ls\r"},
		{in: "line1\nline2\n", want: "line1\nline2\r"},
		{in: "line1\r\nline2\r\n", want: "line1\nline2\r"},
		{in: "\n", want: "\r"},
	}
	for _, tc := range cases {
		got := normalizeSessionCommandText(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeSessionCommandText(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsClaudeLandingState(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "", want: false},
		{text: "normal shell prompt", want: false},
		{text: "/ide forCursor", want: true},
		{text: "❯ Try\"fixlint\"", want: true},
	}
	for _, tc := range cases {
		got := isClaudeLandingState(tc.text)
		if got != tc.want {
			t.Fatalf("isClaudeLandingState(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}

func TestValidateControlKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "enter", want: "C-m"},
		{in: "C-m", want: "C-m"},
		{in: "ctrl+m", want: "C-m"},
		{in: "c-c", want: "C-c"},
		{in: "escape", want: "Escape"},
		{in: "tab", want: "Tab"},
		{in: "unknown", want: ""},
	}
	for _, tc := range cases {
		got := ValidateControlKey(tc.in)
		if got != tc.want {
			t.Fatalf("ValidateControlKey(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
