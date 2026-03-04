package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type ProjectRepo struct {
	db *sql.DB
}

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) Create(ctx context.Context, project *Project) error {
	if project.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		project.ID = id
	}
	if project.CreatedAt.IsZero() {
		project.CreatedAt = nowUTC()
	}
	if project.UpdatedAt.IsZero() {
		project.UpdatedAt = project.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO projects (id, name, repo_path, status, playbook, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, project.ID, project.Name, project.RepoPath, project.Status, project.Playbook, formatTimestamp(project.CreatedAt), formatTimestamp(project.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

func (r *ProjectRepo) Get(ctx context.Context, id string) (*Project, error) {
	var p Project
	var createdAtRaw, updatedAtRaw string

	err := r.db.QueryRowContext(ctx, `
SELECT id, name, repo_path, status, playbook, created_at, updated_at
FROM projects
WHERE id = ?
`, id).Scan(&p.ID, &p.Name, &p.RepoPath, &p.Status, &p.Playbook, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project %q: %w", id, err)
	}

	p.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *ProjectRepo) List(ctx context.Context, filter ProjectFilter) ([]*Project, error) {
	query := `SELECT id, name, repo_path, status, playbook, created_at, updated_at FROM projects`
	args := []any{}
	where := []string{}
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
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	projects := []*Project{}
	for rows.Next() {
		var p Project
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&p.ID, &p.Name, &p.RepoPath, &p.Status, &p.Playbook, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		p.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		p.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating projects: %w", err)
	}

	return projects, nil
}

func (r *ProjectRepo) ListByStatus(ctx context.Context, status string) ([]*Project, error) {
	return r.List(ctx, ProjectFilter{Status: status})
}

func (r *ProjectRepo) Update(ctx context.Context, project *Project) error {
	project.UpdatedAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE projects
SET name = ?, repo_path = ?, status = ?, playbook = ?, updated_at = ?
WHERE id = ?
`, project.Name, project.RepoPath, project.Status, project.Playbook, formatTimestamp(project.UpdatedAt), project.ID)
	if err != nil {
		return fmt.Errorf("failed to update project %q: %w", project.ID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for project %q: %w", project.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("project %q not found", project.ID)
	}
	return nil
}

func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete project %q: %w", id, err)
	}
	return nil
}
