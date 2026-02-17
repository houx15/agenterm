package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type AgentConfigRepo struct {
	db *sql.DB
}

func NewAgentConfigRepo(db *sql.DB) *AgentConfigRepo {
	return &AgentConfigRepo{db: db}
}

func (r *AgentConfigRepo) Create(ctx context.Context, agent *AgentConfig) error {
	if agent.ID == "" {
		id, err := NewID()
		if err != nil {
			return err
		}
		agent.ID = id
	}

	capabilitiesRaw, err := encodeStringSlice(agent.Capabilities)
	if err != nil {
		return err
	}
	languagesRaw, err := encodeStringSlice(agent.Languages)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO agent_configs (
	id, name, command, resume_command, headless_command, capabilities, languages,
	cost_tier, speed_tier, supports_session_resume, supports_headless, auto_accept_mode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, agent.ID, agent.Name, agent.Command, agent.ResumeCommand, agent.HeadlessCommand, capabilitiesRaw, languagesRaw, agent.CostTier, agent.SpeedTier, boolToInt(agent.SupportsSessionResume), boolToInt(agent.SupportsHeadless), agent.AutoAcceptMode)
	if err != nil {
		return fmt.Errorf("failed to create agent config: %w", err)
	}
	return nil
}

func (r *AgentConfigRepo) Get(ctx context.Context, id string) (*AgentConfig, error) {
	var a AgentConfig
	var capabilitiesRaw, languagesRaw string
	var supportsSessionResumeInt, supportsHeadlessInt int

	err := r.db.QueryRowContext(ctx, `
SELECT id, name, command, resume_command, headless_command, capabilities, languages,
	cost_tier, speed_tier, supports_session_resume, supports_headless, auto_accept_mode
FROM agent_configs
WHERE id = ?
`, id).Scan(&a.ID, &a.Name, &a.Command, &a.ResumeCommand, &a.HeadlessCommand, &capabilitiesRaw, &languagesRaw, &a.CostTier, &a.SpeedTier, &supportsSessionResumeInt, &supportsHeadlessInt, &a.AutoAcceptMode)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get agent config %q: %w", id, err)
	}

	a.Capabilities, err = decodeStringSlice(capabilitiesRaw)
	if err != nil {
		return nil, err
	}
	a.Languages, err = decodeStringSlice(languagesRaw)
	if err != nil {
		return nil, err
	}
	a.SupportsSessionResume = supportsSessionResumeInt != 0
	a.SupportsHeadless = supportsHeadlessInt != 0

	return &a, nil
}

func (r *AgentConfigRepo) List(ctx context.Context, filter AgentConfigFilter) ([]*AgentConfig, error) {
	query := `
SELECT id, name, command, resume_command, headless_command, capabilities, languages,
	cost_tier, speed_tier, supports_session_resume, supports_headless, auto_accept_mode
FROM agent_configs
`
	args := []any{}
	where := []string{}

	if filter.CostTier != "" {
		where = append(where, "cost_tier = ?")
		args = append(args, filter.CostTier)
	}
	if filter.SpeedTier != "" {
		where = append(where, "speed_tier = ?")
		args = append(args, filter.SpeedTier)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY name ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent configs: %w", err)
	}
	defer rows.Close()

	agents := []*AgentConfig{}
	for rows.Next() {
		var a AgentConfig
		var capabilitiesRaw, languagesRaw string
		var supportsSessionResumeInt, supportsHeadlessInt int
		if err := rows.Scan(&a.ID, &a.Name, &a.Command, &a.ResumeCommand, &a.HeadlessCommand, &capabilitiesRaw, &languagesRaw, &a.CostTier, &a.SpeedTier, &supportsSessionResumeInt, &supportsHeadlessInt, &a.AutoAcceptMode); err != nil {
			return nil, fmt.Errorf("failed to scan agent config: %w", err)
		}
		a.Capabilities, err = decodeStringSlice(capabilitiesRaw)
		if err != nil {
			return nil, err
		}
		a.Languages, err = decodeStringSlice(languagesRaw)
		if err != nil {
			return nil, err
		}
		a.SupportsSessionResume = supportsSessionResumeInt != 0
		a.SupportsHeadless = supportsHeadlessInt != 0
		agents = append(agents, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed while iterating agent configs: %w", err)
	}

	return agents, nil
}

func (r *AgentConfigRepo) Update(ctx context.Context, agent *AgentConfig) error {
	capabilitiesRaw, err := encodeStringSlice(agent.Capabilities)
	if err != nil {
		return err
	}
	languagesRaw, err := encodeStringSlice(agent.Languages)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, `
UPDATE agent_configs
SET name = ?, command = ?, resume_command = ?, headless_command = ?, capabilities = ?, languages = ?,
	cost_tier = ?, speed_tier = ?, supports_session_resume = ?, supports_headless = ?, auto_accept_mode = ?
WHERE id = ?
`, agent.Name, agent.Command, agent.ResumeCommand, agent.HeadlessCommand, capabilitiesRaw, languagesRaw, agent.CostTier, agent.SpeedTier, boolToInt(agent.SupportsSessionResume), boolToInt(agent.SupportsHeadless), agent.AutoAcceptMode, agent.ID)
	if err != nil {
		return fmt.Errorf("failed to update agent config %q: %w", agent.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read updated rows for agent config %q: %w", agent.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("agent config %q not found", agent.ID)
	}
	return nil
}

func (r *AgentConfigRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM agent_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent config %q: %w", id, err)
	}
	return nil
}

func (r *AgentConfigRepo) LoadFromYAMLDir(ctx context.Context, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read agent YAML dir %q: %w", dir, err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		agents, err := LoadAgentConfigsFromYAML(filepath.Join(dir, entry.Name()))
		if err != nil {
			return count, err
		}
		for i := range agents {
			agent := agents[i]
			existing, err := r.Get(ctx, agent.ID)
			if err != nil {
				return count, err
			}
			if existing == nil {
				if err := r.Create(ctx, &agent); err != nil {
					return count, err
				}
			} else {
				if err := r.Update(ctx, &agent); err != nil {
					return count, err
				}
			}
			count++
		}
	}

	return count, nil
}

func LoadAgentConfigsFromYAML(path string) ([]AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent YAML file %q: %w", path, err)
	}

	var single AgentConfig
	if err := yaml.Unmarshal(data, &single); err == nil && single.ID != "" {
		return []AgentConfig{single}, nil
	}

	var list []AgentConfig
	if err := yaml.Unmarshal(data, &list); err == nil {
		return list, nil
	}

	var wrapped struct {
		Agents []AgentConfig `yaml:"agents"`
	}
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to parse agent YAML file %q: %w", path, err)
	}

	return wrapped.Agents, nil
}
