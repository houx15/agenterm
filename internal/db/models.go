package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RepoPath  string    `json:"repo_path"`
	Status    string    `json:"status"`
	Playbook  string    `json:"playbook,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Task struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	DependsOn   []string  `json:"depends_on"`
	WorktreeID  string    `json:"worktree_id,omitempty"`
	SpecPath    string    `json:"spec_path,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Worktree struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	BranchName string `json:"branch_name"`
	Path       string `json:"path"`
	TaskID     string `json:"task_id,omitempty"`
	Status     string `json:"status"`
}

type Session struct {
	ID              string    `json:"id"`
	TaskID          string    `json:"task_id,omitempty"`
	TmuxSessionName string    `json:"tmux_session_name"`
	TmuxWindowID    string    `json:"tmux_window_id,omitempty"`
	AgentType       string    `json:"agent_type"`
	Role            string    `json:"role"`
	Status          string    `json:"status"`
	HumanAttached   bool      `json:"human_attached"`
	CreatedAt       time.Time `json:"created_at"`
	LastActivityAt  time.Time `json:"last_activity_at"`
}

type DemandPoolItem struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	Priority       int       `json:"priority"`
	Impact         int       `json:"impact"`
	Effort         int       `json:"effort"`
	Risk           int       `json:"risk"`
	Urgency        int       `json:"urgency"`
	Tags           []string  `json:"tags"`
	Source         string    `json:"source"`
	CreatedBy      string    `json:"created_by,omitempty"`
	SelectedTaskID string    `json:"selected_task_id,omitempty"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AgentConfig struct {
	ID                    string   `json:"id" yaml:"id"`
	Name                  string   `json:"name" yaml:"name"`
	Command               string   `json:"command" yaml:"command"`
	ResumeCommand         string   `json:"resume_command,omitempty" yaml:"resume_command,omitempty"`
	HeadlessCommand       string   `json:"headless_command,omitempty" yaml:"headless_command,omitempty"`
	Capabilities          []string `json:"capabilities" yaml:"capabilities"`
	Languages             []string `json:"languages" yaml:"languages"`
	CostTier              string   `json:"cost_tier" yaml:"cost_tier"`
	SpeedTier             string   `json:"speed_tier" yaml:"speed_tier"`
	SupportsSessionResume bool     `json:"supports_session_resume" yaml:"supports_session_resume"`
	SupportsHeadless      bool     `json:"supports_headless" yaml:"supports_headless"`
	AutoAcceptMode        string   `json:"auto_accept_mode,omitempty" yaml:"auto_accept_mode,omitempty"`
}

type ProjectOrchestrator struct {
	ProjectID       string    `json:"project_id"`
	WorkflowID      string    `json:"workflow_id"`
	DefaultProvider string    `json:"default_provider"`
	DefaultModel    string    `json:"default_model"`
	MaxParallel     int       `json:"max_parallel"`
	ReviewPolicy    string    `json:"review_policy"`
	NotifyOnBlocked bool      `json:"notify_on_blocked"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Workflow struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Scope       string           `json:"scope"`
	IsBuiltin   bool             `json:"is_builtin"`
	Version     int              `json:"version"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Phases      []*WorkflowPhase `json:"phases,omitempty"`
}

type WorkflowPhase struct {
	ID            string    `json:"id"`
	WorkflowID    string    `json:"workflow_id"`
	Ordinal       int       `json:"ordinal"`
	PhaseType     string    `json:"phase_type"`
	Role          string    `json:"role"`
	EntryRule     string    `json:"entry_rule"`
	ExitRule      string    `json:"exit_rule"`
	MaxParallel   int       `json:"max_parallel"`
	AgentSelector string    `json:"agent_selector"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RoleBinding struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Role        string    `json:"role"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	MaxParallel int       `json:"max_parallel"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RoleLoopAttempt struct {
	TaskID    string    `json:"task_id"`
	RoleName  string    `json:"role_name"`
	Attempts  int       `json:"attempts"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectKnowledgeEntry struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	SourceURI string    `json:"source_uri,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReviewCycle struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"task_id"`
	Iteration  int       `json:"iteration"`
	Status     string    `json:"status"`
	CommitHash string    `json:"commit_hash,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ReviewIssue struct {
	ID         string    `json:"id"`
	CycleID    string    `json:"cycle_id"`
	Severity   string    `json:"severity"`
	Summary    string    `json:"summary"`
	Status     string    `json:"status"`
	Resolution string    `json:"resolution,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ProjectFilter struct {
	Status string
}

type TaskFilter struct {
	ProjectID string
	Status    string
}

type WorktreeFilter struct {
	ProjectID string
	Status    string
	TaskID    string
}

type SessionFilter struct {
	TaskID string
	Status string
}

type DemandPoolFilter struct {
	ProjectID string
	Status    string
	Tag       string
	Query     string
	Limit     int
	Offset    int
}

type AgentConfigFilter struct {
	CostTier  string
	SpeedTier string
}

func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		ts = nowUTC()
	}
	return ts.UTC().Format(time.RFC3339)
}

func parseTimestamp(v string) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp %q: %w", v, err)
	}
	return ts, nil
}

func encodeStringSlice(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	buf, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("failed to encode string slice: %w", err)
	}
	return string(buf), nil
}

func decodeStringSlice(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("failed to decode string slice: %w", err)
	}
	return values, nil
}

func nullIfEmpty(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
