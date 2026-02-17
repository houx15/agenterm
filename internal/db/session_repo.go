package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, session *Session) error {
	if session.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		session.ID = id
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = nowUTC()
	}
	if session.LastActivityAt.IsZero() {
		session.LastActivityAt = session.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO sessions (id, task_id, tmux_session_name, tmux_window_id, agent_type, role, status, human_attached, created_at, last_activity_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, session.ID, nullIfEmpty(session.TaskID), session.TmuxSessionName, session.TmuxWindowID, session.AgentType, session.Role, session.Status, boolToInt(session.HumanAttached), formatTimestamp(session.CreatedAt), formatTimestamp(session.LastActivityAt))
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (r *SessionRepo) Get(ctx context.Context, id string) (*Session, error) {
	var s Session
	var taskID sql.NullString
	var humanAttachedInt int
	var createdAtRaw, lastActivityAtRaw string

	err := r.db.QueryRowContext(ctx, `
SELECT id, task_id, tmux_session_name, tmux_window_id, agent_type, role, status, human_attached, created_at, last_activity_at
FROM sessions
WHERE id = ?
`, id).Scan(&s.ID, &taskID, &s.TmuxSessionName, &s.TmuxWindowID, &s.AgentType, &s.Role, &s.Status, &humanAttachedInt, &createdAtRaw, &lastActivityAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session %q: %w", id, err)
	}

	s.TaskID = taskID.String
	s.HumanAttached = humanAttachedInt != 0
	s.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	s.LastActivityAt, err = parseTimestamp(lastActivityAtRaw)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *SessionRepo) List(ctx context.Context, filter SessionFilter) ([]*Session, error) {
	query := `SELECT id, task_id, tmux_session_name, tmux_window_id, agent_type, role, status, human_attached, created_at, last_activity_at FROM sessions`
	args := []any{}
	where := []string{}

	if filter.TaskID != "" {
		where = append(where, "task_id = ?")
		args = append(args, filter.TaskID)
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
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	sessions := []*Session{}
	for rows.Next() {
		var s Session
		var taskID sql.NullString
		var humanAttachedInt int
		var createdAtRaw, lastActivityAtRaw string
		if err := rows.Scan(&s.ID, &taskID, &s.TmuxSessionName, &s.TmuxWindowID, &s.AgentType, &s.Role, &s.Status, &humanAttachedInt, &createdAtRaw, &lastActivityAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		s.TaskID = taskID.String
		s.HumanAttached = humanAttachedInt != 0
		s.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		s.LastActivityAt, err = parseTimestamp(lastActivityAtRaw)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating sessions: %w", err)
	}

	return sessions, nil
}

func (r *SessionRepo) ListByTask(ctx context.Context, taskID string) ([]*Session, error) {
	return r.List(ctx, SessionFilter{TaskID: taskID})
}

func (r *SessionRepo) ListActive(ctx context.Context) ([]*Session, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, task_id, tmux_session_name, tmux_window_id, agent_type, role, status, human_attached, created_at, last_activity_at
FROM sessions
WHERE status != 'completed'
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}
	defer rows.Close()

	sessions := []*Session{}
	for rows.Next() {
		var s Session
		var taskID sql.NullString
		var humanAttachedInt int
		var createdAtRaw, lastActivityAtRaw string
		if err := rows.Scan(&s.ID, &taskID, &s.TmuxSessionName, &s.TmuxWindowID, &s.AgentType, &s.Role, &s.Status, &humanAttachedInt, &createdAtRaw, &lastActivityAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan active session: %w", err)
		}
		s.TaskID = taskID.String
		s.HumanAttached = humanAttachedInt != 0
		s.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		s.LastActivityAt, err = parseTimestamp(lastActivityAtRaw)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating active sessions: %w", err)
	}

	return sessions, nil
}

func (r *SessionRepo) Update(ctx context.Context, session *Session) error {
	session.LastActivityAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET task_id = ?, tmux_session_name = ?, tmux_window_id = ?, agent_type = ?, role = ?, status = ?, human_attached = ?, last_activity_at = ?
WHERE id = ?
`, nullIfEmpty(session.TaskID), session.TmuxSessionName, session.TmuxWindowID, session.AgentType, session.Role, session.Status, boolToInt(session.HumanAttached), formatTimestamp(session.LastActivityAt), session.ID)
	if err != nil {
		return fmt.Errorf("failed to update session %q: %w", session.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for session %q: %w", session.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("session %q not found", session.ID)
	}
	return nil
}

func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete session %q: %w", id, err)
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
