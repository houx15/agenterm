package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SessionCommandRepo struct {
	db *sql.DB
}

func NewSessionCommandRepo(db *sql.DB) *SessionCommandRepo {
	return &SessionCommandRepo{db: db}
}

func (r *SessionCommandRepo) Create(ctx context.Context, cmd *SessionCommand) error {
	if cmd == nil {
		return fmt.Errorf("session command is required")
	}
	if cmd.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		cmd.ID = id
	}
	if cmd.CreatedAt.IsZero() {
		cmd.CreatedAt = nowUTC()
	}
	if strings.TrimSpace(cmd.Status) == "" {
		cmd.Status = "queued"
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO session_commands (
	id, session_id, op, payload_json, status, result_json, error, created_at, sent_at, acked_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		cmd.ID,
		cmd.SessionID,
		cmd.Op,
		cmd.PayloadJSON,
		cmd.Status,
		cmd.ResultJSON,
		cmd.Error,
		formatTimestamp(cmd.CreatedAt),
		formatTimestampOrEmpty(cmd.SentAt),
		formatTimestampOrEmpty(cmd.AckedAt),
		formatTimestampOrEmpty(cmd.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create session command: %w", err)
	}
	return nil
}

func (r *SessionCommandRepo) Get(ctx context.Context, id string) (*SessionCommand, error) {
	var cmd SessionCommand
	var createdAtRaw, sentAtRaw, ackedAtRaw, completedAtRaw string
	err := r.db.QueryRowContext(ctx, `
SELECT id, session_id, op, payload_json, status, result_json, error, created_at, sent_at, acked_at, completed_at
FROM session_commands
WHERE id = ?
`, id).Scan(
		&cmd.ID,
		&cmd.SessionID,
		&cmd.Op,
		&cmd.PayloadJSON,
		&cmd.Status,
		&cmd.ResultJSON,
		&cmd.Error,
		&createdAtRaw,
		&sentAtRaw,
		&ackedAtRaw,
		&completedAtRaw,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session command %q: %w", id, err)
	}
	var parseErr error
	cmd.CreatedAt, parseErr = parseTimestamp(createdAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	cmd.SentAt, parseErr = parseOptionalTimestamp(sentAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	cmd.AckedAt, parseErr = parseOptionalTimestamp(ackedAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	cmd.CompletedAt, parseErr = parseOptionalTimestamp(completedAtRaw)
	if parseErr != nil {
		return nil, parseErr
	}
	return &cmd, nil
}

func (r *SessionCommandRepo) ListBySession(ctx context.Context, sessionID string, limit int) ([]*SessionCommand, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, session_id, op, payload_json, status, result_json, error, created_at, sent_at, acked_at, completed_at
FROM session_commands
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ?
`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list session commands: %w", err)
	}
	defer rows.Close()

	out := make([]*SessionCommand, 0, limit)
	for rows.Next() {
		var cmd SessionCommand
		var createdAtRaw, sentAtRaw, ackedAtRaw, completedAtRaw string
		if err := rows.Scan(
			&cmd.ID,
			&cmd.SessionID,
			&cmd.Op,
			&cmd.PayloadJSON,
			&cmd.Status,
			&cmd.ResultJSON,
			&cmd.Error,
			&createdAtRaw,
			&sentAtRaw,
			&ackedAtRaw,
			&completedAtRaw,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session command: %w", err)
		}
		var parseErr error
		cmd.CreatedAt, parseErr = parseTimestamp(createdAtRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		cmd.SentAt, parseErr = parseOptionalTimestamp(sentAtRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		cmd.AckedAt, parseErr = parseOptionalTimestamp(ackedAtRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		cmd.CompletedAt, parseErr = parseOptionalTimestamp(completedAtRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		out = append(out, &cmd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating session commands: %w", err)
	}
	return out, nil
}

func (r *SessionCommandRepo) Update(ctx context.Context, cmd *SessionCommand) error {
	if cmd == nil {
		return fmt.Errorf("session command is required")
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE session_commands
SET session_id = ?, op = ?, payload_json = ?, status = ?, result_json = ?, error = ?, sent_at = ?, acked_at = ?, completed_at = ?
WHERE id = ?
`,
		cmd.SessionID,
		cmd.Op,
		cmd.PayloadJSON,
		cmd.Status,
		cmd.ResultJSON,
		cmd.Error,
		formatTimestampOrEmpty(cmd.SentAt),
		formatTimestampOrEmpty(cmd.AckedAt),
		formatTimestampOrEmpty(cmd.CompletedAt),
		cmd.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session command %q: %w", cmd.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for session command %q: %w", cmd.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("session command %q not found", cmd.ID)
	}
	return nil
}

func formatTimestampOrEmpty(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return formatTimestamp(ts)
}

func parseOptionalTimestamp(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return parseTimestamp(raw)
}
