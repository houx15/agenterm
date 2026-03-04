package scaffold

import (
	"encoding/json"
	"fmt"
)

// Blueprint represents the output of a planning session.
type Blueprint struct {
	Tasks []BlueprintTask `json:"tasks"`
}

// BlueprintTask represents a single task in a blueprint.
type BlueprintTask struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	CompletionCriteria []string `json:"completion_criteria"`
	WorktreeBranch     string   `json:"worktree_branch"`
	AgentType          string   `json:"agent_type"`
	DependsOn          []string `json:"depends_on,omitempty"`
}

// ParseBlueprint parses a JSON blueprint string into a Blueprint struct.
func ParseBlueprint(raw string) (*Blueprint, error) {
	var bp Blueprint
	if err := json.Unmarshal([]byte(raw), &bp); err != nil {
		return nil, fmt.Errorf("parse blueprint: %w", err)
	}
	return &bp, nil
}
