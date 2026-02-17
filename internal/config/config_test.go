package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileParsesDBPath(t *testing.T) {
	cfg := &Config{}
	cfg.ConfigPath = filepath.Join(t.TempDir(), "config")

	content := "Port=9999\nTmuxSession=ai\nToken=test-token\nDefaultDir=/tmp/work\nDBPath=/tmp/custom/agenterm.db\nAgentsDir=/tmp/custom/agents\nPlaybooksDir=/tmp/custom/playbooks\nLLMAPIKey=test-llm-key\nLLMModel=claude-sonnet-test\nLLMBaseURL=https://example.invalid/v1/messages\nOrchestratorGlobalMaxParallel=19\n"
	if err := os.WriteFile(cfg.ConfigPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file error = %v", err)
	}

	if err := cfg.loadFromFile(); err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}

	if cfg.DBPath != "/tmp/custom/agenterm.db" {
		t.Fatalf("DBPath = %q, want /tmp/custom/agenterm.db", cfg.DBPath)
	}
	if cfg.AgentsDir != "/tmp/custom/agents" {
		t.Fatalf("AgentsDir = %q, want /tmp/custom/agents", cfg.AgentsDir)
	}
	if cfg.PlaybooksDir != "/tmp/custom/playbooks" {
		t.Fatalf("PlaybooksDir = %q, want /tmp/custom/playbooks", cfg.PlaybooksDir)
	}
	if cfg.LLMAPIKey != "test-llm-key" {
		t.Fatalf("LLMAPIKey = %q, want test-llm-key", cfg.LLMAPIKey)
	}
	if cfg.LLMModel != "claude-sonnet-test" {
		t.Fatalf("LLMModel = %q, want claude-sonnet-test", cfg.LLMModel)
	}
	if cfg.LLMBaseURL != "https://example.invalid/v1/messages" {
		t.Fatalf("LLMBaseURL = %q, want https://example.invalid/v1/messages", cfg.LLMBaseURL)
	}
	if cfg.OrchestratorGlobalMaxParallel != 19 {
		t.Fatalf("OrchestratorGlobalMaxParallel = %d, want 19", cfg.OrchestratorGlobalMaxParallel)
	}
}
