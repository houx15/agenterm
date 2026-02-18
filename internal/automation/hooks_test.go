package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureClaudeCodeAutomationWritesSettingsWithCommands(t *testing.T) {
	workDir := t.TempDir()
	if err := EnsureClaudeCodeAutomation(workDir); err != nil {
		t.Fatalf("EnsureClaudeCodeAutomation error: %v", err)
	}

	settingsPath := filepath.Join(workDir, ".claude", "settings.json")

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

func TestEnsureClaudeCodeAutomationMergesExistingSettings(t *testing.T) {
	workDir := t.TempDir()
	claudeDir := filepath.Join(workDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	existing := `{
  "model": "claude-sonnet",
  "hooks": {
    "PostToolUse": [
      {"matcher": "Read", "hooks": [{"type":"command","command":"echo read"}]}
    ]
  }
}
`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing settings: %v", err)
	}

	if err := EnsureClaudeCodeAutomation(workDir); err != nil {
		t.Fatalf("EnsureClaudeCodeAutomation error: %v", err)
	}
	if err := EnsureClaudeCodeAutomation(workDir); err != nil {
		t.Fatalf("EnsureClaudeCodeAutomation second run error: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read merged settings: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal merged settings: %v", err)
	}
	if got, ok := payload["model"].(string); !ok || got != "claude-sonnet" {
		t.Fatalf("expected model to be preserved, got %v", payload["model"])
	}
	hooks, ok := payload["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing hooks object")
	}
	post, ok := hooks["PostToolUse"].([]any)
	if !ok || len(post) == 0 {
		t.Fatalf("PostToolUse hooks missing")
	}

	foundRead := false
	foundWriteEdit := false
	for _, ruleAny := range post {
		rule, ok := ruleAny.(map[string]any)
		if !ok {
			continue
		}
		matcher, _ := rule["matcher"].(string)
		hookList, _ := rule["hooks"].([]any)
		for _, hookAny := range hookList {
			hookMap, ok := hookAny.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hookMap["command"].(string)
			if matcher == "Read" && cmd == "echo read" {
				foundRead = true
			}
			if matcher == "Write|Edit" && cmd == "bash scripts/auto-commit.sh" {
				foundWriteEdit = true
			}
		}
	}
	if !foundRead {
		t.Fatalf("expected existing Read hook to be preserved")
	}
	if !foundWriteEdit {
		t.Fatalf("expected Write|Edit auto-commit command hook to be present")
	}
}
