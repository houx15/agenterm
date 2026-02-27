package pty

import (
	"strings"
	"testing"
	"time"
)

// TestSessionSpawnAndOutput spawns "echo hello-pty", collects events until
// EventClosed, and verifies the accumulated output contains "hello-pty".
func TestSessionSpawnAndOutput(t *testing.T) {
	s, err := newSession("test-echo", "echo-test", []string{"echo", "hello-pty"}, "", nil)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer s.Close()

	var output strings.Builder
	timeout := time.After(5 * time.Second)

	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				// Channel closed without EventClosed — still check output.
				goto done
			}
			if ev.Type == EventOutput {
				output.WriteString(ev.Data)
			}
			if ev.Type == EventClosed {
				goto done
			}
		case <-timeout:
			t.Fatal("timed out waiting for events")
		}
	}

done:
	if !strings.Contains(output.String(), "hello-pty") {
		t.Errorf("expected output to contain %q, got %q", "hello-pty", output.String())
	}
}

// TestSessionResize spawns "sleep 10", calls Resize(200, 50), verifies no error,
// and closes the session.
func TestSessionResize(t *testing.T) {
	s, err := newSession("test-resize", "resize-test", []string{"sleep", "10"}, "", nil)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	defer s.Close()

	if err := s.Resize(200, 50); err != nil {
		t.Fatalf("Resize: %v", err)
	}
}

// TestSessionWriteAndClose spawns "cat", writes "hello\n", closes the session,
// and verifies that a second Close does not panic.
func TestSessionWriteAndClose(t *testing.T) {
	s, err := newSession("test-write", "write-test", []string{"cat"}, "", nil)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}

	_, err = s.Write([]byte("hello\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close must not panic (closeOnce guarantees this).
	if err := s.Close(); err != nil {
		t.Logf("second Close returned: %v (expected nil)", err)
	}
}
