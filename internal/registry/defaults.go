package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/agenterm/configs"
)

var defaultAgentFiles = []string{
	"claude-code.yaml",
	"codex.yaml",
	"gemini-cli.yaml",
	"opencode.yaml",
	"kimi-cli.yaml",
}

func ensureDefaults(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read registry dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			return nil
		}
	}

	for _, file := range defaultAgentFiles {
		content, err := configs.AgentDefaults.ReadFile(filepath.Join("agents", file))
		if err != nil {
			return fmt.Errorf("read embedded default %q: %w", file, err)
		}
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write default %q: %w", path, err)
		}
	}

	return nil
}
