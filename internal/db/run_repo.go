package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type RunRepo struct {
	db *sql.DB
}

func NewRunRepo(db *sql.DB) *RunRepo {
	return &RunRepo{db: db}
}

func (r *RunRepo) GetActiveByProject(ctx context.Context, projectID string) (*ProjectRun, error) {
	var item ProjectRun
	var createdAtRaw, updatedAtRaw string
	err := r.db.QueryRowContext(ctx, `
SELECT id, project_id, status, current_stage, trigger, created_at, updated_at
FROM project_runs
WHERE project_id = ? AND status = 'active'
ORDER BY updated_at DESC
LIMIT 1
`, projectID).Scan(&item.ID, &item.ProjectID, &item.Status, &item.CurrentStage, &item.Trigger, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active project run: %w", err)
	}
	var parseErr error
	item.CreatedAt, parseErr = parseTimestamp(createdAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	item.UpdatedAt, parseErr = parseTimestamp(updatedAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	return &item, nil
}

func (r *RunRepo) Get(ctx context.Context, runID string) (*ProjectRun, error) {
	var item ProjectRun
	var createdAtRaw, updatedAtRaw string
	err := r.db.QueryRowContext(ctx, `
SELECT id, project_id, status, current_stage, trigger, created_at, updated_at
FROM project_runs
WHERE id = ?
`, runID).Scan(&item.ID, &item.ProjectID, &item.Status, &item.CurrentStage, &item.Trigger, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get project run: %w", err)
	}
	var parseErr error
	item.CreatedAt, parseErr = parseTimestamp(createdAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	item.UpdatedAt, parseErr = parseTimestamp(updatedAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	return &item, nil
}

func (r *RunRepo) EnsureActive(ctx context.Context, projectID string, stage string, trigger string) (*ProjectRun, error) {
	existing, err := r.GetActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	now := nowUTC()
	id, err := NewID()
	if err != nil {
		return nil, err
	}
	stage = normalizeStage(stage)
	if stage == "" {
		stage = "plan"
	}
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}
	item := &ProjectRun{
		ID:           id,
		ProjectID:    projectID,
		Status:       "active",
		CurrentStage: stage,
		Trigger:      trigger,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO project_runs (id, project_id, status, current_stage, trigger, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, item.ID, item.ProjectID, item.Status, item.CurrentStage, item.Trigger, formatTimestamp(item.CreatedAt), formatTimestamp(item.UpdatedAt))
	if err != nil {
		return nil, fmt.Errorf("create project run: %w", err)
	}
	return item, nil
}

func (r *RunRepo) UpdateStage(ctx context.Context, runID string, stage string, status string) error {
	stage = normalizeStage(stage)
	if stage == "" {
		return fmt.Errorf("stage is required")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "active"
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE project_runs
SET current_stage = ?, status = ?, updated_at = ?
WHERE id = ?
`, stage, status, formatTimestamp(nowUTC()), runID)
	if err != nil {
		return fmt.Errorf("update project run stage: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project run rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project run not found")
	}
	return nil
}

func (r *RunRepo) UpsertStageRun(ctx context.Context, runID string, stage string, status string, evidenceJSON string) error {
	stage = normalizeStage(stage)
	if stage == "" {
		return fmt.Errorf("stage is required")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "active"
	}
	evidenceJSON = strings.TrimSpace(evidenceJSON)
	now := nowUTC()

	existing, err := r.GetStageRunByStage(ctx, runID, stage)
	if err != nil {
		return err
	}
	if existing == nil {
		id, err := NewID()
		if err != nil {
			return err
		}
		endedAt := ""
		if status == "completed" || status == "failed" || status == "blocked" {
			endedAt = formatTimestamp(now)
		}
		_, err = r.db.ExecContext(ctx, `
INSERT INTO stage_runs (id, run_id, stage, status, evidence_json, started_at, ended_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, id, runID, stage, status, evidenceJSON, formatTimestamp(now), endedAt, formatTimestamp(now))
		if err != nil {
			return fmt.Errorf("create stage run: %w", err)
		}
		return nil
	}

	endedAt := ""
	if status == "completed" || status == "failed" || status == "blocked" {
		endedAt = formatTimestamp(now)
	}
	_, err = r.db.ExecContext(ctx, `
UPDATE stage_runs
SET status = ?, evidence_json = ?, ended_at = ?, updated_at = ?
WHERE id = ?
`, status, evidenceJSON, endedAt, formatTimestamp(now), existing.ID)
	if err != nil {
		return fmt.Errorf("update stage run: %w", err)
	}
	return nil
}

func (r *RunRepo) ListStageRuns(ctx context.Context, runID string) ([]*StageRun, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_id, stage, status, evidence_json, started_at, ended_at, updated_at
FROM stage_runs
WHERE run_id = ?
ORDER BY updated_at ASC
`, runID)
	if err != nil {
		return nil, fmt.Errorf("list stage runs: %w", err)
	}
	defer rows.Close()
	out := make([]*StageRun, 0)
	for rows.Next() {
		var item StageRun
		var startedRaw, endedRaw, updatedRaw string
		if err := rows.Scan(
			&item.ID,
			&item.RunID,
			&item.Stage,
			&item.Status,
			&item.EvidenceJSON,
			&startedRaw,
			&endedRaw,
			&updatedRaw,
		); err != nil {
			return nil, fmt.Errorf("scan stage run: %w", err)
		}
		var parseErr error
		item.StartedAt, parseErr = parseTimestamp(startedRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		item.EndedAt, parseErr = parseOptionalTimestamp(endedRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		item.UpdatedAt, parseErr = parseTimestamp(updatedRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stage runs: %w", err)
	}
	return out, nil
}

func (r *RunRepo) GetStageRunByStage(ctx context.Context, runID string, stage string) (*StageRun, error) {
	stage = normalizeStage(stage)
	if stage == "" {
		return nil, nil
	}
	var item StageRun
	var startedRaw, endedRaw, updatedRaw string
	err := r.db.QueryRowContext(ctx, `
SELECT id, run_id, stage, status, evidence_json, started_at, ended_at, updated_at
FROM stage_runs
WHERE run_id = ? AND stage = ?
`, runID, stage).Scan(
		&item.ID,
		&item.RunID,
		&item.Stage,
		&item.Status,
		&item.EvidenceJSON,
		&startedRaw,
		&endedRaw,
		&updatedRaw,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get stage run: %w", err)
	}
	var parseErr error
	item.StartedAt, parseErr = parseTimestamp(startedRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	item.EndedAt, parseErr = parseOptionalTimestamp(endedRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	item.UpdatedAt, parseErr = parseTimestamp(updatedRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	return &item, nil
}

func normalizeStage(stage string) string {
	stage = strings.ToLower(strings.TrimSpace(stage))
	switch stage {
	case "plan", "build", "test":
		return stage
	default:
		return ""
	}
}
