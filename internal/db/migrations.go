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
	{
		version: 3,
		name:    "create orchestrator governance tables",
		sql: `
CREATE TABLE IF NOT EXISTS workflows (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	scope TEXT NOT NULL DEFAULT 'global',
	is_builtin INTEGER NOT NULL DEFAULT 0,
	version INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_phases (
	id TEXT PRIMARY KEY,
	workflow_id TEXT NOT NULL,
	ordinal INTEGER NOT NULL,
	phase_type TEXT NOT NULL,
	role TEXT NOT NULL,
	entry_rule TEXT NOT NULL DEFAULT '',
	exit_rule TEXT NOT NULL DEFAULT '',
	max_parallel INTEGER NOT NULL DEFAULT 1,
	agent_selector TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS project_orchestrators (
	project_id TEXT PRIMARY KEY,
	workflow_id TEXT NOT NULL,
	default_provider TEXT NOT NULL DEFAULT 'anthropic',
	default_model TEXT NOT NULL DEFAULT 'claude-sonnet-4-5',
	max_parallel INTEGER NOT NULL DEFAULT 4,
	review_policy TEXT NOT NULL DEFAULT 'strict',
	notify_on_blocked INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
	FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS role_bindings (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	role TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	max_parallel INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_role_bindings_project_role
ON role_bindings(project_id, role);

CREATE TABLE IF NOT EXISTS project_knowledge_entries (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	source_uri TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS review_cycles (
	id TEXT PRIMARY KEY,
	task_id TEXT NOT NULL,
	iteration INTEGER NOT NULL,
	status TEXT NOT NULL,
	commit_hash TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_review_cycles_task_iteration
ON review_cycles(task_id, iteration);

CREATE TABLE IF NOT EXISTS review_issues (
	id TEXT PRIMARY KEY,
	cycle_id TEXT NOT NULL,
	severity TEXT NOT NULL,
	summary TEXT NOT NULL,
	status TEXT NOT NULL,
	resolution TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(cycle_id) REFERENCES review_cycles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_phases_workflow_id ON workflow_phases(workflow_id);
CREATE INDEX IF NOT EXISTS idx_project_knowledge_entries_project_id ON project_knowledge_entries(project_id);
CREATE INDEX IF NOT EXISTS idx_review_issues_cycle_id ON review_issues(cycle_id);

INSERT OR IGNORE INTO workflows (id, name, description, scope, is_builtin, version, created_at, updated_at)
VALUES
	('workflow-fast', 'Fast', 'Minimal process with speed-first execution', 'global', 1, 1, '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('workflow-balanced', 'Balanced', 'Balanced quality and throughput with mandatory review', 'global', 1, 1, '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('workflow-hardening', 'Hardening', 'Security and reliability focused workflow', 'global', 1, 1, '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z');

INSERT OR IGNORE INTO workflow_phases (id, workflow_id, ordinal, phase_type, role, entry_rule, exit_rule, max_parallel, agent_selector, created_at, updated_at)
VALUES
	('wf-fast-01-scan', 'workflow-fast', 1, 'scan', 'planner', '', 'repository scanned', 1, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-fast-02-implement', 'workflow-fast', 2, 'implementation', 'coder', 'scan complete', 'changes committed', 4, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-fast-03-review', 'workflow-fast', 3, 'review', 'reviewer', 'changes committed', 'issues triaged', 2, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-balanced-01-scan', 'workflow-balanced', 1, 'scan', 'planner', '', 'scan summary persisted', 1, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-balanced-02-plan', 'workflow-balanced', 2, 'planning', 'planner', 'scan done', 'task DAG created', 1, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-balanced-03-implement', 'workflow-balanced', 3, 'implementation', 'coder', 'task ready', 'ready_for_review commit', 4, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-balanced-04-review', 'workflow-balanced', 4, 'review', 'reviewer', 'ready_for_review commit', 'no open review issues', 2, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-hardening-01-scan', 'workflow-hardening', 1, 'scan', 'planner', '', 'critical risk list created', 1, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-hardening-02-plan', 'workflow-hardening', 2, 'planning', 'planner', 'scan done', 'risk-aware task DAG created', 1, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-hardening-03-implement', 'workflow-hardening', 3, 'implementation', 'coder', 'task ready', 'commit tagged ready_for_review', 3, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z'),
	('wf-hardening-04-review', 'workflow-hardening', 4, 'review', 'reviewer', 'ready_for_review commit', 'all high+ resolved', 2, '', '2026-02-17T00:00:00Z', '2026-02-17T00:00:00Z');
`,
	},
	{
		version: 4,
		name:    "create demand pool items",
		sql: `
CREATE TABLE IF NOT EXISTS demand_pool_items (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'captured',
	priority INTEGER NOT NULL DEFAULT 0,
	impact INTEGER NOT NULL DEFAULT 0,
	effort INTEGER NOT NULL DEFAULT 0,
	risk INTEGER NOT NULL DEFAULT 0,
	urgency INTEGER NOT NULL DEFAULT 0,
	tags TEXT NOT NULL DEFAULT '[]',
	source TEXT NOT NULL DEFAULT 'user',
	created_by TEXT NOT NULL DEFAULT '',
	selected_task_id TEXT,
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
	FOREIGN KEY(selected_task_id) REFERENCES tasks(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_demand_pool_project_id ON demand_pool_items(project_id);
CREATE INDEX IF NOT EXISTS idx_demand_pool_status ON demand_pool_items(status);
CREATE INDEX IF NOT EXISTS idx_demand_pool_project_priority ON demand_pool_items(project_id, priority DESC, created_at DESC);
`,
	},
	{
		version: 5,
		name:    "add structured orchestrator history payload",
		sql: `
ALTER TABLE orchestrator_messages ADD COLUMN message_json TEXT NOT NULL DEFAULT '';
`,
	},
	{
		version: 6,
		name:    "create role loop attempts",
		sql: `
CREATE TABLE IF NOT EXISTS role_loop_attempts (
	task_id TEXT NOT NULL,
	role_name TEXT NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (task_id, role_name),
	FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_role_loop_attempts_task_id ON role_loop_attempts(task_id);
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
