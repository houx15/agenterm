package pty

import (
	"testing"
	"time"
)

// TestManagerCreateAndDestroy creates session "s1" with "sleep 10", verifies
// ListSessions has 1 entry, destroys the session, and verifies 0 entries.
func TestManagerCreateAndDestroy(t *testing.T) {
	m := NewManager()
	defer m.Close()

	_, err := m.CreateSession("s1", "session-1", []string{"sleep", "10"}, "", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	infos := m.ListSessions()
	if len(infos) != 1 {
		t.Fatalf("expected 1 session, got %d", len(infos))
	}
	if infos[0].ID != "s1" {
		t.Errorf("expected session ID %q, got %q", "s1", infos[0].ID)
	}

	if err := m.DestroySession("s1"); err != nil {
		t.Fatalf("DestroySession: %v", err)
	}

	infos = m.ListSessions()
	if len(infos) != 0 {
		t.Fatalf("expected 0 sessions after destroy, got %d", len(infos))
	}
}

// TestManagerDuplicateSession creates "dup", then tries creating "dup" again
// and expects an error.
func TestManagerDuplicateSession(t *testing.T) {
	m := NewManager()
	defer m.Close()

	_, err := m.CreateSession("dup", "dup-session", []string{"sleep", "10"}, "", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_, err = m.CreateSession("dup", "dup-session-2", []string{"sleep", "10"}, "", nil)
	if err == nil {
		t.Fatal("expected error when creating duplicate session, got nil")
	}
}

// TestManagerGetSession verifies that getting a nonexistent session returns an
// error, and that getting an existing session succeeds.
func TestManagerGetSession(t *testing.T) {
	m := NewManager()
	defer m.Close()

	// Getting a nonexistent session must fail.
	_, err := m.GetSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}

	// Create a session and verify retrieval succeeds.
	created, err := m.CreateSession("exists", "exists-session", []string{"sleep", "10"}, "", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := m.GetSession("exists")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if got.ID() != created.ID() {
		t.Errorf("expected session ID %q, got %q", created.ID(), got.ID())
	}

	// Drain events channel with a timeout to prevent goroutine leaks.
	go func() {
		timeout := time.After(5 * time.Second)
		for {
			select {
			case _, ok := <-got.Events():
				if !ok {
					return
				}
			case <-timeout:
				return
			}
		}
	}()
}
