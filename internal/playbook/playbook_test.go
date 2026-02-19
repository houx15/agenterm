package playbook

import (
	"errors"
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

	if r.Get("pairing-coding") == nil {
		t.Fatalf("expected pairing-coding playbook")
	}
	if r.Get("tdd-coding") == nil {
		t.Fatalf("expected tdd-coding playbook")
	}
	if r.Get("compound-engineering-workflow") == nil {
		t.Fatalf("expected compound-engineering-workflow playbook")
	}
	if _, err := os.Stat(filepath.Join(dir, "pairing-coding.yaml")); err != nil {
		t.Fatalf("pairing-coding file missing: %v", err)
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
	if matched.ID != "tdd-coding" {
		t.Fatalf("matched id=%q want tdd-coding", matched.ID)
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

func TestDeleteMissingReturnsNotFound(t *testing.T) {
	r, err := NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	err = r.Delete("missing-playbook")
	if err == nil {
		t.Fatalf("Delete() error = nil, want not found")
	}
	if !errors.Is(err, ErrPlaybookNotFound) {
		t.Fatalf("Delete() error = %v, want ErrPlaybookNotFound", err)
	}
}
