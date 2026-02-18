package automation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureClaudeCodeAutomationIsNoop(t *testing.T) {
	workDir := t.TempDir()
	if err := EnsureClaudeCodeAutomation(workDir); err != nil {
		t.Fatalf("EnsureClaudeCodeAutomation error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("expected hook settings to remain unmanaged, got err=%v", err)
	}
}

func TestEnsureClaudeCodeAutomationPreservesExistingSettings(t *testing.T) {
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

	raw, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read existing settings: %v", err)
	}
	if string(raw) != existing {
		t.Fatalf("expected existing settings to remain unchanged")
	}
}
