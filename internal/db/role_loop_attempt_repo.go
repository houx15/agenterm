package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type RoleLoopAttemptRepo struct {
	db *sql.DB
}

func NewRoleLoopAttemptRepo(db *sql.DB) *RoleLoopAttemptRepo {
	return &RoleLoopAttemptRepo{db: db}
}

func (r *RoleLoopAttemptRepo) Increment(ctx context.Context, taskID string, roleName string) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("role loop attempt repo unavailable")
	}
	taskID = strings.TrimSpace(taskID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if taskID == "" {
		return 0, fmt.Errorf("task id is required")
	}
	if roleName == "" {
		return 0, fmt.Errorf("role name is required")
	}
	now := formatTimestamp(nowUTC())
	_, err := r.db.ExecContext(ctx, `
INSERT INTO role_loop_attempts (task_id, role_name, attempts, updated_at)
VALUES (?, ?, 1, ?)
ON CONFLICT(task_id, role_name) DO UPDATE SET
	attempts = role_loop_attempts.attempts + 1,
	updated_at = excluded.updated_at
`, taskID, roleName, now)
	if err != nil {
		return 0, fmt.Errorf("increment role loop attempt: %w", err)
	}
	return r.GetAttempt(ctx, taskID, roleName)
}

func (r *RoleLoopAttemptRepo) GetAttempt(ctx context.Context, taskID string, roleName string) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("role loop attempt repo unavailable")
	}
	taskID = strings.TrimSpace(taskID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if taskID == "" {
		return 0, fmt.Errorf("task id is required")
	}
	if roleName == "" {
		return 0, fmt.Errorf("role name is required")
	}
	var attempts int
	err := r.db.QueryRowContext(ctx, `
SELECT attempts
FROM role_loop_attempts
WHERE task_id = ? AND role_name = ?
`, taskID, roleName).Scan(&attempts)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("get role loop attempt: %w", err)
	}
	return attempts, nil
}

func (r *RoleLoopAttemptRepo) GetTaskAttempts(ctx context.Context, taskID string) (map[string]int, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("role loop attempt repo unavailable")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT role_name, attempts
FROM role_loop_attempts
WHERE task_id = ?
`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list role loop attempts: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var roleName string
		var attempts int
		if err := rows.Scan(&roleName, &attempts); err != nil {
			return nil, fmt.Errorf("scan role loop attempt: %w", err)
		}
		out[strings.ToLower(strings.TrimSpace(roleName))] = attempts
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role loop attempts: %w", err)
	}
	return out, nil
}
