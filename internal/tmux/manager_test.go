package tmux

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestManagerGetGatewayMissing(t *testing.T) {
	m := NewManager("")
	if _, err := m.GetGateway("missing"); err == nil {
		t.Fatal("expected error for missing gateway")
	}
}

func TestManagerIntegrationCreateAttachDestroy(t *testing.T) {
	if os.Getenv("TMUX_INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set TMUX_INTEGRATION_TEST=1 to run")
	}

	m := NewManager("")
	sessionName := "agenterm_mgr_" + time.Now().Format("20060102150405")

	gw, err := m.CreateSession(sessionName, "")
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if gw.SessionName() != sessionName {
		t.Fatalf("session name mismatch: got %q want %q", gw.SessionName(), sessionName)
	}

	attached, err := m.AttachSession(sessionName)
	if err != nil {
		t.Fatalf("attach session failed: %v", err)
	}
	if attached != gw {
		t.Fatal("AttachSession should reuse existing gateway instance")
	}

	if err := m.DestroySession(sessionName); err != nil {
		t.Fatalf("destroy session failed: %v", err)
	}

	if err := exec.Command("tmux", "has-session", "-t", sessionName).Run(); err == nil {
		t.Fatal("expected session to be destroyed")
	}
}
