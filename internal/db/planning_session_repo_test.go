package db

import (
	"context"
	"testing"
)

func TestPlanningSessionRepoCRUD(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	reqRepo := NewRequirementRepo(database.SQL())
	psRepo := NewPlanningSessionRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "PSProj", RepoPath: "/tmp/psproj", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}
	req := &Requirement{ProjectID: project.ID, Title: "Req1", Status: "queued"}
	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("create requirement error = %v", err)
	}

	// Create
	ps := &PlanningSession{
		RequirementID: req.ID,
		Status:        "active",
		Blueprint:     "initial plan",
	}
	if err := psRepo.Create(ctx, ps); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if ps.ID == "" {
		t.Fatal("Create() did not set ID")
	}
	if ps.CreatedAt.IsZero() {
		t.Fatal("Create() did not set CreatedAt")
	}

	// Get
	got, err := psRepo.Get(ctx, ps.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.RequirementID != req.ID || got.Status != "active" || got.Blueprint != "initial plan" {
		t.Fatalf("Get() got = %#v", got)
	}
	if got.AgentSessionID != "" {
		t.Fatalf("Get() AgentSessionID = %q, want empty", got.AgentSessionID)
	}

	// Get non-existent
	missing, err := psRepo.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get(nonexistent) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("Get(nonexistent) = %#v, want nil", missing)
	}

	// Update
	ps.Status = "completed"
	ps.Blueprint = "final blueprint"
	if err := psRepo.Update(ctx, ps); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := psRepo.Get(ctx, ps.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Status != "completed" || updated.Blueprint != "final blueprint" {
		t.Fatalf("updated = %#v", updated)
	}

	// GetByRequirement
	byReq, err := psRepo.GetByRequirement(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetByRequirement() error = %v", err)
	}
	if byReq == nil {
		t.Fatal("GetByRequirement() returned nil")
	}
	if byReq.ID != ps.ID {
		t.Fatalf("GetByRequirement() ID = %q, want %q", byReq.ID, ps.ID)
	}

	// GetByRequirement non-existent
	noReq, err := psRepo.GetByRequirement(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetByRequirement(nonexistent) error = %v", err)
	}
	if noReq != nil {
		t.Fatalf("GetByRequirement(nonexistent) = %#v, want nil", noReq)
	}
}

func TestPlanningSessionRepoWithAgentSession(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	reqRepo := NewRequirementRepo(database.SQL())
	taskRepo := NewTaskRepo(database.SQL())
	sessionRepo := NewSessionRepo(database.SQL())
	psRepo := NewPlanningSessionRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "PSProj2", RepoPath: "/tmp/psproj2", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}
	task := &Task{ProjectID: project.ID, Title: "T1", Description: "D1", Status: "pending"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task error = %v", err)
	}
	session := &Session{
		TaskID:          task.ID,
		TmuxSessionName: "test-session",
		AgentType:       "claude",
		Role:            "planner",
		Status:          "running",
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session error = %v", err)
	}
	req := &Requirement{ProjectID: project.ID, Title: "Req2", Status: "queued"}
	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("create requirement error = %v", err)
	}

	ps := &PlanningSession{
		RequirementID:  req.ID,
		AgentSessionID: session.ID,
		Status:         "active",
	}
	if err := psRepo.Create(ctx, ps); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := psRepo.Get(ctx, ps.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.AgentSessionID != session.ID {
		t.Fatalf("AgentSessionID = %q, want %q", got.AgentSessionID, session.ID)
	}
}
