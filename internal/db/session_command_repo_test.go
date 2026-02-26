package db

import (
	"context"
	"testing"
	"time"
)

func TestSessionCommandRepoCRUD(t *testing.T) {
	database, _ := openTestDB(t)
	projectRepo := NewProjectRepo(database.SQL())
	taskRepo := NewTaskRepo(database.SQL())
	sessionRepo := NewSessionRepo(database.SQL())
	cmdRepo := NewSessionCommandRepo(database.SQL())
	ctx := context.Background()

	project := &Project{Name: "P", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &Task{
		ProjectID:   project.ID,
		Title:       "T",
		Description: "D",
		Status:      "pending",
	}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	session := &Session{
		TaskID:          task.ID,
		TmuxSessionName: "s1",
		TmuxWindowID:    "@1",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "working",
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := &SessionCommand{
		SessionID:   session.ID,
		Op:          "send_text",
		PayloadJSON: `{"text":"echo hello\n"}`,
		Status:      "queued",
	}
	if err := cmdRepo.Create(ctx, cmd); err != nil {
		t.Fatalf("create command: %v", err)
	}
	if cmd.ID == "" {
		t.Fatalf("expected command id")
	}

	got, err := cmdRepo.Get(ctx, cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got == nil || got.Op != "send_text" || got.Status != "queued" {
		t.Fatalf("unexpected command: %#v", got)
	}

	got.Status = "completed"
	got.SentAt = time.Now().UTC().Add(-2 * time.Second)
	got.AckedAt = got.SentAt.Add(time.Second)
	got.CompletedAt = got.AckedAt.Add(time.Second)
	got.ResultJSON = `{"status":"sent"}`
	if err := cmdRepo.Update(ctx, got); err != nil {
		t.Fatalf("update command: %v", err)
	}

	list, err := cmdRepo.ListBySession(ctx, session.ID, 10)
	if err != nil {
		t.Fatalf("list commands: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list)=%d want 1", len(list))
	}
	if list[0].Status != "completed" {
		t.Fatalf("status=%q want completed", list[0].Status)
	}
}
