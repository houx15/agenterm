package db

import (
	"context"
	"database/sql"
	"fmt"
)

type PermissionTemplateRepo struct {
	db *sql.DB
}

func NewPermissionTemplateRepo(db *sql.DB) *PermissionTemplateRepo {
	return &PermissionTemplateRepo{db: db}
}

func (r *PermissionTemplateRepo) Create(ctx context.Context, tmpl *PermissionTemplate) error {
	if tmpl.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		tmpl.ID = id
	}
	if tmpl.CreatedAt.IsZero() {
		tmpl.CreatedAt = nowUTC()
	}
	if tmpl.UpdatedAt.IsZero() {
		tmpl.UpdatedAt = tmpl.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO permission_templates (id, agent_type, name, config, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
`, tmpl.ID, tmpl.AgentType, tmpl.Name, tmpl.Config, formatTimestamp(tmpl.CreatedAt), formatTimestamp(tmpl.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create permission template: %w", err)
	}
	return nil
}

func (r *PermissionTemplateRepo) Get(ctx context.Context, id string) (*PermissionTemplate, error) {
	var tmpl PermissionTemplate
	var createdAtRaw, updatedAtRaw string

	err := r.db.QueryRowContext(ctx, `
SELECT id, agent_type, name, config, created_at, updated_at
FROM permission_templates
WHERE id = ?
`, id).Scan(&tmpl.ID, &tmpl.AgentType, &tmpl.Name, &tmpl.Config, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get permission template %q: %w", id, err)
	}

	tmpl.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	tmpl.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	return &tmpl, nil
}

func (r *PermissionTemplateRepo) Update(ctx context.Context, tmpl *PermissionTemplate) error {
	tmpl.UpdatedAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE permission_templates
SET agent_type = ?, name = ?, config = ?, updated_at = ?
WHERE id = ?
`, tmpl.AgentType, tmpl.Name, tmpl.Config, formatTimestamp(tmpl.UpdatedAt), tmpl.ID)
	if err != nil {
		return fmt.Errorf("failed to update permission template %q: %w", tmpl.ID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for permission template %q: %w", tmpl.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("permission template %q not found", tmpl.ID)
	}
	return nil
}

func (r *PermissionTemplateRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM permission_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete permission template %q: %w", id, err)
	}
	return nil
}

func (r *PermissionTemplateRepo) ListByAgent(ctx context.Context, agentType string) ([]*PermissionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, agent_type, name, config, created_at, updated_at
FROM permission_templates
WHERE agent_type = ?
ORDER BY created_at DESC
`, agentType)
	if err != nil {
		return nil, fmt.Errorf("failed to list permission templates by agent: %w", err)
	}
	defer rows.Close()

	return scanPermissionTemplates(rows)
}

func (r *PermissionTemplateRepo) List(ctx context.Context) ([]*PermissionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, agent_type, name, config, created_at, updated_at
FROM permission_templates
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, fmt.Errorf("failed to list permission templates: %w", err)
	}
	defer rows.Close()

	return scanPermissionTemplates(rows)
}

func scanPermissionTemplates(rows *sql.Rows) ([]*PermissionTemplate, error) {
	templates := []*PermissionTemplate{}
	for rows.Next() {
		var tmpl PermissionTemplate
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&tmpl.ID, &tmpl.AgentType, &tmpl.Name, &tmpl.Config, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan permission template: %w", err)
		}
		var err error
		tmpl.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		tmpl.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		templates = append(templates, &tmpl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating permission templates: %w", err)
	}

	return templates, nil
}
