package db

import (
	"context"
	"testing"
)

func TestRoleAgentAssignmentRepoReplaceAndList(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	repo := NewRoleAgentAssignmentRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "P", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := repo.ReplaceForProject(ctx, project.ID, []*RoleAgentAssignment{
		{Stage: "plan", Role: "planner", AgentType: "claude-code", MaxParallel: 1},
		{Stage: "build", Role: "coder", AgentType: "codex", MaxParallel: 4},
	}); err != nil {
		t.Fatalf("replace assignments: %v", err)
	}

	items, err := repo.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("list assignments: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items)=%d want 2", len(items))
	}
	if items[0].Role == "" || items[1].Role == "" {
		t.Fatalf("unexpected empty roles: %#v", items)
	}
}
