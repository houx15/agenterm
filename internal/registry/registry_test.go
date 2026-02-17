package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRegistryCreatesDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got := r.List()
	if len(got) < 5 {
		t.Fatalf("len(List()) = %d, want >= 5", len(got))
	}

	for _, id := range []string{"claude-code", "codex", "gemini-cli", "opencode", "kimi-cli"} {
		if r.Get(id) == nil {
			t.Fatalf("expected default agent %q", id)
		}
		if _, err := os.Stat(filepath.Join(dir, id+".yaml")); err != nil {
			t.Fatalf("default file missing for %q: %v", id, err)
		}
	}
}

func TestNewRegistryValidationFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: bad-agent\nname: \"\"\ncommand: run\n"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	if _, err := NewRegistry(dir); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestRegistrySaveDeleteReload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	custom := &AgentConfig{
		ID:      "test-agent",
		Name:    "Test Agent",
		Command: "test-agent run",
	}
	if err := r.Save(custom); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got := r.Get("test-agent"); got == nil || got.Name != "Test Agent" {
		t.Fatalf("Get(test-agent) = %#v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "test-agent.yaml"), []byte("id: test-agent\nname: Updated\ncommand: test-agent run\n"), 0o644); err != nil {
		t.Fatalf("overwrite file: %v", err)
	}
	if err := r.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if got := r.Get("test-agent"); got == nil || got.Name != "Updated" {
		t.Fatalf("after reload = %#v", got)
	}

	if err := r.Delete("test-agent"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := r.Get("test-agent"); got != nil {
		t.Fatalf("expected deleted agent, got %#v", got)
	}
}

func TestRegistrySaveValidation(t *testing.T) {
	r, err := NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	err = r.Save(&AgentConfig{ID: "Bad_ID", Name: "Bad", Command: "run"})
	if err == nil {
		t.Fatalf("expected invalid id error")
	}
}
