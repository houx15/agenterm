package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func openTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agenterm-test.db")
	database, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return database, path
}

func assertTableExists(t *testing.T, conn *sql.DB, table string) {
	t.Helper()
	var count int
	err := conn.QueryRow(`SELECT count(1) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master error: %v", err)
	}
	if count != 1 {
		t.Fatalf("table %q not found", table)
	}
}

func TestOpenCreatesDBFileAndRunsMigrations(t *testing.T) {
	database, path := openTestDB(t)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected DB file at %q: %v", path, err)
	}

	assertTableExists(t, database.SQL(), "_meta")
	assertTableExists(t, database.SQL(), "projects")
	assertTableExists(t, database.SQL(), "tasks")
	assertTableExists(t, database.SQL(), "worktrees")
	assertTableExists(t, database.SQL(), "sessions")
	assertTableExists(t, database.SQL(), "agent_configs")
	assertTableExists(t, database.SQL(), "orchestrator_messages")
	assertTableExists(t, database.SQL(), "workflows")
	assertTableExists(t, database.SQL(), "workflow_phases")
	assertTableExists(t, database.SQL(), "project_orchestrators")
	assertTableExists(t, database.SQL(), "role_bindings")
	assertTableExists(t, database.SQL(), "project_knowledge_entries")
	assertTableExists(t, database.SQL(), "review_cycles")
	assertTableExists(t, database.SQL(), "review_issues")
	assertTableExists(t, database.SQL(), "demand_pool_items")
}

func TestMigrationsAreIdempotent(t *testing.T) {
	database, _ := openTestDB(t)

	if err := RunMigrations(context.Background(), database.SQL()); err != nil {
		t.Fatalf("second RunMigrations() error = %v", err)
	}

	var version string
	if err := database.SQL().QueryRow(`SELECT value FROM _meta WHERE key='schema_version'`).Scan(&version); err != nil {
		t.Fatalf("read schema version error = %v", err)
	}
	if version != "4" {
		t.Fatalf("schema version = %s, want 4", version)
	}
}

func TestProjectRepoCRUDAndListByStatus(t *testing.T) {
	database, _ := openTestDB(t)
	repo := NewProjectRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "AgenTerm", RepoPath: "/tmp/agenterm", Status: "active", Playbook: "default"}
	if err := repo.Create(ctx, project); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if project.ID == "" {
		t.Fatal("Create() did not set project ID")
	}

	got, err := repo.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || got.Name != "AgenTerm" || got.Status != "active" {
		t.Fatalf("Get() got = %#v", got)
	}

	activeList, err := repo.ListByStatus(ctx, "active")
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(activeList) != 1 {
		t.Fatalf("ListByStatus(active) len = %d, want 1", len(activeList))
	}

	project.Status = "paused"
	if err := repo.Update(ctx, project); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	pausedList, err := repo.List(ctx, ProjectFilter{Status: "paused"})
	if err != nil {
		t.Fatalf("List(paused) error = %v", err)
	}
	if len(pausedList) != 1 || pausedList[0].ID != project.ID {
		t.Fatalf("List(paused) got = %#v", pausedList)
	}

	if err := repo.Delete(ctx, project.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	deleted, err := repo.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if deleted != nil {
		t.Fatalf("Get() after delete = %#v, want nil", deleted)
	}
}

func TestTaskRepoCRUDAndJSONDependsOn(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	taskRepo := NewTaskRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "P1", RepoPath: "/tmp/p1", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}

	task := &Task{
		ProjectID:   project.ID,
		Title:       "Implement DB",
		Description: "Create schema",
		Status:      "pending",
		DependsOn:   []string{"task-a", "task-b"},
		SpecPath:    "docs/TASK-07-database-models.md",
	}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := taskRepo.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if !reflect.DeepEqual(got.DependsOn, []string{"task-a", "task-b"}) {
		t.Fatalf("DependsOn = %#v, want [task-a task-b]", got.DependsOn)
	}

	byProject, err := taskRepo.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(byProject) != 1 {
		t.Fatalf("ListByProject len = %d, want 1", len(byProject))
	}

	byStatus, err := taskRepo.ListByStatus(ctx, project.ID, "pending")
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(byStatus) != 1 {
		t.Fatalf("ListByStatus len = %d, want 1", len(byStatus))
	}

	task.Status = "running"
	task.DependsOn = []string{"task-a"}
	if err := taskRepo.Update(ctx, task); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := taskRepo.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Status != "running" || !reflect.DeepEqual(updated.DependsOn, []string{"task-a"}) {
		t.Fatalf("updated task = %#v", updated)
	}

	if err := taskRepo.Delete(ctx, task.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestWorktreeRepoCRUDAndListByProject(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	worktreeRepo := NewWorktreeRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "P2", RepoPath: "/tmp/p2", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}

	worktree := &Worktree{
		ProjectID:  project.ID,
		BranchName: "feature/database",
		Path:       "/tmp/p2-worktree",
		Status:     "active",
	}
	if err := worktreeRepo.Create(ctx, worktree); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := worktreeRepo.Get(ctx, worktree.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || got.BranchName != "feature/database" {
		t.Fatalf("Get() got = %#v", got)
	}

	list, err := worktreeRepo.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByProject len = %d, want 1", len(list))
	}

	worktree.Status = "merged"
	if err := worktreeRepo.Update(ctx, worktree); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if err := worktreeRepo.Delete(ctx, worktree.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestSessionRepoCRUDAndFilters(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	taskRepo := NewTaskRepo(database.SQL())
	sessionRepo := NewSessionRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "P3", RepoPath: "/tmp/p3", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}
	task := &Task{ProjectID: project.ID, Title: "T", Description: "D", Status: "pending"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task error = %v", err)
	}

	session := &Session{
		TaskID:          task.ID,
		TmuxSessionName: "ai-coding",
		TmuxWindowID:    "@1",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "running",
		HumanAttached:   true,
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := sessionRepo.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || !got.HumanAttached || got.Status != "running" {
		t.Fatalf("Get() got = %#v", got)
	}

	byTask, err := sessionRepo.ListByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListByTask() error = %v", err)
	}
	if len(byTask) != 1 {
		t.Fatalf("ListByTask len = %d, want 1", len(byTask))
	}

	active, err := sessionRepo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("ListActive len = %d, want 1", len(active))
	}

	session.Status = "completed"
	session.HumanAttached = false
	if err := sessionRepo.Update(ctx, session); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	activeAfter, err := sessionRepo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive() after update error = %v", err)
	}
	if len(activeAfter) != 0 {
		t.Fatalf("ListActive after update len = %d, want 0", len(activeAfter))
	}

	if err := sessionRepo.Delete(ctx, session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestDemandPoolRepoCRUDAndFilters(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	demandRepo := NewDemandPoolRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "Demand Project", RepoPath: "/tmp/demand", Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project error = %v", err)
	}

	item := &DemandPoolItem{
		ProjectID:   project.ID,
		Title:       "Feature A",
		Description: "Improve onboarding",
		Status:      "captured",
		Priority:    3,
		Impact:      4,
		Effort:      2,
		Risk:        1,
		Urgency:     5,
		Tags:        []string{"onboarding", "ux"},
		Source:      "user",
	}
	if err := demandRepo.Create(ctx, item); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := demandRepo.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || got.Title != "Feature A" {
		t.Fatalf("Get() got = %#v", got)
	}

	list, err := demandRepo.List(ctx, DemandPoolFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}

	filtered, err := demandRepo.List(ctx, DemandPoolFilter{ProjectID: project.ID, Tag: "ux"})
	if err != nil {
		t.Fatalf("List(tag) error = %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("List(tag) len = %d, want 1", len(filtered))
	}

	item.Status = "triaged"
	item.Priority = 5
	if err := demandRepo.Update(ctx, item); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := demandRepo.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Status != "triaged" || updated.Priority != 5 {
		t.Fatalf("updated item = %#v", updated)
	}

	if err := demandRepo.Delete(ctx, item.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	deleted, err := demandRepo.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if deleted != nil {
		t.Fatalf("Get() after delete = %#v, want nil", deleted)
	}
}

func TestAgentConfigRepoCRUDAndYAMLLoad(t *testing.T) {
	database, _ := openTestDB(t)
	repo := NewAgentConfigRepo(database.SQL())
	ctx := context.Background()

	agent := &AgentConfig{
		ID:                    "codex",
		Name:                  "Codex",
		Command:               "codex",
		ResumeCommand:         "codex resume",
		HeadlessCommand:       "codex --headless",
		Capabilities:          []string{"coding", "review"},
		Languages:             []string{"go", "typescript"},
		CostTier:              "high",
		SpeedTier:             "medium",
		SupportsSessionResume: true,
		SupportsHeadless:      true,
		AutoAcceptMode:        "optional",
	}
	if err := repo.Create(ctx, agent); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.Get(ctx, "codex")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || !reflect.DeepEqual(got.Capabilities, []string{"coding", "review"}) {
		t.Fatalf("Get() got = %#v", got)
	}

	agent.SpeedTier = "fast"
	agent.Capabilities = []string{"coding"}
	if err := repo.Update(ctx, agent); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	fastAgents, err := repo.List(ctx, AgentConfigFilter{SpeedTier: "fast"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(fastAgents) != 1 || fastAgents[0].ID != "codex" {
		t.Fatalf("List(fast) got = %#v", fastAgents)
	}

	yamlDir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(yamlDir, 0o755); err != nil {
		t.Fatalf("mkdir yamlDir error = %v", err)
	}
	yamlFile := filepath.Join(yamlDir, "claude.yaml")
	yamlData := []byte("id: claude\nname: Claude\ncommand: claude\ncapabilities:\n  - coding\nlanguages:\n  - go\ncost_tier: medium\nspeed_tier: medium\nsupports_session_resume: true\nsupports_headless: false\n")
	if err := os.WriteFile(yamlFile, yamlData, 0o600); err != nil {
		t.Fatalf("write yaml file error = %v", err)
	}

	loaded, err := repo.LoadFromYAMLDir(ctx, yamlDir)
	if err != nil {
		t.Fatalf("LoadFromYAMLDir() error = %v", err)
	}
	if loaded != 1 {
		t.Fatalf("LoadFromYAMLDir count = %d, want 1", loaded)
	}

	claude, err := repo.Get(ctx, "claude")
	if err != nil {
		t.Fatalf("Get(claude) error = %v", err)
	}
	if claude == nil || claude.Name != "Claude" {
		t.Fatalf("Get(claude) got = %#v", claude)
	}

	if err := repo.Delete(ctx, "codex"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestNewIDUniqueness(t *testing.T) {
	ids := make(map[string]struct{}, 2000)
	for i := 0; i < 2000; i++ {
		id, err := NewID()
		if err != nil {
			t.Fatalf("NewID() error = %v", err)
		}
		if _, exists := ids[id]; exists {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = struct{}{}
	}
}
