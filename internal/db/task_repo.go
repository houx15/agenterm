package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type TaskRepo struct {
	db *sql.DB
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(ctx context.Context, task *Task) error {
	if task.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		task.ID = id
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = nowUTC()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}

	dependsOnRaw, err := encodeStringSlice(task.DependsOn)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO tasks (id, project_id, title, description, status, depends_on, worktree_id, spec_path, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, task.ID, task.ProjectID, task.Title, task.Description, task.Status, dependsOnRaw, task.WorktreeID, task.SpecPath, formatTimestamp(task.CreatedAt), formatTimestamp(task.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

func (r *TaskRepo) Get(ctx context.Context, id string) (*Task, error) {
	var t Task
	var dependsOnRaw, createdAtRaw, updatedAtRaw string

	err := r.db.QueryRowContext(ctx, `
SELECT id, project_id, title, description, status, depends_on, worktree_id, spec_path, created_at, updated_at
FROM tasks
WHERE id = ?
`, id).Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &dependsOnRaw, &t.WorktreeID, &t.SpecPath, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task %q: %w", id, err)
	}

	t.DependsOn, err = decodeStringSlice(dependsOnRaw)
	if err != nil {
		return nil, err
	}
	t.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	t.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *TaskRepo) List(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	query := `SELECT id, project_id, title, description, status, depends_on, worktree_id, spec_path, created_at, updated_at FROM tasks`
	args := []any{}
	where := []string{}

	if filter.ProjectID != "" {
		where = append(where, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []*Task{}
	for rows.Next() {
		var t Task
		var dependsOnRaw, createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &dependsOnRaw, &t.WorktreeID, &t.SpecPath, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		t.DependsOn, err = decodeStringSlice(dependsOnRaw)
		if err != nil {
			return nil, err
		}
		t.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		t.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating tasks: %w", err)
	}

	return tasks, nil
}

func (r *TaskRepo) ListByProject(ctx context.Context, projectID string) ([]*Task, error) {
	return r.List(ctx, TaskFilter{ProjectID: projectID})
}

func (r *TaskRepo) ListByStatus(ctx context.Context, projectID, status string) ([]*Task, error) {
	return r.List(ctx, TaskFilter{ProjectID: projectID, Status: status})
}

func (r *TaskRepo) Update(ctx context.Context, task *Task) error {
	task.UpdatedAt = nowUTC()
	dependsOnRaw, err := encodeStringSlice(task.DependsOn)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE tasks
SET project_id = ?, title = ?, description = ?, status = ?, depends_on = ?, worktree_id = ?, spec_path = ?, updated_at = ?
WHERE id = ?
`, task.ProjectID, task.Title, task.Description, task.Status, dependsOnRaw, task.WorktreeID, task.SpecPath, formatTimestamp(task.UpdatedAt), task.ID)
	if err != nil {
		return fmt.Errorf("failed to update task %q: %w", task.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for task %q: %w", task.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("task %q not found", task.ID)
	}
	return nil
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete task %q: %w", id, err)
	}
	return nil
}
