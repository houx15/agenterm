package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureClaudeCodeAutomationWritesHooksAndSettings(t *testing.T) {
	workDir := t.TempDir()
	if err := EnsureClaudeCodeAutomation(workDir); err != nil {
		t.Fatalf("EnsureClaudeCodeAutomation error: %v", err)
	}

	autoCommitPath := filepath.Join(workDir, ".orchestra", "hooks", "auto-commit.sh")
	stopPath := filepath.Join(workDir, ".orchestra", "hooks", "on-agent-stop.sh")
	settingsPath := filepath.Join(workDir, ".claude", "settings.json")

	if _, err := os.Stat(autoCommitPath); err != nil {
		t.Fatalf("auto-commit hook missing: %v", err)
	}
	if _, err := os.Stat(stopPath); err != nil {
		t.Fatalf("on-agent-stop hook missing: %v", err)
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	hooks, ok := payload["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing hooks object")
	}
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Fatalf("settings missing PostToolUse")
	}
	if _, ok := hooks["Stop"]; !ok {
		t.Fatalf("settings missing Stop")
	}
}
