package playbook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRegistryCreatesDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "playbooks")
	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	if r.Get("default") == nil {
		t.Fatalf("expected default playbook")
	}
	if r.Get("go-backend") == nil {
		t.Fatalf("expected go-backend playbook")
	}
	if _, err := os.Stat(filepath.Join(dir, "default.yaml")); err != nil {
		t.Fatalf("default file missing: %v", err)
	}
}

func TestMatchProjectByLanguageAndPattern(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir internal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "internal", "main.go"), []byte("package internal\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r, err := NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	matched := r.MatchProject(repo)
	if matched == nil {
		t.Fatalf("expected matched playbook")
	}
	if matched.ID != "go-backend" {
		t.Fatalf("matched id=%q want go-backend", matched.ID)
	}
}

func TestSaveDeleteReload(t *testing.T) {
	r, err := NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	custom := &Playbook{
		ID:                  "custom-playbook",
		Name:                "Custom Playbook",
		Description:         "desc",
		ParallelismStrategy: "strategy",
		Match:               Match{Languages: []string{"go"}},
		Phases:              []Phase{{Name: "Plan", Agent: "codex", Role: "planner", Description: "discover"}},
	}
	if err := r.Save(custom); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got := r.Get("custom-playbook"); got == nil || got.Name != "Custom Playbook" {
		t.Fatalf("Get(custom-playbook) = %#v", got)
	}

	if err := r.Delete("custom-playbook"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := r.Get("custom-playbook"); got != nil {
		t.Fatalf("expected deleted playbook, got %#v", got)
	}
}
