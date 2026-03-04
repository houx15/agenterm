package db

import (
	"context"
	"testing"
)

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

func TestReviewRepoUpdateCycleStatusRejectsInvalidTransitionAndOpenIssues(t *testing.T) {
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
	cycle := &ReviewCycle{TaskID: task.ID}
	if err := reviewRepo.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	issue := &ReviewIssue{CycleID: cycle.ID, Summary: "must fix"}
	if err := reviewRepo.CreateIssue(ctx, issue); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	if err := reviewRepo.UpdateCycleStatus(ctx, cycle.ID, "review_passed"); err == nil {
		t.Fatalf("expected review_passed to be blocked while issues are open")
	}

	if err := reviewRepo.UpdateCycleStatus(ctx, cycle.ID, "done"); err == nil {
		t.Fatalf("expected invalid cycle status to fail")
	}

	if err := reviewRepo.UpdateCycleStatus(ctx, cycle.ID, "review_running"); err != nil {
		t.Fatalf("set running: %v", err)
	}
	if err := reviewRepo.UpdateCycleStatus(ctx, cycle.ID, "review_pending"); err == nil {
		t.Fatalf("expected invalid transition review_running -> review_pending")
	}
}

func TestReviewRepoSetCycleStatusByTaskOpenIssuesUsesStateMachine(t *testing.T) {
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
	cycle := &ReviewCycle{TaskID: task.ID}
	if err := reviewRepo.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if err := reviewRepo.UpdateCycleStatus(ctx, cycle.ID, "review_passed"); err != nil {
		t.Fatalf("set cycle passed: %v", err)
	}
	issue := &ReviewIssue{CycleID: cycle.ID, Summary: "new regression"}
	if err := reviewRepo.CreateIssue(ctx, issue); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	if err := reviewRepo.SetCycleStatusByTaskOpenIssues(ctx, task.ID); err != nil {
		t.Fatalf("sync cycle status by issues: %v", err)
	}
	updated, err := reviewRepo.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("get cycle: %v", err)
	}
	if updated.Status != "review_changes_requested" {
		t.Fatalf("cycle status=%q want review_changes_requested", updated.Status)
	}

	issue.Status = "resolved"
	if err := reviewRepo.UpdateIssue(ctx, issue); err != nil {
		t.Fatalf("resolve issue: %v", err)
	}
	if err := reviewRepo.SetCycleStatusByTaskOpenIssues(ctx, task.ID); err != nil {
		t.Fatalf("sync cycle status after resolve: %v", err)
	}
	updated, err = reviewRepo.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("get cycle after resolve: %v", err)
	}
	if updated.Status != "review_passed" {
		t.Fatalf("cycle status=%q want review_passed", updated.Status)
	}
}

func TestReviewRepoSyncLatestCycleStatusByTaskOpenIssuesReturnsChangedFlag(t *testing.T) {
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
	cycle := &ReviewCycle{TaskID: task.ID, Status: "review_changes_requested"}
	if err := reviewRepo.CreateCycle(ctx, cycle); err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	issue := &ReviewIssue{CycleID: cycle.ID, Summary: "open issue", Status: "open"}
	if err := reviewRepo.CreateIssue(ctx, issue); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	changed, latest, err := reviewRepo.SyncLatestCycleStatusByTaskOpenIssues(ctx, task.ID)
	if err != nil {
		t.Fatalf("sync status when unchanged: %v", err)
	}
	if changed {
		t.Fatalf("changed=%v want false", changed)
	}
	if latest == nil || latest.Status != "review_changes_requested" {
		t.Fatalf("latest=%#v want status review_changes_requested", latest)
	}

	issue.Status = "resolved"
	if err := reviewRepo.UpdateIssue(ctx, issue); err != nil {
		t.Fatalf("resolve issue: %v", err)
	}
	changed, latest, err = reviewRepo.SyncLatestCycleStatusByTaskOpenIssues(ctx, task.ID)
	if err != nil {
		t.Fatalf("sync status when changed: %v", err)
	}
	if !changed {
		t.Fatalf("changed=%v want true", changed)
	}
	if latest == nil || latest.Status != "review_passed" {
		t.Fatalf("latest=%#v want status review_passed", latest)
	}
}
