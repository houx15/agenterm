package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var reviewCycleStatusTransitions = map[string]map[string]bool{
	"review_pending": {
		"review_pending":           true,
		"review_running":           true,
		"review_changes_requested": true,
		"review_passed":            true,
	},
	"review_running": {
		"review_running":           true,
		"review_changes_requested": true,
		"review_passed":            true,
	},
	"review_changes_requested": {
		"review_changes_requested": true,
		"review_running":           true,
		"review_passed":            true,
	},
	"review_passed": {
		"review_passed":            true,
		"review_running":           true,
		"review_changes_requested": true,
	},
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
	nextStatus, err := normalizeReviewCycleStatus(status)
	if err != nil {
		return err
	}

	current, err := r.GetCycle(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("review cycle not found")
	}
	currentStatus, err := normalizeReviewCycleStatus(current.Status)
	if err != nil {
		return err
	}
	allowed := reviewCycleStatusTransitions[currentStatus][nextStatus]
	if !allowed {
		return fmt.Errorf("invalid review cycle transition: %s -> %s", currentStatus, nextStatus)
	}
	if nextStatus == "review_passed" {
		openIssues, err := r.CountOpenIssuesByCycle(ctx, id)
		if err != nil {
			return err
		}
		if openIssues > 0 {
			return fmt.Errorf("cannot set review_passed while review issues are open")
		}
	}

	res, err := r.db.ExecContext(ctx, `
UPDATE review_cycles
SET status = ?, updated_at = ?
WHERE id = ?
`, nextStatus, formatTimestamp(nowUTC()), id)
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

func normalizeReviewCycleStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "review_pending", "review_running", "review_changes_requested", "review_passed":
		return status, nil
	default:
		return "", fmt.Errorf("invalid review cycle status")
	}
}

func (r *ReviewRepo) CountOpenIssuesByCycle(ctx context.Context, cycleID string) (int, error) {
	cycleID = strings.TrimSpace(cycleID)
	if cycleID == "" {
		return 0, fmt.Errorf("cycle id is required")
	}
	var total int
	err := r.db.QueryRowContext(ctx, `
SELECT count(1)
FROM review_issues
WHERE cycle_id = ?
  AND lower(trim(status)) NOT IN ('resolved', 'closed', 'accepted')
`, cycleID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count open review issues by cycle: %w", err)
	}
	return total, nil
}

func (r *ReviewRepo) SetCycleStatusByTaskOpenIssues(ctx context.Context, taskID string) error {
	_, _, err := r.SyncLatestCycleStatusByTaskOpenIssues(ctx, taskID)
	return err
}

func (r *ReviewRepo) SyncLatestCycleStatusByTaskOpenIssues(ctx context.Context, taskID string) (bool, *ReviewCycle, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, nil, fmt.Errorf("task id is required")
	}
	cycles, err := r.ListCyclesByTask(ctx, taskID)
	if err != nil {
		return false, nil, err
	}
	if len(cycles) == 0 {
		return false, nil, nil
	}
	latest := cycles[len(cycles)-1]

	open, err := r.CountOpenIssuesByTask(ctx, taskID)
	if err != nil {
		return false, nil, err
	}
	nextStatus := "review_passed"
	if open > 0 {
		nextStatus = "review_changes_requested"
	}
	current, err := normalizeReviewCycleStatus(latest.Status)
	if err != nil {
		return false, nil, err
	}
	if current == nextStatus {
		return false, latest, nil
	}
	if err := r.UpdateCycleStatus(ctx, latest.ID, nextStatus); err != nil {
		return false, nil, fmt.Errorf("set latest cycle status by issues: %w", err)
	}
	latest.Status = nextStatus
	return true, latest, nil
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
