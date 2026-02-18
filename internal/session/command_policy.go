package session

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	shellExecPattern = regexp.MustCompile(`(^|[;&|]\s*)(bash|sh|zsh|fish)\s+-c(\s|$)`)
	evalPattern      = regexp.MustCompile(`(^|[;&|]\s*)eval(\s|$)`)
	redirectPattern  = regexp.MustCompile(`(?:^|[\s;|&])(?:>|>>|1>|2>|&>)\s*([^\s]+)`)
)

var commandWrappers = []string{"sudo", "command", "nohup"}

// CommandPolicyError indicates a command was denied by hard-coded policy rules.
type CommandPolicyError struct {
	Rule    string
	Detail  string
	Command string
}

func (e *CommandPolicyError) Error() string {
	return fmt.Sprintf("command blocked by policy (%s): %s", e.Rule, e.Detail)
}

func IsCommandPolicyError(err error) bool {
	_, ok := err.(*CommandPolicyError)
	return ok
}

func ValidateCommandPolicy(raw string, allowedRoot string) error {
	return enforceCommandPolicy(raw, allowedRoot)
}

func AuditCommandPolicyViolation(workDir string, sessionID string, raw string, policyErr *CommandPolicyError) {
	auditCommandPolicyViolation(workDir, sessionID, raw, policyErr)
}

func enforceCommandPolicy(raw string, allowedRoot string) error {
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		return nil
	}
	lower := strings.ToLower(cmd)
	if strings.Contains(cmd, "`") || strings.Contains(cmd, "$(") {
		return &CommandPolicyError{
			Rule:    "no_shell_substitution",
			Detail:  "shell substitution is blocked (` or $())",
			Command: cmd,
		}
	}
	if shellExecPattern.MatchString(lower) {
		return &CommandPolicyError{
			Rule:    "no_shell_dash_c",
			Detail:  "shell -c execution is blocked",
			Command: cmd,
		}
	}
	if evalPattern.MatchString(lower) {
		return &CommandPolicyError{
			Rule:    "no_eval",
			Detail:  "eval is blocked",
			Command: cmd,
		}
	}
	if strings.Contains(cmd, "../") || strings.Contains(cmd, "..\\") {
		return &CommandPolicyError{
			Rule:    "no_path_traversal",
			Detail:  "relative traversal (../) is blocked",
			Command: cmd,
		}
	}

	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return nil
	}
	pathTokens := extractPathTokens(cmd, fields)
	if err := blockPathExpansionVariables(cmd, pathTokens); err != nil {
		return err
	}
	if err := blockAbsoluteRecursiveRemove(cmd, fields); err != nil {
		return err
	}
	if err := validatePathScope(cmd, allowedRoot, pathTokens); err != nil {
		return err
	}
	return nil
}

func blockAbsoluteRecursiveRemove(command string, fields []string) error {
	exe, args := unwrapCommand(fields)
	if !strings.EqualFold(exe, "rm") {
		return nil
	}
	recursive := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && strings.Contains(arg, "r") {
			recursive = true
		}
	}
	if !recursive {
		return nil
	}
	for _, arg := range args {
		normalized := normalizePathToken(arg)
		if strings.HasPrefix(normalized, "-") || !looksLikePath(normalized) {
			continue
		}
		if filepath.IsAbs(normalized) {
			return &CommandPolicyError{
				Rule:    "no_rm_rf_absolute",
				Detail:  "recursive rm with absolute path is blocked",
				Command: command,
			}
		}
	}
	return nil
}

func blockPathExpansionVariables(command string, paths []string) error {
	for _, p := range paths {
		if strings.Contains(p, "$") || strings.Contains(p, "%") {
			return &CommandPolicyError{
				Rule:    "no_env_path_expansion",
				Detail:  "environment variable path expansion is blocked",
				Command: command,
			}
		}
	}
	return nil
}

func unwrapCommand(fields []string) (string, []string) {
	i := 0
	for i < len(fields) {
		tok := strings.TrimSpace(fields[i])
		if tok == "" {
			i++
			continue
		}
		base := filepath.Base(tok)
		if slices.Contains(commandWrappers, strings.ToLower(base)) {
			i++
			continue
		}
		if base == "env" {
			i++
			for i < len(fields) && isEnvAssignment(fields[i]) {
				i++
			}
			continue
		}
		break
	}
	if i >= len(fields) {
		return "", nil
	}
	return filepath.Base(fields[i]), fields[i+1:]
}

func isEnvAssignment(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" || strings.HasPrefix(token, "-") {
		return false
	}
	key, _, ok := strings.Cut(token, "=")
	if !ok || key == "" {
		return false
	}
	for idx, r := range key {
		if idx == 0 && !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func validatePathScope(command string, allowedRoot string, paths []string) error {
	root := strings.TrimSpace(allowedRoot)
	if root == "" {
		if len(paths) > 0 {
			return &CommandPolicyError{
				Rule:    "missing_workdir_scope",
				Detail:  "cannot validate path scope because workdir is unavailable",
				Command: command,
			}
		}
		return nil
	}
	root = canonicalPathForPolicy(root)
	for _, p := range paths {
		p = normalizePathToken(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "~") {
			return &CommandPolicyError{
				Rule:    "no_tilde_path",
				Detail:  "home-expanded paths are blocked",
				Command: command,
			}
		}
		resolved := p
		if filepath.IsAbs(resolved) {
			resolved = filepath.Clean(resolved)
		} else {
			resolved = filepath.Clean(filepath.Join(root, resolved))
		}
		resolved = canonicalPathForPolicy(resolved)
		if !pathWithinBase(root, resolved) {
			return &CommandPolicyError{
				Rule:    "path_outside_workdir",
				Detail:  fmt.Sprintf("path %q is outside allowed workdir", p),
				Command: command,
			}
		}
	}
	return nil
}

func extractPathTokens(raw string, fields []string) []string {
	paths := make([]string, 0, 8)
	for _, f := range fields {
		if f == "" {
			continue
		}
		if strings.HasPrefix(f, "-") {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) == 2 {
				pathPart := normalizePathToken(parts[1])
				if looksLikePath(pathPart) {
					paths = append(paths, pathPart)
				}
			}
			continue
		}
		pathPart := normalizePathToken(f)
		if looksLikePath(pathPart) {
			paths = append(paths, pathPart)
		}
	}
	for _, m := range redirectPattern.FindAllStringSubmatch(raw, -1) {
		if len(m) > 1 {
			pathPart := normalizePathToken(m[1])
			if looksLikePath(pathPart) {
				paths = append(paths, pathPart)
			}
		}
	}
	return paths
}

func looksLikePath(token string) bool {
	token = normalizePathToken(token)
	if token == "" {
		return false
	}
	if strings.Contains(token, "://") {
		return false
	}
	return strings.HasPrefix(token, "/") ||
		strings.HasPrefix(token, ".") ||
		strings.HasPrefix(token, "~") ||
		strings.Contains(token, "/") ||
		strings.Contains(token, "\\")
}

func normalizePathToken(token string) string {
	token = strings.TrimSpace(token)
	for len(token) >= 2 {
		if (token[0] == '\'' && token[len(token)-1] == '\'') || (token[0] == '"' && token[len(token)-1] == '"') {
			token = strings.TrimSpace(token[1 : len(token)-1])
			continue
		}
		break
	}
	return token
}

func canonicalPathForPolicy(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return path
}

func pathWithinBase(base string, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	rel = filepath.Clean(rel)
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func auditCommandPolicyViolation(workDir string, sessionID string, raw string, policyErr *CommandPolicyError) {
	if policyErr == nil {
		return
	}
	slog.Warn("blocked command by policy", "session_id", sessionID, "rule", policyErr.Rule, "detail", policyErr.Detail)

	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return
	}
	auditDir := filepath.Join(workDir, ".orchestra")
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		return
	}
	auditPath := filepath.Join(auditDir, "command-policy-audit.log")
	line := fmt.Sprintf(
		"%s session=%s rule=%s detail=%q command=%q\n",
		time.Now().UTC().Format(time.RFC3339),
		sessionID,
		policyErr.Rule,
		policyErr.Detail,
		strings.TrimSpace(raw),
	)
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}
