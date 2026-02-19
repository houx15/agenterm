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
	if r.Get("tdd") == nil {
		t.Fatalf("expected tdd playbook")
	}
	if r.Get("compound-engineering") == nil {
		t.Fatalf("expected compound-engineering playbook")
	}
	for _, name := range []string{"pairing-coding.yaml", "tdd.yaml", "compound-engineering.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s file missing: %v", name, err)
		}
	}
}

func TestMatchProjectDefaultsToPairingCoding(t *testing.T) {
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
	if matched.ID != "pairing-coding" {
		t.Fatalf("matched id=%q want pairing-coding", matched.ID)
	}
}

func TestSaveDeleteReload(t *testing.T) {
	r, err := NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	custom := &Playbook{
		ID:          "custom-playbook",
		Name:        "Custom Playbook",
		Description: "desc",
		Phases:      []Phase{{Name: "Plan", Agent: "codex", Role: "planner", Description: "discover"}},
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
