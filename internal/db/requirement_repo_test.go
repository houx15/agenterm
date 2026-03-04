package db

import (
	"context"
	"testing"
)

func TestRequirementRepoCRUD(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	reqRepo := NewRequirementRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "ReqProj", RepoPath: "/tmp/reqproj", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}

	// Create
	req := &Requirement{
		ProjectID:   project.ID,
		Title:       "Build auth module",
		Description: "Implement OAuth2 flow",
		Priority:    0,
		Status:      "queued",
	}
	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if req.ID == "" {
		t.Fatal("Create() did not set ID")
	}
	if req.CreatedAt.IsZero() {
		t.Fatal("Create() did not set CreatedAt")
	}

	// Get
	got, err := reqRepo.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Title != "Build auth module" || got.Description != "Implement OAuth2 flow" || got.Status != "queued" {
		t.Fatalf("Get() got = %#v", got)
	}

	// Get non-existent
	missing, err := reqRepo.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get(nonexistent) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("Get(nonexistent) = %#v, want nil", missing)
	}

	// Update
	req.Title = "Build auth module v2"
	req.Status = "in_progress"
	req.Priority = 5
	if err := reqRepo.Update(ctx, req); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := reqRepo.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Title != "Build auth module v2" || updated.Status != "in_progress" || updated.Priority != 5 {
		t.Fatalf("updated = %#v", updated)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) || updated.UpdatedAt.Equal(updated.CreatedAt) {
		// UpdatedAt should be >= CreatedAt after update
	}

	// Delete
	if err := reqRepo.Delete(ctx, req.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	deleted, err := reqRepo.Get(ctx, req.ID)
	if err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if deleted != nil {
		t.Fatalf("Get() after delete = %#v, want nil", deleted)
	}
}

func TestRequirementRepoListByProject(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	reqRepo := NewRequirementRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "ListProj", RepoPath: "/tmp/listproj", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}

	// Create multiple requirements with different priorities
	for i, title := range []string{"Third", "First", "Second"} {
		priority := []int{2, 0, 1}[i]
		req := &Requirement{
			ProjectID: project.ID,
			Title:     title,
			Status:    "queued",
			Priority:  priority,
		}
		if err := reqRepo.Create(ctx, req); err != nil {
			t.Fatalf("Create(%s) error = %v", title, err)
		}
	}

	// ListByProject should be ordered by priority ASC
	list, err := reqRepo.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByProject len = %d, want 3", len(list))
	}
	if list[0].Title != "First" || list[1].Title != "Second" || list[2].Title != "Third" {
		t.Fatalf("ListByProject order = [%s, %s, %s], want [First, Second, Third]",
			list[0].Title, list[1].Title, list[2].Title)
	}

	// Different project should have no requirements
	other := &Project{Name: "OtherProj", RepoPath: "/tmp/other", Status: "active"}
	if err := projectRepo.Create(ctx, other); err != nil {
		t.Fatalf("create other project error = %v", err)
	}
	otherList, err := reqRepo.ListByProject(ctx, other.ID)
	if err != nil {
		t.Fatalf("ListByProject(other) error = %v", err)
	}
	if len(otherList) != 0 {
		t.Fatalf("ListByProject(other) len = %d, want 0", len(otherList))
	}
}

func TestRequirementRepoReorder(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	reqRepo := NewRequirementRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "ReorderProj", RepoPath: "/tmp/reorder", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}

	var ids []string
	for _, title := range []string{"A", "B", "C"} {
		req := &Requirement{
			ProjectID: project.ID,
			Title:     title,
			Status:    "queued",
		}
		if err := reqRepo.Create(ctx, req); err != nil {
			t.Fatalf("Create(%s) error = %v", title, err)
		}
		ids = append(ids, req.ID)
	}

	// Reorder: C, A, B
	reordered := []string{ids[2], ids[0], ids[1]}
	if err := reqRepo.Reorder(ctx, project.ID, reordered); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	list, err := reqRepo.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject() after reorder error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByProject len = %d, want 3", len(list))
	}
	if list[0].Title != "C" || list[1].Title != "A" || list[2].Title != "B" {
		t.Fatalf("reorder result = [%s, %s, %s], want [C, A, B]",
			list[0].Title, list[1].Title, list[2].Title)
	}
	// Verify priorities are set to array indices
	if list[0].Priority != 0 || list[1].Priority != 1 || list[2].Priority != 2 {
		t.Fatalf("reorder priorities = [%d, %d, %d], want [0, 1, 2]",
			list[0].Priority, list[1].Priority, list[2].Priority)
	}
}
