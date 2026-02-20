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

func TestMatchProjectDefaultsToCompoundEngineering(t *testing.T) {
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
	if matched.ID != "compound-engineering" {
		t.Fatalf("matched id=%q want compound-engineering", matched.ID)
	}
}

func TestNewRegistryAddsMissingDefaultsWhenDirAlreadyHasYAML(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "playbooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(`
id: custom
name: Custom
description: custom
workflow:
  plan:
    enabled: true
    roles:
      - name: planner
        responsibilities: plan
        allowed_agents: [codex]
  build:
    enabled: false
    roles: []
  test:
    enabled: false
    roles: []
`), 0o644); err != nil {
		t.Fatalf("write custom.yaml: %v", err)
	}

	r, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if r.Get("custom") == nil {
		t.Fatalf("expected custom playbook")
	}
	if r.Get("pairing-coding") == nil {
		t.Fatalf("expected pairing-coding default playbook")
	}
	if r.Get("tdd") == nil {
		t.Fatalf("expected tdd default playbook")
	}
	if r.Get("compound-engineering") == nil {
		t.Fatalf("expected compound-engineering default playbook")
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

func TestSaveInfersRoleModeAndKeepsContractFields(t *testing.T) {
	r, err := NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	custom := &Playbook{
		ID:          "contract-playbook",
		Name:        "Contract Playbook",
		Description: "desc",
		Workflow: Workflow{
			Plan: Stage{
				Enabled: true,
				Roles: []StageRole{{
					Name:             "planner",
					Responsibilities: "plan",
					AllowedAgents:    []string{"claude-code"},
					InputsRequired:   []string{"goal"},
					OutputsContract: OutputsContract{
						Type:     "plan_result",
						Required: []string{"task_graph", "worktree_plan"},
					},
					Gates: RoleGates{
						RequiresUserApproval: true,
						PassCondition:        "has_task_graph",
					},
					RetryPolicy: RetryPolicy{
						MaxIterations: 3,
						EscalateOn:    []string{"blocked"},
					},
				}},
				StagePolicy: StagePolicy{
					EnterGate:            "user_confirmed",
					ExitGate:             "plan_approved",
					MaxParallelWorktrees: 4,
				},
			},
			Build: Stage{Enabled: false, Roles: []StageRole{}},
			Test:  Stage{Enabled: false, Roles: []StageRole{}},
		},
	}

	if err := r.Save(custom); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got := r.Get("contract-playbook")
	if got == nil {
		t.Fatalf("expected saved playbook")
	}
	role := got.Workflow.Plan.Roles[0]
	if role.Mode != "planner" {
		t.Fatalf("mode=%q want planner", role.Mode)
	}
	if role.OutputsContract.Type != "plan_result" {
		t.Fatalf("outputs_contract.type=%q", role.OutputsContract.Type)
	}
	if !role.Gates.RequiresUserApproval {
		t.Fatalf("requires_user_approval=false want true")
	}
	if got.Workflow.Plan.StagePolicy.MaxParallelWorktrees != 4 {
		t.Fatalf("max_parallel_worktrees=%d want 4", got.Workflow.Plan.StagePolicy.MaxParallelWorktrees)
	}
}
