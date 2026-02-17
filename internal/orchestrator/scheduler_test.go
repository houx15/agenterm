package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/registry"
)

func TestSchedulerBlocksCreateSessionWhenProjectLimitReached(t *testing.T) {
	database := openOrchestratorTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())
	profileRepo := db.NewProjectOrchestratorRepo(database.SQL())
	workflowRepo := db.NewWorkflowRepo(database.SQL())
	ctx := context.Background()

	project := &db.Project{Name: "P", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := profileRepo.EnsureDefaultForProject(ctx, project.ID); err != nil {
		t.Fatalf("ensure profile: %v", err)
	}
	profile, err := profileRepo.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	profile.MaxParallel = 1
	if err := profileRepo.Update(ctx, profile); err != nil {
		t.Fatalf("update profile: %v", err)
	}

	task1 := &db.Task{ProjectID: project.ID, Title: "t1", Description: "d", Status: "pending"}
	task2 := &db.Task{ProjectID: project.ID, Title: "t2", Description: "d", Status: "pending"}
	if err := taskRepo.Create(ctx, task1); err != nil {
		t.Fatalf("create task1: %v", err)
	}
	if err := taskRepo.Create(ctx, task2); err != nil {
		t.Fatalf("create task2: %v", err)
	}

	sess := &db.Session{TaskID: task1.ID, TmuxSessionName: "s1", TmuxWindowID: "@1", AgentType: "codex", Role: "coder", Status: "working"}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	reg, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	o := New(Options{
		ProjectRepo:             projectRepo,
		TaskRepo:                taskRepo,
		SessionRepo:             sessionRepo,
		ProjectOrchestratorRepo: profileRepo,
		WorkflowRepo:            workflowRepo,
		Registry:                reg,
	})

	decision := o.checkSessionCreationAllowed(ctx, map[string]any{
		"task_id":    task2.ID,
		"role":       "coder",
		"agent_type": "codex",
	})
	if decision.Allowed {
		t.Fatalf("expected session creation to be blocked")
	}
}
