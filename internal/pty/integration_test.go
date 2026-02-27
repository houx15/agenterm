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

	// Create a bash session
	id, err := b.CreateSession(ctx, "int-test", "bash", "bash", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Wait for shell to start
	time.Sleep(500 * time.Millisecond)

	if !b.SessionExists(ctx, id) {
		t.Fatal("session should exist")
	}

	// Verify Events channel is available
	events := b.Events(id)
	if events == nil {
		t.Fatal("Events channel should not be nil")
	}

	// Send a command
	if err := b.SendInput(ctx, id, "echo hello-integration\n"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}

	// Wait for output to accumulate
	time.Sleep(500 * time.Millisecond)

	// Capture output
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

	// Test SendKey (Enter)
	if err := b.SendKey(ctx, id, "Enter"); err != nil {
		t.Errorf("SendKey: %v", err)
	}

	// Test Resize
	if err := b.Resize(ctx, id, 200, 50); err != nil {
		t.Errorf("Resize: %v", err)
	}

	// Destroy
	if err := b.DestroySession(ctx, id); err != nil {
		t.Errorf("DestroySession: %v", err)
	}

	if b.SessionExists(ctx, id) {
		t.Error("session should not exist after destroy")
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"bash", []string{"bash"}},
		{"echo hello", []string{"echo", "hello"}},
		{"sh -c 'echo hello'", []string{"sh", "-c", "'echo", "hello'"}}, // simple split
		{"echo hello | grep hello", []string{"sh", "-c", "echo hello | grep hello"}}, // pipes trigger sh -c
		{"cd /tmp\nls", []string{"sh", "-c", "cd /tmp\nls"}},                         // newlines trigger sh -c
		{"", nil},
	}
	for _, tt := range tests {
		result := parseCommand(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseCommand(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("parseCommand(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestMapNamedKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"Enter", "\r"},
		{"C-c", "\x03"},
		{"C-d", "\x04"},
		{"escape", "\x1b"},
		{"tab", "\t"},
		{"up", "\x1b[A"},
		{"down", "\x1b[B"},
		{"left", "\x1b[D"},
		{"right", "\x1b[C"},
		{"backspace", "\x7f"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		result := mapNamedKey(tt.key)
		if result != tt.expected {
			t.Errorf("mapNamedKey(%q) = %q, want %q", tt.key, result, tt.expected)
		}
	}
}
