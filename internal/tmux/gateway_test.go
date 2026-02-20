package tmux

import (
	"context"
	"os"
	"os/exec"
	"sync"
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
	if got := g.SessionName(); got != "test-session" {
		t.Errorf("SessionName() = %q, want test-session", got)
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
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single enter",
			input:    "\n",
			expected: []string{"send-keys -t @1 Enter\n"},
		},
		{
			name:     "single ctrl-c",
			input:    "\x03",
			expected: []string{"send-keys -t @1 C-c\n"},
		},
		{
			name:     "simple text",
			input:    "hello",
			expected: []string{"send-keys -t @1 -l -- 'hello'\n"},
		},
		{
			name:     "text plus enter",
			input:    "hello\n",
			expected: []string{"send-keys -t @1 -l -- 'hello'\n", "send-keys -t @1 Enter\n"},
		},
		{
			name:     "multiline text",
			input:    "hello\nworld",
			expected: []string{"send-keys -t @1 -l -- 'hello'\n", "send-keys -t @1 Enter\n", "send-keys -t @1 -l -- 'world'\n"},
		},
		{
			name:     "escaped quote",
			input:    "it's",
			expected: []string{"send-keys -t @1 -l -- 'it'\\''s'\n"},
		},
	}

	for _, tt := range tests {
		result := g.buildSendKeysCommands("@1", tt.input)
		if len(result) != len(tt.expected) {
			t.Fatalf("%s: got %d commands, want %d", tt.name, len(result), len(tt.expected))
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("%s: command %d = %q, want %q", tt.name, i, result[i], tt.expected[i])
			}
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

func TestGatewaySendRawNotStarted(t *testing.T) {
	g := New("test")
	err := g.SendRaw("@0", "hello")
	if err == nil {
		t.Error("expected error when SendRaw called before Start")
	}
}

func TestGatewaySendRawUsesEnterForNewlines(t *testing.T) {
	g := New("test")
	capture := &captureWriteCloser{}
	g.stdin = capture

	if err := g.SendRaw("@1", "hello\nworld\r"); err != nil {
		t.Fatalf("SendRaw returned error: %v", err)
	}

	got := capture.String()
	if !strings.Contains(got, "send-keys -t @1 Enter\n") {
		t.Fatalf("expected Enter key command in output, got: %q", got)
	}
	if strings.Contains(got, " -H 0a") || strings.Contains(got, " -H 0d") {
		t.Fatalf("expected newline bytes to map to Enter key, got: %q", got)
	}
}

func TestGatewayResizeWindowNotStarted(t *testing.T) {
	g := New("test")
	err := g.ResizeWindow("@0", 120, 40)
	if err == nil {
		t.Error("expected error when ResizeWindow called before Start")
	}
}

func TestGatewayResizeWindowInvalidSize(t *testing.T) {
	g := New("test")
	g.stdin = nopWriteCloser{}
	err := g.ResizeWindow("@0", 0, 40)
	if err == nil {
		t.Error("expected error for invalid cols")
	}
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (n int, err error) { return len(p), nil }
func (nopWriteCloser) Close() error                      { return nil }

type captureWriteCloser struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (c *captureWriteCloser) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *captureWriteCloser) Close() error { return nil }

func (c *captureWriteCloser) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func TestGatewayReaderHandlesWindowEventsInsideCommandBlock(t *testing.T) {
	g := New("test")
	input := strings.NewReader("%begin 1 1 0\n%window-add @2\n%window-close @1\n%end 1 1 0\n")

	g.mu.Lock()
	g.windows["@1"] = &Window{ID: "@1", Name: "old"}
	g.mu.Unlock()

	g.wg.Add(1)
	go g.reader(input)

	select {
	case <-g.done:
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not finish")
	}

	windows := g.ListWindows()
	hasOld := false
	hasNew := false
	for _, w := range windows {
		if w.ID == "@1" {
			hasOld = true
		}
		if w.ID == "@2" {
			hasNew = true
		}
	}

	if hasOld {
		t.Error("expected @1 to be removed after window-close event")
	}
	if !hasNew {
		t.Error("expected @2 to be present after window-add event")
	}
}

func TestParseWindowsOutput(t *testing.T) {
	output := "@1\tmain\t1\n@2\tprobe-window\t0\ninvalid\tline\n"
	parsed := parseWindowsOutput(output)

	if len(parsed) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(parsed))
	}
	if parsed["@2"] == nil || parsed["@2"].Name != "probe-window" {
		t.Fatalf("expected @2 name probe-window, got %+v", parsed["@2"])
	}
	if !parsed["@1"].Active {
		t.Fatal("expected @1 active=true")
	}
}

func TestParsePanesOutput(t *testing.T) {
	output := "%10\t@1\n%11\t@2\n%bad\t@3\n"
	parsed := parsePanesOutput(output)

	if len(parsed) != 2 {
		t.Fatalf("expected 2 pane mappings, got %d", len(parsed))
	}
	if parsed["%10"] != "@1" {
		t.Fatalf("expected %%10 -> @1, got %q", parsed["%10"])
	}
}
