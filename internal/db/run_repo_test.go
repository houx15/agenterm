package db

import (
	"context"
	"testing"
)

func TestRunRepoEnsureAndTransition(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	runRepo := NewRunRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "P", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	run, err := runRepo.EnsureActive(ctx, project.ID, "plan", "manual")
	if err != nil {
		t.Fatalf("ensure active run: %v", err)
	}
	if run.CurrentStage != "plan" {
		t.Fatalf("current_stage=%q want plan", run.CurrentStage)
	}

	if err := runRepo.UpsertStageRun(ctx, run.ID, "plan", "completed", `{"note":"ok"}`); err != nil {
		t.Fatalf("upsert plan stage: %v", err)
	}
	if err := runRepo.UpdateStage(ctx, run.ID, "build", "active"); err != nil {
		t.Fatalf("update stage: %v", err)
	}
	if err := runRepo.UpsertStageRun(ctx, run.ID, "build", "active", `{"workers":2}`); err != nil {
		t.Fatalf("upsert build stage: %v", err)
	}

	active, err := runRepo.GetActiveByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("get active run: %v", err)
	}
	if active == nil || active.CurrentStage != "build" {
		t.Fatalf("active run=%#v want build stage", active)
	}
	stages, err := runRepo.ListStageRuns(ctx, run.ID)
	if err != nil {
		t.Fatalf("list stages: %v", err)
	}
	if len(stages) != 2 {
		t.Fatalf("len(stages)=%d want 2", len(stages))
	}
}
