package db

import (
	"context"
	"database/sql"
	"fmt"
)

type PlanningSessionRepo struct {
	db *sql.DB
}

func NewPlanningSessionRepo(db *sql.DB) *PlanningSessionRepo {
	return &PlanningSessionRepo{db: db}
}

func (r *PlanningSessionRepo) Create(ctx context.Context, ps *PlanningSession) error {
	if ps.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		ps.ID = id
	}
	if ps.CreatedAt.IsZero() {
		ps.CreatedAt = nowUTC()
	}
	if ps.UpdatedAt.IsZero() {
		ps.UpdatedAt = ps.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO planning_sessions (id, requirement_id, agent_session_id, status, blueprint, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, ps.ID, ps.RequirementID, nullIfEmpty(ps.AgentSessionID), ps.Status, ps.Blueprint, formatTimestamp(ps.CreatedAt), formatTimestamp(ps.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create planning session: %w", err)
	}
	return nil
}

func (r *PlanningSessionRepo) Get(ctx context.Context, id string) (*PlanningSession, error) {
	var ps PlanningSession
	var createdAtRaw, updatedAtRaw string
	var agentSessionID sql.NullString

	err := r.db.QueryRowContext(ctx, `
SELECT id, requirement_id, agent_session_id, status, blueprint, created_at, updated_at
FROM planning_sessions
WHERE id = ?
`, id).Scan(&ps.ID, &ps.RequirementID, &agentSessionID, &ps.Status, &ps.Blueprint, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get planning session %q: %w", id, err)
	}

	if agentSessionID.Valid {
		ps.AgentSessionID = agentSessionID.String
	}

	ps.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	ps.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	return &ps, nil
}

func (r *PlanningSessionRepo) Update(ctx context.Context, ps *PlanningSession) error {
	ps.UpdatedAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE planning_sessions
SET requirement_id = ?, agent_session_id = ?, status = ?, blueprint = ?, updated_at = ?
WHERE id = ?
`, ps.RequirementID, nullIfEmpty(ps.AgentSessionID), ps.Status, ps.Blueprint, formatTimestamp(ps.UpdatedAt), ps.ID)
	if err != nil {
		return fmt.Errorf("failed to update planning session %q: %w", ps.ID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for planning session %q: %w", ps.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("planning session %q not found", ps.ID)
	}
	return nil
}

func (r *PlanningSessionRepo) GetByRequirement(ctx context.Context, requirementID string) (*PlanningSession, error) {
	var ps PlanningSession
	var createdAtRaw, updatedAtRaw string
	var agentSessionID sql.NullString

	err := r.db.QueryRowContext(ctx, `
SELECT id, requirement_id, agent_session_id, status, blueprint, created_at, updated_at
FROM planning_sessions
WHERE requirement_id = ?
ORDER BY created_at DESC
LIMIT 1
`, requirementID).Scan(&ps.ID, &ps.RequirementID, &agentSessionID, &ps.Status, &ps.Blueprint, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get planning session by requirement %q: %w", requirementID, err)
	}

	if agentSessionID.Valid {
		ps.AgentSessionID = agentSessionID.String
	}

	ps.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	ps.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	return &ps, nil
}
