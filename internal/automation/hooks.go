package automation

import "strings"

// EnsureClaudeCodeAutomation is intentionally non-intrusive.
// Tool-specific hook files should be managed by users/tooling directly.
func EnsureClaudeCodeAutomation(workDir string) error {
	_ = strings.TrimSpace(workDir)
	return nil
}
