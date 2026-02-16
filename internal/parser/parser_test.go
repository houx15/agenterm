package parser

import (
	"strings"
	"testing"
	"time"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no ANSI codes",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "color codes SGR",
			input:    "\x1b[31mred text\x1b[0m",
			expected: "red text",
		},
		{
			name:     "multiple color codes",
			input:    "\x1b[1;32;40mbold green\x1b[0m normal",
			expected: "bold green normal",
		},
		{
			name:     "cursor movement",
			input:    "\x1b[2J\x1b[Hclear screen",
			expected: "clear screen",
		},
		{
			name:     "OSC sequence with bell",
			input:    "\x1b]0;window title\x07text",
			expected: "text",
		},
		{
			name:     "OSC sequence with ST",
			input:    "\x1b]0;title\x1b\\text",
			expected: "text",
		},
		{
			name:     "carriage return removal",
			input:    "line1\r\nline2\r",
			expected: "line1\nline2",
		},
		{
			name:     "mixed sequences",
			input:    "\x1b[1m\x1b]0;title\x07bold\x1b[0m\r\nnext",
			expected: "bold\nnext",
		},
		{
			name:     "charset selection",
			input:    "\x1b(Btext\x1b)0more",
			expected: "textmore",
		},
		{
			name:     "private mode and keypad mode",
			input:    "\x1b[?1h\x1b=\x1b[?2004htext\x1b[?2004l\x1b[?1l\x1b>",
			expected: "text",
		},
		{
			name:     "old title sequence",
			input:    "\x1bk..ding/agenterm\x1b\\hello",
			expected: "hello",
		},
		{
			name:     "backspace cleanup",
			input:    "e\becho",
			expected: "echo",
		},
		{
			name:     "remove other control bytes",
			input:    "a\x00b\x1fc",
			expected: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripANSI(tt.input)
			if result != tt.expected {
				t.Errorf("StripANSI() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestClassification(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedClass  MessageClass
		expectedAction int
	}{
		{
			name:           "Y/n prompt",
			input:          "Do you want to proceed? [Y/n]",
			expectedClass:  ClassPrompt,
			expectedAction: 3,
		},
		{
			name:           "y/N prompt",
			input:          "Continue? [y/N]",
			expectedClass:  ClassPrompt,
			expectedAction: 3,
		},
		{
			name:           "generic question",
			input:          "Are you sure?",
			expectedClass:  ClassPrompt,
			expectedAction: 2,
		},
		{
			name:           "error message",
			input:          "Error: file not found",
			expectedClass:  ClassError,
			expectedAction: 0,
		},
		{
			name:           "fatal error",
			input:          "FATAL: connection refused",
			expectedClass:  ClassError,
			expectedAction: 0,
		},
		{
			name:           "panic",
			input:          "panic: runtime error",
			expectedClass:  ClassError,
			expectedAction: 0,
		},
		{
			name:           "python traceback",
			input:          "Traceback (most recent call last):\n  File \"test.py\", line 1",
			expectedClass:  ClassError,
			expectedAction: 0,
		},
		{
			name:           "code fence",
			input:          "```python\nprint('hello')\n```",
			expectedClass:  ClassCode,
			expectedAction: 0,
		},
		{
			name:           "indented code",
			input:          "    line 1\n    line 2\n    line 3",
			expectedClass:  ClassCode,
			expectedAction: 0,
		},
		{
			name:           "normal text",
			input:          "This is normal output",
			expectedClass:  ClassNormal,
			expectedAction: 0,
		},
		{
			name:           "numbered choices",
			input:          "1. Option one\n2. Option two\n3. Option three",
			expectedClass:  ClassPrompt,
			expectedAction: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, actions := (&Parser{}).classify(tt.input)
			if class != tt.expectedClass {
				t.Errorf("classify() class = %v, want %v", class, tt.expectedClass)
			}
			if len(actions) != tt.expectedAction {
				t.Errorf("classify() actions count = %d, want %d", len(actions), tt.expectedAction)
			}
		})
	}
}

func TestQuickActions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectYes   bool
		expectNo    bool
		expectCtrlC bool
		expectCont  bool
	}{
		{
			name:        "[Y/n] prompt",
			input:       "Proceed? [Y/n]",
			expectYes:   true,
			expectNo:    true,
			expectCtrlC: true,
		},
		{
			name:        "[y/N] prompt",
			input:       "Continue? [y/N]",
			expectYes:   true,
			expectNo:    true,
			expectCtrlC: true,
		},
		{
			name:       "generic question",
			input:      "Would you like to continue?",
			expectCont: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := generateQuickActions(tt.input)
			foundYes := false
			foundNo := false
			foundCtrlC := false
			foundCont := false

			for _, a := range actions {
				switch a.Label {
				case "Yes":
					foundYes = true
					if a.Keys != "y\n" {
						t.Errorf("Yes action keys = %q, want %q", a.Keys, "y\n")
					}
				case "No":
					foundNo = true
					if a.Keys != "n\n" {
						t.Errorf("No action keys = %q, want %q", a.Keys, "n\n")
					}
				case "Ctrl+C":
					foundCtrlC = true
					if a.Keys != "\x03" {
						t.Errorf("Ctrl+C action keys = %q, want %q", a.Keys, "\x03")
					}
				case "Continue":
					foundCont = true
					if a.Keys != "\n" {
						t.Errorf("Continue action keys = %q, want %q", a.Keys, "\n")
					}
				}
			}

			if tt.expectYes != foundYes {
				t.Errorf("found Yes = %v, want %v", foundYes, tt.expectYes)
			}
			if tt.expectNo != foundNo {
				t.Errorf("found No = %v, want %v", foundNo, tt.expectNo)
			}
			if tt.expectCtrlC != foundCtrlC {
				t.Errorf("found Ctrl+C = %v, want %v", foundCtrlC, tt.expectCtrlC)
			}
			if tt.expectCont != foundCont {
				t.Errorf("found Continue = %v, want %v", foundCont, tt.expectCont)
			}
		})
	}
}

func TestParserFeedAndMessages(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "Hello ")

	select {
	case <-p.Messages():
		t.Error("unexpected immediate message")
	case <-time.After(100 * time.Millisecond):
	}

	p.Feed("win1", "World\n")

	select {
	case msg := <-p.Messages():
		if msg.WindowID != "win1" {
			t.Errorf("WindowID = %q, want %q", msg.WindowID, "win1")
		}
		if msg.Text != "Hello World\n" {
			t.Errorf("Text = %q, want %q", msg.Text, "Hello World\n")
		}
		if msg.Class != ClassNormal {
			t.Errorf("Class = %v, want %v", msg.Class, ClassNormal)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestParserImmediateFlushOnPrompt(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "Do you want to continue? [Y/n]")

	select {
	case msg := <-p.Messages():
		if msg.Class != ClassPrompt {
			t.Errorf("Class = %v, want %v", msg.Class, ClassPrompt)
		}
		if len(msg.Actions) != 3 {
			t.Errorf("Actions count = %d, want 3", len(msg.Actions))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected immediate flush for prompt")
	}
}

func TestParserImmediateFlushOnShellPrompt(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "output\n$ ")

	select {
	case msg := <-p.Messages():
		if msg.Text != "output\n$ " {
			t.Errorf("Text = %q, want %q", msg.Text, "output\n$ ")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected immediate flush for shell prompt")
	}
}

func TestParserStatusTracking(t *testing.T) {
	p := New()

	p.Feed("win1", "working")
	if status := p.Status("win1"); status != StatusWorking {
		t.Errorf("Status = %v, want %v", status, StatusWorking)
	}

	p.Feed("win1", " [Y/n]")
	time.Sleep(50 * time.Millisecond)
	if status := p.Status("win1"); status != StatusWaiting {
		t.Errorf("Status after prompt = %v, want %v", status, StatusWaiting)
	}

	p.Close()
}

func TestParserStatusIdle(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "test")
	p.mu.Lock()
	buf := p.buffers["win1"]
	buf.lastOutput = time.Now().Add(-35 * time.Second)
	p.mu.Unlock()

	time.Sleep(1100 * time.Millisecond)

	if status := p.Status("win1"); status != StatusIdle {
		t.Errorf("Status = %v, want %v", status, StatusIdle)
	}
}

func TestParserTimeoutFlush(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "some output")

	select {
	case <-p.Messages():
		t.Error("unexpected immediate message")
	case <-time.After(1 * time.Second):
	}

	select {
	case msg := <-p.Messages():
		if msg.Text != "some output" {
			t.Errorf("Text = %q, want %q", msg.Text, "some output")
		}
	case <-time.After(700 * time.Millisecond):
		t.Error("expected message after timeout")
	}
}

func TestParserANSIStrippingInMessages(t *testing.T) {
	p := New()
	defer p.Close()

	coloredInput := "\x1b[31mError: \x1b[0mfile not found"
	p.Feed("win1", coloredInput+"\n$ ")

	select {
	case msg := <-p.Messages():
		if msg.RawText != coloredInput+"\n$ " {
			t.Errorf("RawText = %q, want %q", msg.RawText, coloredInput+"\n$ ")
		}
		if strings.Contains(msg.Text, "\x1b[") {
			t.Errorf("Text should not contain ANSI codes: %q", msg.Text)
		}
		if msg.Text != "Error: file not found\n$ " {
			t.Errorf("Text = %q, want %q", msg.Text, "Error: file not found\n$ ")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}

func TestParserMultipleWindows(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "window 1\n$ ")
	p.Feed("win2", "window 2\n$ ")

	msgs := make([]Message, 0, 2)
	timeout := time.After(200 * time.Millisecond)
	for len(msgs) < 2 {
		select {
		case msg := <-p.Messages():
			msgs = append(msgs, msg)
		case <-timeout:
			t.Fatal("timeout waiting for messages")
		}
	}

	foundWin1 := false
	foundWin2 := false
	for _, msg := range msgs {
		if msg.WindowID == "win1" {
			foundWin1 = true
		}
		if msg.WindowID == "win2" {
			foundWin2 = true
		}
	}

	if !foundWin1 || !foundWin2 {
		t.Errorf("expected messages from both windows, got win1=%v win2=%v", foundWin1, foundWin2)
	}
}

func TestParserMessageIDs(t *testing.T) {
	p := New()
	defer p.Close()

	p.Feed("win1", "msg1\n$ ")
	p.Feed("win1", "msg2\n$ ")
	p.Feed("win2", "msg3\n$ ")

	msgs := make([]Message, 0, 3)
	timeout := time.After(200 * time.Millisecond)
	for len(msgs) < 3 {
		select {
		case msg := <-p.Messages():
			msgs = append(msgs, msg)
		case <-timeout:
			t.Fatal("timeout waiting for messages")
		}
	}

	for _, msg := range msgs {
		if !strings.HasPrefix(msg.ID, msg.WindowID+"-") {
			t.Errorf("Message ID %q should start with WindowID %q", msg.ID, msg.WindowID)
		}
	}
}

func TestParserNoGoroutineLeak(t *testing.T) {
	p := New()
	p.Feed("win1", "test")

	done := make(chan struct{})
	go func() {
		p.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Close() took too long, possible goroutine leak")
	}
}
