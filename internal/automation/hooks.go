package automation

import (
	"bytes"
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

const (
	autoCommitCommand  = "bash scripts/auto-commit.sh"
	onAgentStopCommand = "bash scripts/on-agent-stop.sh"
)

func EnsureClaudeCodeAutomation(workDir string) error {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return fmt.Errorf("workdir is required")
	}
	return ensureClaudeSettings(workDir)
}

func ensureClaudeSettings(workDir string) error {
	claudeDir := filepath.Join(workDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")

	root := map[string]json.RawMessage{}
	if raw, err := os.ReadFile(settingsPath); err == nil && strings.TrimSpace(string(raw)) != "" {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse existing claude settings: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read claude settings: %w", err)
	}

	settings := claudeSettings{Hooks: map[string][]hookRule{}}
	if existingHooksRaw, ok := root["hooks"]; ok && len(existingHooksRaw) > 0 && string(existingHooksRaw) != "null" {
		if err := json.Unmarshal(existingHooksRaw, &settings.Hooks); err != nil {
			return fmt.Errorf("parse existing claude hooks: %w", err)
		}
	}
	settings.Hooks["PostToolUse"] = upsertHookRule(
		settings.Hooks["PostToolUse"],
		"Write|Edit",
		hookEntry{Type: "command", Command: autoCommitCommand},
	)
	settings.Hooks["Stop"] = upsertHookRule(
		settings.Hooks["Stop"],
		"",
		hookEntry{Type: "command", Command: onAgentStopCommand},
	)

	hooksRaw, err := json.Marshal(settings.Hooks)
	if err != nil {
		return fmt.Errorf("marshal claude hooks: %w", err)
	}
	root["hooks"] = hooksRaw

	rawRoot, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal claude settings: %w", err)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, rawRoot, "", "  "); err != nil {
		return fmt.Errorf("format claude settings: %w", err)
	}
	pretty.WriteByte('\n')
	if err := os.WriteFile(settingsPath, pretty.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}
	return nil
}

func upsertHookRule(rules []hookRule, matcher string, entry hookEntry) []hookRule {
	for i := range rules {
		if strings.TrimSpace(rules[i].Matcher) != strings.TrimSpace(matcher) {
			continue
		}
		if !containsHookEntry(rules[i].Hooks, entry) {
			rules[i].Hooks = append(rules[i].Hooks, entry)
		}
		return rules
	}
	return append(rules, hookRule{
		Matcher: matcher,
		Hooks:   []hookEntry{entry},
	})
}

func containsHookEntry(entries []hookEntry, target hookEntry) bool {
	for _, entry := range entries {
		if strings.TrimSpace(entry.Type) == strings.TrimSpace(target.Type) &&
			strings.TrimSpace(entry.Command) == strings.TrimSpace(target.Command) {
			return true
		}
	}
	return false
}
