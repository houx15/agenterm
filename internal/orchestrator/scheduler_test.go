package orchestrator

import (
	"context"
	"path/filepath"
	"strings"
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

func TestSchedulerBlocksCreateSessionWhenGlobalLimitReachedAcrossProjects(t *testing.T) {
	database := openOrchestratorTestDB(t)
	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())
	profileRepo := db.NewProjectOrchestratorRepo(database.SQL())
	workflowRepo := db.NewWorkflowRepo(database.SQL())
	ctx := context.Background()

	projectA := &db.Project{Name: "PA", RepoPath: t.TempDir(), Status: "active"}
	projectB := &db.Project{Name: "PB", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, projectA); err != nil {
		t.Fatalf("create projectA: %v", err)
	}
	if err := projectRepo.Create(ctx, projectB); err != nil {
		t.Fatalf("create projectB: %v", err)
	}
	if err := profileRepo.EnsureDefaultForProject(ctx, projectA.ID); err != nil {
		t.Fatalf("ensure profileA: %v", err)
	}
	if err := profileRepo.EnsureDefaultForProject(ctx, projectB.ID); err != nil {
		t.Fatalf("ensure profileB: %v", err)
	}

	taskA := &db.Task{ProjectID: projectA.ID, Title: "ta", Description: "d", Status: "pending"}
	taskB := &db.Task{ProjectID: projectB.ID, Title: "tb", Description: "d", Status: "pending"}
	if err := taskRepo.Create(ctx, taskA); err != nil {
		t.Fatalf("create taskA: %v", err)
	}
	if err := taskRepo.Create(ctx, taskB); err != nil {
		t.Fatalf("create taskB: %v", err)
	}
	if err := sessionRepo.Create(ctx, &db.Session{
		TaskID: taskA.ID, TmuxSessionName: "s-global", TmuxWindowID: "@g1", AgentType: "codex", Role: "coder", Status: "working",
	}); err != nil {
		t.Fatalf("create active session: %v", err)
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
		GlobalMaxParallel:       1,
	})

	decision := o.checkSessionCreationAllowed(ctx, map[string]any{
		"task_id":    taskB.ID,
		"role":       "coder",
		"agent_type": "codex",
	})
	if decision.Allowed {
		t.Fatalf("expected session creation to be blocked by global limit")
	}
	if !strings.Contains(decision.Reason, "global max_parallel") {
		t.Fatalf("reason=%q want global max_parallel", decision.Reason)
	}
}

func TestSchedulerCountsHumanTakeoverTowardProjectCapacity(t *testing.T) {
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
	if err := sessionRepo.Create(ctx, &db.Session{
		TaskID: task1.ID, TmuxSessionName: "s-takeover", TmuxWindowID: "@t1", AgentType: "codex", Role: "coder", Status: "human_takeover",
	}); err != nil {
		t.Fatalf("create takeover session: %v", err)
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
		t.Fatalf("expected session creation to be blocked by project capacity")
	}
	if !strings.Contains(decision.Reason, "project max_parallel") {
		t.Fatalf("reason=%q want project max_parallel", decision.Reason)
	}
}
