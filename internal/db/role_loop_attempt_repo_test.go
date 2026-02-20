package db

import (
	"context"
	"testing"
)

func TestRoleLoopAttemptRepoIncrementAndGetTaskAttempts(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()

	projectRepo := NewProjectRepo(database.SQL())
	taskRepo := NewTaskRepo(database.SQL())
	repo := NewRoleLoopAttemptRepo(database.SQL())

	project := &Project{Name: "P", RepoPath: "/tmp/p", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &Task{ProjectID: project.ID, Title: "Task", Description: "Desc", Status: "pending"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	if n, err := repo.Increment(ctx, task.ID, "Worker"); err != nil || n != 1 {
		t.Fatalf("increment worker #1 = (%d, %v), want (1, nil)", n, err)
	}
	if n, err := repo.Increment(ctx, task.ID, "worker"); err != nil || n != 2 {
		t.Fatalf("increment worker #2 = (%d, %v), want (2, nil)", n, err)
	}
	if n, err := repo.Increment(ctx, task.ID, "reviewer"); err != nil || n != 1 {
		t.Fatalf("increment reviewer #1 = (%d, %v), want (1, nil)", n, err)
	}

	attempt, err := repo.GetAttempt(ctx, task.ID, "worker")
	if err != nil {
		t.Fatalf("get attempt worker: %v", err)
	}
	if attempt != 2 {
		t.Fatalf("worker attempts=%d want 2", attempt)
	}

	perTask, err := repo.GetTaskAttempts(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task attempts: %v", err)
	}
	if perTask["worker"] != 2 {
		t.Fatalf("task attempts worker=%d want 2", perTask["worker"])
	}
	if perTask["reviewer"] != 1 {
		t.Fatalf("task attempts reviewer=%d want 1", perTask["reviewer"])
	}
}
