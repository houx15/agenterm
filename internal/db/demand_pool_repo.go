package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type DemandPoolRepo struct {
	db *sql.DB
}

func NewDemandPoolRepo(db *sql.DB) *DemandPoolRepo {
	return &DemandPoolRepo{db: db}
}

func (r *DemandPoolRepo) Create(ctx context.Context, item *DemandPoolItem) error {
	if item.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		item.ID = id
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = nowUTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "captured"
	}
	tagsRaw, err := encodeStringSlice(item.Tags)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO demand_pool_items (
  id, project_id, title, description, status, priority, impact, effort, risk, urgency,
  tags, source, created_by, selected_task_id, notes, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, item.ID, item.ProjectID, item.Title, item.Description, item.Status, item.Priority, item.Impact, item.Effort, item.Risk, item.Urgency,
		tagsRaw, item.Source, item.CreatedBy, nullIfEmpty(item.SelectedTaskID), item.Notes, formatTimestamp(item.CreatedAt), formatTimestamp(item.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create demand pool item: %w", err)
	}
	return nil
}

func (r *DemandPoolRepo) Get(ctx context.Context, id string) (*DemandPoolItem, error) {
	var item DemandPoolItem
	var tagsRaw, createdAtRaw, updatedAtRaw string
	var selectedTaskID sql.NullString
	err := r.db.QueryRowContext(ctx, `
SELECT id, project_id, title, description, status, priority, impact, effort, risk, urgency,
       tags, source, created_by, selected_task_id, notes, created_at, updated_at
FROM demand_pool_items
WHERE id = ?
`, id).Scan(
		&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.Priority, &item.Impact, &item.Effort, &item.Risk, &item.Urgency,
		&tagsRaw, &item.Source, &item.CreatedBy, &selectedTaskID, &item.Notes, &createdAtRaw, &updatedAtRaw,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get demand pool item %q: %w", id, err)
	}
	item.Tags, err = decodeStringSlice(tagsRaw)
	if err != nil {
		return nil, err
	}
	item.SelectedTaskID = selectedTaskID.String
	item.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *DemandPoolRepo) List(ctx context.Context, filter DemandPoolFilter) ([]*DemandPoolItem, error) {
	query := `
SELECT id, project_id, title, description, status, priority, impact, effort, risk, urgency,
       tags, source, created_by, selected_task_id, notes, created_at, updated_at
FROM demand_pool_items`
	where := []string{}
	args := []any{}

	if filter.ProjectID != "" {
		where = append(where, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if tag := strings.TrimSpace(filter.Tag); tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, `%`+`"`+tag+`"`+`%`)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, "(title LIKE ? OR description LIKE ?)")
		needle := "%" + q + "%"
		args = append(args, needle, needle)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY priority DESC, created_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list demand pool items: %w", err)
	}
	defer rows.Close()

	items := make([]*DemandPoolItem, 0)
	for rows.Next() {
		var item DemandPoolItem
		var tagsRaw, createdAtRaw, updatedAtRaw string
		var selectedTaskID sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.Priority, &item.Impact, &item.Effort, &item.Risk, &item.Urgency,
			&tagsRaw, &item.Source, &item.CreatedBy, &selectedTaskID, &item.Notes, &createdAtRaw, &updatedAtRaw,
		); err != nil {
			return nil, fmt.Errorf("failed to scan demand pool item: %w", err)
		}
		item.Tags, err = decodeStringSlice(tagsRaw)
		if err != nil {
			return nil, err
		}
		item.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		item.SelectedTaskID = selectedTaskID.String
		item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating demand pool items: %w", err)
	}
	return items, nil
}

func (r *DemandPoolRepo) Update(ctx context.Context, item *DemandPoolItem) error {
	item.UpdatedAt = nowUTC()
	tagsRaw, err := encodeStringSlice(item.Tags)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE demand_pool_items
SET project_id = ?, title = ?, description = ?, status = ?, priority = ?, impact = ?, effort = ?, risk = ?, urgency = ?,
    tags = ?, source = ?, created_by = ?, selected_task_id = ?, notes = ?, updated_at = ?
WHERE id = ?
`, item.ProjectID, item.Title, item.Description, item.Status, item.Priority, item.Impact, item.Effort, item.Risk, item.Urgency,
		tagsRaw, item.Source, item.CreatedBy, nullIfEmpty(item.SelectedTaskID), item.Notes, formatTimestamp(item.UpdatedAt), item.ID)
	if err != nil {
		return fmt.Errorf("failed to update demand pool item %q: %w", item.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for demand pool item %q: %w", item.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("demand pool item %q not found", item.ID)
	}
	return nil
}

func (r *DemandPoolRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM demand_pool_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete demand pool item %q: %w", id, err)
	}
	return nil
}
