package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type ProjectOrchestratorRepo struct {
	db *sql.DB
}

func NewProjectOrchestratorRepo(db *sql.DB) *ProjectOrchestratorRepo {
	return &ProjectOrchestratorRepo{db: db}
}

func (r *ProjectOrchestratorRepo) EnsureDefaultForProject(ctx context.Context, projectID string) error {
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	now := formatTimestamp(nowUTC())
	_, err := r.db.ExecContext(ctx, `
INSERT OR IGNORE INTO project_orchestrators (
	project_id, workflow_id, default_provider, default_model, max_parallel, review_policy, notify_on_blocked, created_at, updated_at
) VALUES (?, 'workflow-balanced', 'anthropic', 'claude-sonnet-4-5', 4, 'strict', 1, ?, ?)
`, projectID, now, now)
	if err != nil {
		return fmt.Errorf("ensure project orchestrator: %w", err)
	}
	return nil
}

func (r *ProjectOrchestratorRepo) Get(ctx context.Context, projectID string) (*ProjectOrchestrator, error) {
	var item ProjectOrchestrator
	var createdAtRaw, updatedAtRaw string
	var notifyInt int
	err := r.db.QueryRowContext(ctx, `
SELECT project_id, workflow_id, default_provider, default_model, max_parallel, review_policy, notify_on_blocked, created_at, updated_at
FROM project_orchestrators
WHERE project_id = ?
`, projectID).Scan(
		&item.ProjectID,
		&item.WorkflowID,
		&item.DefaultProvider,
		&item.DefaultModel,
		&item.MaxParallel,
		&item.ReviewPolicy,
		&notifyInt,
		&createdAtRaw,
		&updatedAtRaw,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get project orchestrator: %w", err)
	}
	item.NotifyOnBlocked = notifyInt != 0
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

func (r *ProjectOrchestratorRepo) Update(ctx context.Context, item *ProjectOrchestrator) error {
	if item == nil || item.ProjectID == "" {
		return fmt.Errorf("project orchestrator is required")
	}
	if item.WorkflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}
	item.UpdatedAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE project_orchestrators
SET workflow_id = ?, default_provider = ?, default_model = ?, max_parallel = ?, review_policy = ?, notify_on_blocked = ?, updated_at = ?
WHERE project_id = ?
`, item.WorkflowID, item.DefaultProvider, item.DefaultModel, item.MaxParallel, item.ReviewPolicy, boolToInt(item.NotifyOnBlocked), formatTimestamp(item.UpdatedAt), item.ProjectID)
	if err != nil {
		return fmt.Errorf("update project orchestrator: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project orchestrator rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("project orchestrator not found")
	}
	return nil
}

type WorkflowRepo struct {
	db *sql.DB
}

func NewWorkflowRepo(db *sql.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db}
}

func (r *WorkflowRepo) Create(ctx context.Context, w *Workflow) error {
	if w == nil {
		return fmt.Errorf("workflow is required")
	}
	if w.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		w.ID = id
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = nowUTC()
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = w.CreatedAt
	}
	if w.Version <= 0 {
		w.Version = 1
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO workflows (id, name, description, scope, is_builtin, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, w.ID, w.Name, w.Description, w.Scope, boolToInt(w.IsBuiltin), w.Version, formatTimestamp(w.CreatedAt), formatTimestamp(w.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create workflow: %w", err)
	}
	for _, phase := range w.Phases {
		if phase == nil {
			continue
		}
		phase.WorkflowID = w.ID
		if err := r.CreatePhase(ctx, phase); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkflowRepo) CreatePhase(ctx context.Context, p *WorkflowPhase) error {
	if p == nil {
		return fmt.Errorf("workflow phase is required")
	}
	if p.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		p.ID = id
	}
	if p.WorkflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = nowUTC()
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}
	if p.MaxParallel <= 0 {
		p.MaxParallel = 1
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO workflow_phases (id, workflow_id, ordinal, phase_type, role, entry_rule, exit_rule, max_parallel, agent_selector, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, p.ID, p.WorkflowID, p.Ordinal, p.PhaseType, p.Role, p.EntryRule, p.ExitRule, p.MaxParallel, p.AgentSelector, formatTimestamp(p.CreatedAt), formatTimestamp(p.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create workflow phase: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) Get(ctx context.Context, id string) (*Workflow, error) {
	var w Workflow
	var createdAtRaw, updatedAtRaw string
	var builtinInt int
	err := r.db.QueryRowContext(ctx, `
SELECT id, name, description, scope, is_builtin, version, created_at, updated_at
FROM workflows
WHERE id = ?
`, id).Scan(&w.ID, &w.Name, &w.Description, &w.Scope, &builtinInt, &w.Version, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	w.IsBuiltin = builtinInt != 0
	w.CreatedAt, err = parseTimestamp(createdAtRaw)
	if err != nil {
		return nil, err
	}
	w.UpdatedAt, err = parseTimestamp(updatedAtRaw)
	if err != nil {
		return nil, err
	}
	w.Phases, err = r.ListPhases(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WorkflowRepo) List(ctx context.Context) ([]*Workflow, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, description, scope, is_builtin, version, created_at, updated_at
FROM workflows
ORDER BY is_builtin DESC, name ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()
	items := make([]*Workflow, 0)
	for rows.Next() {
		var w Workflow
		var createdAtRaw, updatedAtRaw string
		var builtinInt int
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Scope, &builtinInt, &w.Version, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		w.IsBuiltin = builtinInt != 0
		w.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		w.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflows: %w", err)
	}
	for _, w := range items {
		phases, err := r.ListPhases(ctx, w.ID)
		if err != nil {
			return nil, err
		}
		w.Phases = phases
	}
	return items, nil
}

func (r *WorkflowRepo) ListPhases(ctx context.Context, workflowID string) ([]*WorkflowPhase, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, workflow_id, ordinal, phase_type, role, entry_rule, exit_rule, max_parallel, agent_selector, created_at, updated_at
FROM workflow_phases
WHERE workflow_id = ?
ORDER BY ordinal ASC
`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("list workflow phases: %w", err)
	}
	defer rows.Close()
	items := make([]*WorkflowPhase, 0)
	for rows.Next() {
		var p WorkflowPhase
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&p.ID, &p.WorkflowID, &p.Ordinal, &p.PhaseType, &p.Role, &p.EntryRule, &p.ExitRule, &p.MaxParallel, &p.AgentSelector, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan workflow phase: %w", err)
		}
		p.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		p.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow phases: %w", err)
	}
	return items, nil
}

func (r *WorkflowRepo) Update(ctx context.Context, w *Workflow) error {
	if w == nil || w.ID == "" {
		return fmt.Errorf("workflow is required")
	}
	w.UpdatedAt = nowUTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow update tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
UPDATE workflows
SET name = ?, description = ?, scope = ?, version = ?, updated_at = ?
WHERE id = ?
`, w.Name, w.Description, w.Scope, w.Version, formatTimestamp(w.UpdatedAt), w.ID)
	if err != nil {
		return fmt.Errorf("update workflow: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("workflow not found")
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM workflow_phases WHERE workflow_id = ?`, w.ID)
	if err != nil {
		return fmt.Errorf("replace workflow phases: %w", err)
	}
	for _, phase := range w.Phases {
		if phase == nil {
			continue
		}
		phase.WorkflowID = w.ID
		if phase.ID == "" {
			id, err := NewID()
			if err != nil {
				return err
			}
			phase.ID = id
		}
		if phase.CreatedAt.IsZero() {
			phase.CreatedAt = nowUTC()
		}
		if phase.UpdatedAt.IsZero() {
			phase.UpdatedAt = phase.CreatedAt
		}
		if phase.MaxParallel <= 0 {
			phase.MaxParallel = 1
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_phases (id, workflow_id, ordinal, phase_type, role, entry_rule, exit_rule, max_parallel, agent_selector, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, phase.ID, phase.WorkflowID, phase.Ordinal, phase.PhaseType, phase.Role, phase.EntryRule, phase.ExitRule, phase.MaxParallel, phase.AgentSelector, formatTimestamp(phase.CreatedAt), formatTimestamp(phase.UpdatedAt)); err != nil {
			return fmt.Errorf("create workflow phase: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow update tx: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM workflows WHERE id = ? AND is_builtin = 0`, id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("workflow not found or is builtin")
	}
	return nil
}

type RoleBindingRepo struct {
	db *sql.DB
}

func NewRoleBindingRepo(db *sql.DB) *RoleBindingRepo {
	return &RoleBindingRepo{db: db}
}

func (r *RoleBindingRepo) ListByProject(ctx context.Context, projectID string) ([]*RoleBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, project_id, role, provider, model, max_parallel, created_at, updated_at
FROM role_bindings
WHERE project_id = ?
ORDER BY role ASC
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list role bindings: %w", err)
	}
	defer rows.Close()
	items := make([]*RoleBinding, 0)
	for rows.Next() {
		var item RoleBinding
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Role, &item.Provider, &item.Model, &item.MaxParallel, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan role binding: %w", err)
		}
		item.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role bindings: %w", err)
	}
	return items, nil
}

func (r *RoleBindingRepo) ReplaceForProject(ctx context.Context, projectID string, bindings []*RoleBinding) error {
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("project id is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin role binding tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_bindings WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("clear role bindings: %w", err)
	}
	for _, b := range bindings {
		if b == nil {
			continue
		}
		if strings.TrimSpace(b.Role) == "" || strings.TrimSpace(b.Provider) == "" || strings.TrimSpace(b.Model) == "" {
			return fmt.Errorf("role, provider, and model are required")
		}
		if b.ID == "" {
			id, err := NewID()
			if err != nil {
				return err
			}
			b.ID = id
		}
		if b.MaxParallel <= 0 {
			b.MaxParallel = 1
		}
		if b.CreatedAt.IsZero() {
			b.CreatedAt = nowUTC()
		}
		b.UpdatedAt = nowUTC()
		if _, err := tx.ExecContext(ctx, `
INSERT INTO role_bindings (id, project_id, role, provider, model, max_parallel, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, b.ID, projectID, b.Role, b.Provider, b.Model, b.MaxParallel, formatTimestamp(b.CreatedAt), formatTimestamp(b.UpdatedAt)); err != nil {
			return fmt.Errorf("insert role binding: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit role binding tx: %w", err)
	}
	return nil
}

type ProjectKnowledgeRepo struct {
	db *sql.DB
}

func NewProjectKnowledgeRepo(db *sql.DB) *ProjectKnowledgeRepo {
	return &ProjectKnowledgeRepo{db: db}
}

func (r *ProjectKnowledgeRepo) Create(ctx context.Context, entry *ProjectKnowledgeEntry) error {
	if entry == nil {
		return fmt.Errorf("knowledge entry is required")
	}
	if entry.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		entry.ID = id
	}
	if entry.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if entry.Kind == "" {
		entry.Kind = "note"
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = nowUTC()
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = entry.CreatedAt
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO project_knowledge_entries (id, project_id, kind, title, content, source_uri, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, entry.ID, entry.ProjectID, entry.Kind, entry.Title, entry.Content, entry.SourceURI, formatTimestamp(entry.CreatedAt), formatTimestamp(entry.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create project knowledge entry: %w", err)
	}
	return nil
}

func (r *ProjectKnowledgeRepo) ListByProject(ctx context.Context, projectID string) ([]*ProjectKnowledgeEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, project_id, kind, title, content, source_uri, created_at, updated_at
FROM project_knowledge_entries
WHERE project_id = ?
ORDER BY created_at DESC
`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project knowledge entries: %w", err)
	}
	defer rows.Close()
	items := make([]*ProjectKnowledgeEntry, 0)
	for rows.Next() {
		var item ProjectKnowledgeEntry
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Kind, &item.Title, &item.Content, &item.SourceURI, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan project knowledge entry: %w", err)
		}
		item.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project knowledge entries: %w", err)
	}
	return items, nil
}

type ReviewRepo struct {
	db *sql.DB
}

func NewReviewRepo(db *sql.DB) *ReviewRepo {
	return &ReviewRepo{db: db}
}

func (r *ReviewRepo) CreateCycle(ctx context.Context, cycle *ReviewCycle) error {
	if cycle == nil {
		return fmt.Errorf("review cycle is required")
	}
	if strings.TrimSpace(cycle.TaskID) == "" {
		return fmt.Errorf("task_id is required")
	}
	if cycle.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		cycle.ID = id
	}
	if cycle.Iteration <= 0 {
		next, err := r.nextIteration(ctx, cycle.TaskID)
		if err != nil {
			return err
		}
		cycle.Iteration = next
	}
	if strings.TrimSpace(cycle.Status) == "" {
		cycle.Status = "review_pending"
	}
	if cycle.CreatedAt.IsZero() {
		cycle.CreatedAt = nowUTC()
	}
	if cycle.UpdatedAt.IsZero() {
		cycle.UpdatedAt = cycle.CreatedAt
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO review_cycles (id, task_id, iteration, status, commit_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, cycle.ID, cycle.TaskID, cycle.Iteration, cycle.Status, cycle.CommitHash, formatTimestamp(cycle.CreatedAt), formatTimestamp(cycle.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create review cycle: %w", err)
	}
	return nil
}

func (r *ReviewRepo) GetCycle(ctx context.Context, id string) (*ReviewCycle, error) {
	var item ReviewCycle
	var createdAtRaw, updatedAtRaw string
	err := r.db.QueryRowContext(ctx, `
SELECT id, task_id, iteration, status, commit_hash, created_at, updated_at
FROM review_cycles
WHERE id = ?
`, id).Scan(&item.ID, &item.TaskID, &item.Iteration, &item.Status, &item.CommitHash, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get review cycle: %w", err)
	}
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

func (r *ReviewRepo) UpdateCycleStatus(ctx context.Context, id string, status string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		return fmt.Errorf("status is required")
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE review_cycles
SET status = ?, updated_at = ?
WHERE id = ?
`, status, formatTimestamp(nowUTC()), id)
	if err != nil {
		return fmt.Errorf("update review cycle status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("review cycle not found")
	}
	return nil
}

func (r *ReviewRepo) ListCyclesByTask(ctx context.Context, taskID string) ([]*ReviewCycle, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, task_id, iteration, status, commit_hash, created_at, updated_at
FROM review_cycles
WHERE task_id = ?
ORDER BY iteration ASC
`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list review cycles: %w", err)
	}
	defer rows.Close()
	items := make([]*ReviewCycle, 0)
	for rows.Next() {
		var item ReviewCycle
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Iteration, &item.Status, &item.CommitHash, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan review cycle: %w", err)
		}
		item.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review cycles: %w", err)
	}
	return items, nil
}

func (r *ReviewRepo) CreateIssue(ctx context.Context, issue *ReviewIssue) error {
	if issue == nil {
		return fmt.Errorf("review issue is required")
	}
	if strings.TrimSpace(issue.CycleID) == "" {
		return fmt.Errorf("cycle_id is required")
	}
	if strings.TrimSpace(issue.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if issue.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		issue.ID = id
	}
	if strings.TrimSpace(issue.Severity) == "" {
		issue.Severity = "medium"
	}
	if strings.TrimSpace(issue.Status) == "" {
		issue.Status = "open"
	}
	if issue.CreatedAt.IsZero() {
		issue.CreatedAt = nowUTC()
	}
	if issue.UpdatedAt.IsZero() {
		issue.UpdatedAt = issue.CreatedAt
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO review_issues (id, cycle_id, severity, summary, status, resolution, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, issue.ID, issue.CycleID, issue.Severity, issue.Summary, issue.Status, issue.Resolution, formatTimestamp(issue.CreatedAt), formatTimestamp(issue.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create review issue: %w", err)
	}
	return nil
}

func (r *ReviewRepo) GetIssue(ctx context.Context, id string) (*ReviewIssue, error) {
	var item ReviewIssue
	var createdAtRaw, updatedAtRaw string
	err := r.db.QueryRowContext(ctx, `
SELECT id, cycle_id, severity, summary, status, resolution, created_at, updated_at
FROM review_issues
WHERE id = ?
`, id).Scan(&item.ID, &item.CycleID, &item.Severity, &item.Summary, &item.Status, &item.Resolution, &createdAtRaw, &updatedAtRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get review issue: %w", err)
	}
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

func (r *ReviewRepo) ListIssuesByCycle(ctx context.Context, cycleID string) ([]*ReviewIssue, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, cycle_id, severity, summary, status, resolution, created_at, updated_at
FROM review_issues
WHERE cycle_id = ?
ORDER BY created_at ASC
`, cycleID)
	if err != nil {
		return nil, fmt.Errorf("list review issues: %w", err)
	}
	defer rows.Close()
	items := make([]*ReviewIssue, 0)
	for rows.Next() {
		var item ReviewIssue
		var createdAtRaw, updatedAtRaw string
		if err := rows.Scan(&item.ID, &item.CycleID, &item.Severity, &item.Summary, &item.Status, &item.Resolution, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan review issue: %w", err)
		}
		item.CreatedAt, err = parseTimestamp(createdAtRaw)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = parseTimestamp(updatedAtRaw)
		if err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review issues: %w", err)
	}
	return items, nil
}

func (r *ReviewRepo) UpdateIssue(ctx context.Context, issue *ReviewIssue) error {
	if issue == nil || strings.TrimSpace(issue.ID) == "" {
		return fmt.Errorf("review issue is required")
	}
	issue.UpdatedAt = nowUTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE review_issues
SET severity = ?, summary = ?, status = ?, resolution = ?, updated_at = ?
WHERE id = ?
`, issue.Severity, issue.Summary, issue.Status, issue.Resolution, formatTimestamp(issue.UpdatedAt), issue.ID)
	if err != nil {
		return fmt.Errorf("update review issue: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("review issue not found")
	}
	return nil
}

func (r *ReviewRepo) CountOpenIssuesByTask(ctx context.Context, taskID string) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
SELECT count(1)
FROM review_issues ri
JOIN review_cycles rc ON rc.id = ri.cycle_id
WHERE rc.task_id = ?
  AND lower(trim(ri.status)) NOT IN ('resolved', 'closed', 'accepted')
`, taskID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count open review issues: %w", err)
	}
	return total, nil
}

func (r *ReviewRepo) nextIteration(ctx context.Context, taskID string) (int, error) {
	var max sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT max(iteration) FROM review_cycles WHERE task_id = ?`, taskID).Scan(&max); err != nil {
		return 0, fmt.Errorf("query max review iteration: %w", err)
	}
	if !max.Valid {
		return 1, nil
	}
	return int(max.Int64) + 1, nil
}
