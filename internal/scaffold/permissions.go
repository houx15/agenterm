package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WritePermissionConfig writes the agent-specific permission configuration
// file into the given worktree directory.
func WritePermissionConfig(worktreePath string, agentType string, configJSON string) error {
	agentType = strings.ToLower(agentType)

	var targetPath string
	var content string

	switch agentType {
	case "claude", "claude-code":
		targetPath = filepath.Join(worktreePath, ".claude", "settings.json")
		wrapped, err := wrapClaudePermissions(configJSON)
		if err != nil {
			return fmt.Errorf("wrap claude permissions: %w", err)
		}
		content = wrapped

	case "codex":
		targetPath = filepath.Join(worktreePath, ".codex", "rules", "default.rules")
		content = configJSON

	case "opencode":
		targetPath = filepath.Join(worktreePath, "opencode.json")
		content = configJSON

	default:
		targetPath = filepath.Join(worktreePath, ".agents", "permissions.json")
		content = configJSON
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write permission config %s: %w", targetPath, err)
	}

	return nil
}

// wrapClaudePermissions wraps the given config JSON inside a
// {"permissions": <config>} wrapper for Claude settings.
func wrapClaudePermissions(configJSON string) (string, error) {
	var config json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", fmt.Errorf("invalid config JSON: %w", err)
	}

	wrapper := map[string]json.RawMessage{
		"permissions": config,
	}

	out, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal wrapped permissions: %w", err)
	}

	return string(out) + "\n", nil
}
