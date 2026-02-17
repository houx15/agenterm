package db

import (
	"context"
	"testing"
)

func TestWorkflowRepoBuiltinsLoaded(t *testing.T) {
	database, _ := openTestDB(t)
	repo := NewWorkflowRepo(database.SQL())
	items, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if len(items) < 3 {
		t.Fatalf("workflows len=%d want >= 3", len(items))
	}
}

func TestProjectOrchestratorEnsureDefault(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()
	projectRepo := NewProjectRepo(database.SQL())
	profileRepo := NewProjectOrchestratorRepo(database.SQL())

	project := &Project{Name: "P", RepoPath: "/tmp/p", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := profileRepo.EnsureDefaultForProject(ctx, project.ID); err != nil {
		t.Fatalf("ensure default profile: %v", err)
	}
	item, err := profileRepo.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if item == nil {
		t.Fatalf("profile missing")
	}
	if item.WorkflowID != "workflow-balanced" {
		t.Fatalf("workflow_id=%q want workflow-balanced", item.WorkflowID)
	}
}

func TestProjectKnowledgeRepoCreateAndList(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()
	projectRepo := NewProjectRepo(database.SQL())
	knowledgeRepo := NewProjectKnowledgeRepo(database.SQL())

	project := &Project{Name: "P", RepoPath: "/tmp/p", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := knowledgeRepo.Create(ctx, &ProjectKnowledgeEntry{ProjectID: project.ID, Kind: "design", Title: "T", Content: "C"}); err != nil {
		t.Fatalf("create knowledge: %v", err)
	}
	items, err := knowledgeRepo.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("list knowledge: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("knowledge len=%d want 1", len(items))
	}
}

func TestReviewRepoCycleAndIssues(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()

	projectRepo := NewProjectRepo(database.SQL())
	taskRepo := NewTaskRepo(database.SQL())
	reviewRepo := NewReviewRepo(database.SQL())

	project := &Project{Name: "P", RepoPath: "/tmp/p", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &Task{ProjectID: project.ID, Title: "T", Description: "D", Status: "running"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	cycle := &ReviewCycle{TaskID: task.ID, CommitHash: "abc123"}
	if err := reviewRepo.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if cycle.Iteration != 1 {
		t.Fatalf("iteration=%d want 1", cycle.Iteration)
	}

	issue := &ReviewIssue{CycleID: cycle.ID, Severity: "high", Summary: "fix bug"}
	if err := reviewRepo.CreateIssue(ctx, issue); err != nil {
		t.Fatalf("create issue: %v", err)
	}
	open, err := reviewRepo.CountOpenIssuesByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("count open: %v", err)
	}
	if open != 1 {
		t.Fatalf("open issues=%d want 1", open)
	}

	issue.Status = "resolved"
	if err := reviewRepo.UpdateIssue(ctx, issue); err != nil {
		t.Fatalf("update issue: %v", err)
	}
	open, err = reviewRepo.CountOpenIssuesByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("count open after resolve: %v", err)
	}
	if open != 0 {
		t.Fatalf("open issues after resolve=%d want 0", open)
	}
}
