package db

import (
	"context"
	"database/sql"
	"fmt"
)

type RequirementRepo struct {
	db *sql.DB
}

func NewRequirementRepo(db *sql.DB) *RequirementRepo {
	return &RequirementRepo{db: db}
}

func (r *RequirementRepo) Create(ctx context.Context, req *Requirement) error {
	if req.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		req.ID = id
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = nowUTC()
	}
	if req.UpdatedAt.IsZero() {
		req.UpdatedAt = req.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO requirements (id, project_id, title, description, priority, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, req.ID, req.ProjectID, req.Title, req.Description, req.Priority, req.Status, formatTimestamp(req.CreatedAt), formatTimestamp(req.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create requirement: %w", err)
	}
	return nil
}

func (r *RequirementRepo) Get(ctx context.Context, id string) (*Requirement, error) {
	var req Requirement
	var createdAtRaw, updatedAtRaw string

	err := r.db.QueryRowContext(ctx, `
SELECT id, project_id, title, description, priority, status, created_at, updated_at
FROM requirements
WHERE id = ?
`, id).Scan(&req.ID, &req.ProjectID, &req.Title, &req.Description, &req.Priority, &req.Status, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get requirement %q: %w", id, err)
	}

	req.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	req.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (r *RequirementRepo) Update(ctx context.Context, req *Requirement) error {
	req.UpdatedAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE requirements
SET title = ?, description = ?, status = ?, priority = ?, updated_at = ?
WHERE id = ?
`, req.Title, req.Description, req.Status, req.Priority, formatTimestamp(req.UpdatedAt), req.ID)
	if err != nil {
		return fmt.Errorf("failed to update requirement %q: %w", req.ID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for requirement %q: %w", req.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("requirement %q not found", req.ID)
	}
	return nil
}

func (r *RequirementRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM requirements WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete requirement %q: %w", id, err)
	}
	return nil
}

func (r *RequirementRepo) ListByProject(ctx context.Context, projectID string) ([]*Requirement, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, project_id, title, description, priority, status, created_at, updated_at
FROM requirements
WHERE project_id = ?
ORDER BY priority ASC
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list requirements: %w", err)
	}
	defer rows.Close()

	reqs := []*Requirement{}
	for rows.Next() {
		var req Requirement
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&req.ID, &req.ProjectID, &req.Title, &req.Description, &req.Priority, &req.Status, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan requirement: %w", err)
		}
		req.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		req.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, &req)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating requirements: %w", err)
	}

	return reqs, nil
}

func (r *RequirementRepo) Reorder(ctx context.Context, projectID string, orderedIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start reorder transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `UPDATE requirements SET priority = ?, updated_at = ? WHERE id = ? AND project_id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare reorder statement: %w", err)
	}
	defer stmt.Close()

	now := formatTimestamp(nowUTC())
	for i, id := range orderedIDs {
		res, err := stmt.ExecContext(ctx, i, now, id, projectID)
		if err != nil {
			return fmt.Errorf("failed to reorder requirement %q: %w", id, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to read affected rows for requirement %q: %w", id, err)
		}
		if affected == 0 {
			return fmt.Errorf("requirement %q not found in project %q", id, projectID)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit reorder: %w", err)
	}
	return nil
}
