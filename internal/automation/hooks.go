package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type claudeSettings struct {
	Hooks map[string][]hookRule `json:"hooks"`
}

type hookRule struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

const autoCommitScript = `#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "${repo_root}" ]]; then
  exit 0
fi

if git -C "${repo_root}" diff --name-only --diff-filter=U | grep -q .; then
  exit 0
fi

if [[ -z "$(git -C "${repo_root}" status --porcelain)" ]]; then
  exit 0
fi

git -C "${repo_root}" add -A
if git -C "${repo_root}" diff --cached --quiet; then
  exit 0
fi

git -C "${repo_root}" commit -m "[auto] tool-write checkpoint" >/dev/null 2>&1 || true
`

const onAgentStopScript = `#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "${repo_root}" ]]; then
  exit 0
fi

mkdir -p "${repo_root}/.orchestra"
: > "${repo_root}/.orchestra/done"

if [[ -n "$(git -C "${repo_root}" status --porcelain)" ]]; then
  git -C "${repo_root}" add -A
  if ! git -C "${repo_root}" diff --cached --quiet; then
    git -C "${repo_root}" commit -m "[READY_FOR_REVIEW] agent completed run" >/dev/null 2>&1 || true
  fi
fi
`

func EnsureClaudeCodeAutomation(workDir string) error {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return fmt.Errorf("workdir is required")
	}
	if err := ensureHookScripts(workDir); err != nil {
		return err
	}
	return ensureClaudeSettings(workDir)
}

func ensureHookScripts(workDir string) error {
	hooksDir := filepath.Join(workDir, ".orchestra", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "auto-commit.sh"), []byte(autoCommitScript), 0o755); err != nil {
		return fmt.Errorf("write auto-commit hook: %w", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "on-agent-stop.sh"), []byte(onAgentStopScript), 0o755); err != nil {
		return fmt.Errorf("write on-agent-stop hook: %w", err)
	}
	return nil
}

func ensureClaudeSettings(workDir string) error {
	claudeDir := filepath.Join(workDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	settings := claudeSettings{
		Hooks: map[string][]hookRule{
			"PostToolUse": {
				{
					Matcher: "Write|Edit",
					Hooks: []hookEntry{
						{Type: "command", Command: ".orchestra/hooks/auto-commit.sh"},
					},
				},
			},
			"Stop": {
				{
					Hooks: []hookEntry{
						{Type: "command", Command: ".orchestra/hooks/on-agent-stop.sh"},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}
	return nil
}
