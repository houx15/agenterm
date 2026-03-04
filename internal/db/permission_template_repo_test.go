package db

import (
	"context"
	"testing"
)

func TestPermissionTemplateRepoCRUD(t *testing.T) {
	database, _ := openTestDB(t)
	repo := NewPermissionTemplateRepo(database.SQL())
	ctx := context.Background()

	// Create
	tmpl := &PermissionTemplate{
		AgentType: "coder",
		Name:      "Default Coder Permissions",
		Config:    `{"allow_write": true, "allow_exec": false}`,
	}
	if err := repo.Create(ctx, tmpl); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if tmpl.ID == "" {
		t.Fatal("Create() did not set ID")
	}
	if tmpl.CreatedAt.IsZero() {
		t.Fatal("Create() did not set CreatedAt")
	}

	// Get
	got, err := repo.Get(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.AgentType != "coder" || got.Name != "Default Coder Permissions" {
		t.Fatalf("Get() got = %#v", got)
	}
	if got.Config != `{"allow_write": true, "allow_exec": false}` {
		t.Fatalf("Get() Config = %q", got.Config)
	}

	// Get non-existent
	missing, err := repo.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get(nonexistent) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("Get(nonexistent) = %#v, want nil", missing)
	}

	// Update
	tmpl.Name = "Updated Coder Permissions"
	tmpl.Config = `{"allow_write": true, "allow_exec": true}`
	if err := repo.Update(ctx, tmpl); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := repo.Get(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Name != "Updated Coder Permissions" || updated.Config != `{"allow_write": true, "allow_exec": true}` {
		t.Fatalf("updated = %#v", updated)
	}

	// Delete
	if err := repo.Delete(ctx, tmpl.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	deleted, err := repo.Get(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if deleted != nil {
		t.Fatalf("Get() after delete = %#v, want nil", deleted)
	}
}

func TestPermissionTemplateRepoListByAgent(t *testing.T) {
	database, _ := openTestDB(t)
	repo := NewPermissionTemplateRepo(database.SQL())
	ctx := context.Background()

	// Create templates for different agent types
	templates := []*PermissionTemplate{
		{AgentType: "coder", Name: "Coder Default", Config: "{}"},
		{AgentType: "coder", Name: "Coder Strict", Config: `{"strict": true}`},
		{AgentType: "reviewer", Name: "Reviewer Default", Config: "{}"},
	}
	for _, tmpl := range templates {
		if err := repo.Create(ctx, tmpl); err != nil {
			t.Fatalf("Create(%s) error = %v", tmpl.Name, err)
		}
	}

	// ListByAgent for coder
	coderList, err := repo.ListByAgent(ctx, "coder")
	if err != nil {
		t.Fatalf("ListByAgent(coder) error = %v", err)
	}
	if len(coderList) != 2 {
		t.Fatalf("ListByAgent(coder) len = %d, want 2", len(coderList))
	}

	// ListByAgent for reviewer
	reviewerList, err := repo.ListByAgent(ctx, "reviewer")
	if err != nil {
		t.Fatalf("ListByAgent(reviewer) error = %v", err)
	}
	if len(reviewerList) != 1 {
		t.Fatalf("ListByAgent(reviewer) len = %d, want 1", len(reviewerList))
	}

	// ListByAgent for non-existent type
	emptyList, err := repo.ListByAgent(ctx, "planner")
	if err != nil {
		t.Fatalf("ListByAgent(planner) error = %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("ListByAgent(planner) len = %d, want 0", len(emptyList))
	}
}

func TestPermissionTemplateRepoList(t *testing.T) {
	database, _ := openTestDB(t)
	repo := NewPermissionTemplateRepo(database.SQL())
	ctx := context.Background()

	// Initially empty
	emptyList, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("List() len = %d, want 0", len(emptyList))
	}

	// Create some templates
	for _, name := range []string{"Template A", "Template B"} {
		tmpl := &PermissionTemplate{
			AgentType: "coder",
			Name:      name,
			Config:    "{}",
		}
		if err := repo.Create(ctx, tmpl); err != nil {
			t.Fatalf("Create(%s) error = %v", name, err)
		}
	}

	// List all
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
}
