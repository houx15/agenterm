package tmux

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestGatewayNew(t *testing.T) {
	g := New("test-session")
	if g == nil {
		t.Fatal("New returned nil")
	}
	if g.session != "test-session" {
		t.Errorf("session = %q, want 'test-session'", g.session)
	}
}

func TestGatewayListWindowsEmpty(t *testing.T) {
	g := New("test")
	windows := g.ListWindows()
	if len(windows) != 0 {
		t.Errorf("ListWindows() = %v, want empty", windows)
	}
}

func TestGatewayEventsChannel(t *testing.T) {
	g := New("test")
	events := g.Events()
	if events == nil {
		t.Error("Events() returned nil channel")
	}
}

func TestIntegrationGateway(t *testing.T) {
	if os.Getenv("TMUX_INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set TMUX_INTEGRATION_TEST=1 to run")
	}

	sessionName := "agenterm_test_" + time.Now().Format("20060102150405")

	createCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := createCmd.Run(); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	g := New(sessionName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := g.Start(ctx); err != nil {
		t.Fatalf("failed to start gateway: %v", err)
	}
	defer g.Stop()

	time.Sleep(100 * time.Millisecond)

	windows := g.ListWindows()
	if len(windows) == 0 {
		t.Error("ListWindows() returned no windows")
	}

	found := false
	for _, w := range windows {
		if strings.Contains(w.Name, sessionName) || w.Name != "" {
			found = true
			t.Logf("found window: %+v", w)
			break
		}
	}
	if !found {
		t.Logf("windows: %+v", windows)
	}

	if err := g.SendKeys(windows[0].ID, "echo hello"); err != nil {
		t.Errorf("SendKeys failed: %v", err)
	}

	select {
	case event := <-g.Events():
		t.Logf("received event: %+v", event)
	case <-time.After(2 * time.Second):
		t.Log("no event received within timeout")
	}
}

func TestGatewayNonExistentSession(t *testing.T) {
	if os.Getenv("TMUX_INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set TMUX_INTEGRATION_TEST=1 to run")
	}

	g := New("nonexistent_session_12345")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := g.Start(ctx)
	if err == nil {
		g.Stop()
		t.Fatal("expected error for non-existent session")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message should mention 'not found': %v", err)
	}
}

func TestGatewayEscapeKeys(t *testing.T) {
	g := &Gateway{}

	tests := []struct {
		input    string
		expected string
	}{
		{"Enter", "Enter"},
		{"\n", "Enter"},
		{"\x03", "C-c"},
		{"hello", "hello"},
		{"hello\nworld", "'hello Enter world'"},
	}

	for _, tt := range tests {
		result := g.escapeKeys(tt.input)
		if result != tt.expected {
			t.Errorf("escapeKeys(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGatewaySendKeysNotStarted(t *testing.T) {
	g := New("test")
	err := g.SendKeys("@0", "hello")
	if err == nil {
		t.Error("expected error when SendKeys called before Start")
	}
}
