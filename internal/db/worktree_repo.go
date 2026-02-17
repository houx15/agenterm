package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type WorktreeRepo struct {
	db *sql.DB
}

func NewWorktreeRepo(db *sql.DB) *WorktreeRepo {
	return &WorktreeRepo{db: db}
}

func (r *WorktreeRepo) Create(ctx context.Context, worktree *Worktree) error {
	if worktree.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		worktree.ID = id
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO worktrees (id, project_id, branch_name, path, task_id, status)
VALUES (?, ?, ?, ?, ?, ?)
`, worktree.ID, worktree.ProjectID, worktree.BranchName, worktree.Path, nullIfEmpty(worktree.TaskID), worktree.Status)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

func (r *WorktreeRepo) Get(ctx context.Context, id string) (*Worktree, error) {
	var w Worktree
	var taskID sql.NullString
	err := r.db.QueryRowContext(ctx, `
SELECT id, project_id, branch_name, path, task_id, status
FROM worktrees
WHERE id = ?
`, id).Scan(&w.ID, &w.ProjectID, &w.BranchName, &w.Path, &taskID, &w.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get worktree %q: %w", id, err)
	}
	w.TaskID = taskID.String
	return &w, nil
}

func (r *WorktreeRepo) List(ctx context.Context, filter WorktreeFilter) ([]*Worktree, error) {
	query := `SELECT id, project_id, branch_name, path, task_id, status FROM worktrees`
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
	if filter.TaskID != "" {
		where = append(where, "task_id = ?")
		args = append(args, filter.TaskID)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY rowid DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	defer rows.Close()

	worktrees := []*Worktree{}
	for rows.Next() {
		var w Worktree
		var taskID sql.NullString
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.BranchName, &w.Path, &taskID, &w.Status); err != nil {
			return nil, fmt.Errorf("failed to scan worktree: %w", err)
		}
		w.TaskID = taskID.String
		worktrees = append(worktrees, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating worktrees: %w", err)
	}
	return worktrees, nil
}

func (r *WorktreeRepo) ListByProject(ctx context.Context, projectID string) ([]*Worktree, error) {
	return r.List(ctx, WorktreeFilter{ProjectID: projectID})
}

func (r *WorktreeRepo) Update(ctx context.Context, worktree *Worktree) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE worktrees
SET project_id = ?, branch_name = ?, path = ?, task_id = ?, status = ?
WHERE id = ?
`, worktree.ProjectID, worktree.BranchName, worktree.Path, nullIfEmpty(worktree.TaskID), worktree.Status, worktree.ID)
	if err != nil {
		return fmt.Errorf("failed to update worktree %q: %w", worktree.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for worktree %q: %w", worktree.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("worktree %q not found", worktree.ID)
	}
	return nil
}

func (r *WorktreeRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM worktrees WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete worktree %q: %w", id, err)
	}
	return nil
}
