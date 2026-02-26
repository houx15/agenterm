package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type RoleAgentAssignmentRepo struct {
	db *sql.DB
}

func NewRoleAgentAssignmentRepo(db *sql.DB) *RoleAgentAssignmentRepo {
	return &RoleAgentAssignmentRepo{db: db}
}

func (r *RoleAgentAssignmentRepo) ListByProject(ctx context.Context, projectID string) ([]*RoleAgentAssignment, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, project_id, stage, role, agent_type, max_parallel, created_at, updated_at
FROM role_agent_assignments
WHERE project_id = ?
ORDER BY stage ASC, role ASC
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list role agent assignments: %w", err)
	}
	defer rows.Close()
	out := make([]*RoleAgentAssignment, 0)
	for rows.Next() {
		var item RoleAgentAssignment
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(
			&item.ID,
			&item.ProjectID,
			&item.Stage,
			&item.Role,
			&item.AgentType,
			&item.MaxParallel,
			&createdAtRaw,
			&updatedAtRaw,
		); err != nil {
			return nil, fmt.Errorf("scan role agent assignment: %w", err)
		}
		item.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role agent assignments: %w", err)
	}
	return out, nil
}

func (r *RoleAgentAssignmentRepo) ReplaceForProject(ctx context.Context, projectID string, items []*RoleAgentAssignment) error {
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("project id is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin role agent assignment tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_agent_assignments WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("clear role agent assignments: %w", err)
	}

	now := nowUTC()
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.Role) == "" || strings.TrimSpace(item.AgentType) == "" {
			return fmt.Errorf("role and agent_type are required")
		}
		if item.ID == "" {
			id, err := NewID()
			if err != nil {
				return err
			}
			item.ID = id
		}
		if item.MaxParallel <= 0 {
			item.MaxParallel = 1
		}
		item.ProjectID = projectID
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		item.UpdatedAt = now
		_, err := tx.ExecContext(ctx, `
INSERT INTO role_agent_assignments (id, project_id, stage, role, agent_type, max_parallel, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`,
			item.ID,
			item.ProjectID,
			strings.ToLower(strings.TrimSpace(item.Stage)),
			strings.TrimSpace(item.Role),
			strings.TrimSpace(item.AgentType),
			item.MaxParallel,
			formatTimestamp(item.CreatedAt),
			formatTimestamp(item.UpdatedAt),
		)
		if err != nil {
			return fmt.Errorf("insert role agent assignment: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit role agent assignment tx: %w", err)
	}
	return nil
}
