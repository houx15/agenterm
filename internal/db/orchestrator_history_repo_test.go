package db

import (
	"context"
	"testing"
)

func TestOrchestratorHistoryRepoCreateListTrim(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()

	projectRepo := NewProjectRepo(database.SQL())
	history := NewOrchestratorHistoryRepo(database.SQL())

	project := &Project{Name: "P", RepoPath: "/tmp/p", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := history.Create(ctx, &OrchestratorMessage{
			ProjectID: project.ID,
			Role:      "user",
			Content:   "msg",
		}); err != nil {
			t.Fatalf("create message %d: %v", i, err)
		}
	}

	items, err := history.ListByProject(ctx, project.ID, 3)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("history len=%d want 3", len(items))
	}

	if err := history.TrimProject(ctx, project.ID, 2); err != nil {
		t.Fatalf("trim history: %v", err)
	}
	items, err = history.ListByProject(ctx, project.ID, 10)
	if err != nil {
		t.Fatalf("list history after trim: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("history after trim len=%d want 2", len(items))
	}
}
