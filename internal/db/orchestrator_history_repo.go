package db

import (
	"context"
	"database/sql"
	"fmt"
)

type OrchestratorMessage struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type OrchestratorHistoryRepo struct {
	db *sql.DB
}

func NewOrchestratorHistoryRepo(db *sql.DB) *OrchestratorHistoryRepo {
	return &OrchestratorHistoryRepo{db: db}
}

func (r *OrchestratorHistoryRepo) Create(ctx context.Context, msg *OrchestratorMessage) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("orchestrator history repo unavailable")
	}
	if msg == nil {
		return fmt.Errorf("message is required")
	}
	if msg.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if msg.Role == "" {
		return fmt.Errorf("role is required")
	}
	if msg.Content == "" {
		return fmt.Errorf("content is required")
	}
	if msg.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		msg.ID = id
	}
	if msg.CreatedAt == "" {
		msg.CreatedAt = formatTimestamp(nowUTC())
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO orchestrator_messages (id, project_id, role, content, created_at)
VALUES (?, ?, ?, ?, ?)
`, msg.ID, msg.ProjectID, msg.Role, msg.Content, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert orchestrator message: %w", err)
	}
	return nil
}

func (r *OrchestratorHistoryRepo) ListByProject(ctx context.Context, projectID string, limit int) ([]*OrchestratorMessage, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("orchestrator history repo unavailable")
	}
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, project_id, role, content, created_at
FROM orchestrator_messages
WHERE project_id = ?
ORDER BY created_at DESC
LIMIT ?
`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list orchestrator messages: %w", err)
	}
	defer rows.Close()

	items := make([]*OrchestratorMessage, 0)
	for rows.Next() {
		msg := &OrchestratorMessage{}
		if err := rows.Scan(&msg.ID, &msg.ProjectID, &msg.Role, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan orchestrator message: %w", err)
		}
		items = append(items, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orchestrator messages: %w", err)
	}

	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

func (r *OrchestratorHistoryRepo) TrimProject(ctx context.Context, projectID string, keep int) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("orchestrator history repo unavailable")
	}
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if keep <= 0 {
		keep = 50
	}
	_, err := r.db.ExecContext(ctx, `
DELETE FROM orchestrator_messages
WHERE project_id = ?
  AND id NOT IN (
    SELECT id FROM orchestrator_messages
    WHERE project_id = ?
    ORDER BY created_at DESC
    LIMIT ?
  )
`, projectID, projectID, keep)
	if err != nil {
		return fmt.Errorf("trim orchestrator messages: %w", err)
	}
	return nil
}
