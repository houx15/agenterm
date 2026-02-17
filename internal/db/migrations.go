package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "create core tables",
		sql: `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	repo_path TEXT NOT NULL,
	status TEXT NOT NULL,
	playbook TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	status TEXT NOT NULL,
	depends_on TEXT NOT NULL DEFAULT '[]',
	worktree_id TEXT NOT NULL DEFAULT '',
	spec_path TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS worktrees (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	branch_name TEXT NOT NULL,
	path TEXT NOT NULL,
	task_id TEXT,
	status TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
	FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	task_id TEXT,
	tmux_session_name TEXT NOT NULL,
	tmux_window_id TEXT NOT NULL DEFAULT '',
	agent_type TEXT NOT NULL,
	role TEXT NOT NULL,
	status TEXT NOT NULL,
	human_attached INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	last_activity_at TEXT NOT NULL,
	FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS agent_configs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	command TEXT NOT NULL,
	resume_command TEXT NOT NULL DEFAULT '',
	headless_command TEXT NOT NULL DEFAULT '',
	capabilities TEXT NOT NULL DEFAULT '[]',
	languages TEXT NOT NULL DEFAULT '[]',
	cost_tier TEXT NOT NULL,
	speed_tier TEXT NOT NULL,
	supports_session_resume INTEGER NOT NULL DEFAULT 0,
	supports_headless INTEGER NOT NULL DEFAULT 0,
	auto_accept_mode TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_worktrees_project_id ON worktrees(project_id);
CREATE INDEX IF NOT EXISTS idx_worktrees_status ON worktrees(status);
CREATE INDEX IF NOT EXISTS idx_sessions_task_id ON sessions(task_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
`,
	},
	{
		version: 2,
		name:    "create orchestrator history",
		sql: `
CREATE TABLE IF NOT EXISTS orchestrator_messages (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_orchestrator_messages_project_created
ON orchestrator_messages(project_id, created_at);
`,
	},
}

func RunMigrations(ctx context.Context, conn *sql.DB) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS _meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`); err != nil {
		return fmt.Errorf("failed to ensure _meta table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO _meta (key, value) VALUES ('schema_version', '0')`); err != nil {
		return fmt.Errorf("failed to initialize schema version: %w", err)
	}

	var currentRaw string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM _meta WHERE key = 'schema_version'`).Scan(&currentRaw); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}

	currentVersion, err := strconv.Atoi(currentRaw)
	if err != nil {
		return fmt.Errorf("invalid schema version %q: %w", currentRaw, err)
	}

	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("failed migration %03d (%s): %w", m.version, m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE _meta SET value = ? WHERE key = 'schema_version'`, strconv.Itoa(m.version)); err != nil {
			return fmt.Errorf("failed to set schema version %03d: %w", m.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}

	return nil
}
