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

func TestProjectRepoCreateWithDefaultOrchestratorIsAtomic(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()
	projectRepo := NewProjectRepo(database.SQL())

	if _, err := database.SQL().ExecContext(ctx, `DELETE FROM workflow_phases`); err != nil {
		t.Fatalf("delete workflow_phases: %v", err)
	}
	if _, err := database.SQL().ExecContext(ctx, `DELETE FROM workflows`); err != nil {
		t.Fatalf("delete workflows: %v", err)
	}

	project := &Project{Name: "P", RepoPath: "/tmp/p", Status: "active"}
	if err := projectRepo.CreateWithDefaultOrchestrator(ctx, project); err == nil {
		t.Fatalf("expected create with default orchestrator to fail")
	}

	var count int
	if err := database.SQL().QueryRowContext(ctx, `SELECT count(1) FROM projects WHERE id = ?`, project.ID).Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 0 {
		t.Fatalf("project row should be rolled back, count=%d", count)
	}
}

func TestWorkflowRepoUpdateRollsBackWhenPhaseInsertFails(t *testing.T) {
	database, _ := openTestDB(t)
	ctx := context.Background()
	workflowRepo := NewWorkflowRepo(database.SQL())

	original := &Workflow{
		ID:          "workflow-test-update-atomic",
		Name:        "Original",
		Description: "before",
		Scope:       "project",
		Version:     1,
		Phases: []*WorkflowPhase{
			{ID: "phase-original", Ordinal: 1, PhaseType: "scan", Role: "planner", MaxParallel: 1},
		},
	}
	if err := workflowRepo.Create(ctx, original); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	err := workflowRepo.Update(ctx, &Workflow{
		ID:          original.ID,
		Name:        "Updated",
		Description: "after",
		Scope:       "project",
		Version:     2,
		Phases: []*WorkflowPhase{
			{ID: "phase-dup", Ordinal: 1, PhaseType: "planning", Role: "planner", MaxParallel: 1},
			{ID: "phase-dup", Ordinal: 2, PhaseType: "review", Role: "reviewer", MaxParallel: 1},
		},
	})
	if err == nil {
		t.Fatalf("expected update to fail due to duplicate phase id")
	}

	stored, err := workflowRepo.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	if stored == nil {
		t.Fatalf("workflow missing after failed update")
	}
	if stored.Name != "Original" || stored.Version != 1 {
		t.Fatalf("workflow update should roll back, got name=%q version=%d", stored.Name, stored.Version)
	}
	if len(stored.Phases) != 1 {
		t.Fatalf("phase count=%d want 1", len(stored.Phases))
	}
	if stored.Phases[0].ID != "phase-original" || stored.Phases[0].PhaseType != "scan" {
		t.Fatalf("phase not rolled back, got id=%q type=%q", stored.Phases[0].ID, stored.Phases[0].PhaseType)
	}
}
