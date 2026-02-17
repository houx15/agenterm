package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileParsesDBPath(t *testing.T) {
	cfg := &Config{}
	cfg.ConfigPath = filepath.Join(t.TempDir(), "config")

	content := "Port=9999\nTmuxSession=ai\nToken=test-token\nDefaultDir=/tmp/work\nDBPath=/tmp/custom/agenterm.db\n"
	if err := os.WriteFile(cfg.ConfigPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file error = %v", err)
	}

	if err := cfg.loadFromFile(); err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}

	if cfg.DBPath != "/tmp/custom/agenterm.db" {
		t.Fatalf("DBPath = %q, want /tmp/custom/agenterm.db", cfg.DBPath)
	}
}
