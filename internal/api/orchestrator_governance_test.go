package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/user/agenterm/internal/db"
)

func TestHandlerShouldNotifyOnBlockedHonorsProjectPreference(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "governance-test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()

	projectRepo := db.NewProjectRepo(database.SQL())
	profileRepo := db.NewProjectOrchestratorRepo(database.SQL())
	project := &db.Project{Name: "P", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := profileRepo.EnsureDefaultForProject(ctx, project.ID); err != nil {
		t.Fatalf("ensure default orchestrator: %v", err)
	}
	profile, err := profileRepo.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	profile.NotifyOnBlocked = false
	if err := profileRepo.Update(ctx, profile); err != nil {
		t.Fatalf("update profile: %v", err)
	}

	h := &handler{projectOrchestratorRepo: profileRepo}
	if got := h.shouldNotifyOnBlocked(ctx, project.ID); got {
		t.Fatalf("shouldNotifyOnBlocked=%v want false", got)
	}
	if got := h.shouldNotifyOnBlocked(ctx, "missing-project"); !got {
		t.Fatalf("missing profile should default to true")
	}
}
