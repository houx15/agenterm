package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBlueprint(t *testing.T) {
	raw := `{
		"tasks": [
			{
				"id": "task-1",
				"title": "Build API",
				"description": "Create REST endpoints",
				"completion_criteria": ["Tests pass", "Docs updated"],
				"worktree_branch": "feature/api",
				"agent_type": "claude",
				"depends_on": []
			},
			{
				"id": "task-2",
				"title": "Build UI",
				"description": "Create frontend",
				"completion_criteria": ["Renders correctly"],
				"worktree_branch": "feature/ui",
				"agent_type": "codex",
				"depends_on": ["task-1"]
			}
		]
	}`

	bp, err := ParseBlueprint(raw)
	if err != nil {
		t.Fatalf("ParseBlueprint() error = %v", err)
	}
	if len(bp.Tasks) != 2 {
		t.Fatalf("tasks len = %d, want 2", len(bp.Tasks))
	}
	if bp.Tasks[0].ID != "task-1" {
		t.Errorf("task 0 id = %q, want %q", bp.Tasks[0].ID, "task-1")
	}
	if bp.Tasks[0].Title != "Build API" {
		t.Errorf("task 0 title = %q, want %q", bp.Tasks[0].Title, "Build API")
	}
	if len(bp.Tasks[0].CompletionCriteria) != 2 {
		t.Errorf("task 0 criteria len = %d, want 2", len(bp.Tasks[0].CompletionCriteria))
	}
	if bp.Tasks[0].AgentType != "claude" {
		t.Errorf("task 0 agent_type = %q, want %q", bp.Tasks[0].AgentType, "claude")
	}
	if bp.Tasks[1].ID != "task-2" {
		t.Errorf("task 1 id = %q, want %q", bp.Tasks[1].ID, "task-2")
	}
	if len(bp.Tasks[1].DependsOn) != 1 || bp.Tasks[1].DependsOn[0] != "task-1" {
		t.Errorf("task 1 depends_on = %v, want [task-1]", bp.Tasks[1].DependsOn)
	}
}

func TestParseBlueprintEmpty(t *testing.T) {
	bp, err := ParseBlueprint(`{"tasks": []}`)
	if err != nil {
		t.Fatalf("ParseBlueprint() error = %v", err)
	}
	if len(bp.Tasks) != 0 {
		t.Fatalf("tasks len = %d, want 0", len(bp.Tasks))
	}
}

func TestParseBlueprintInvalidJSON(t *testing.T) {
	_, err := ParseBlueprint(`not valid json`)
	if err == nil {
		t.Fatal("ParseBlueprint() expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse blueprint") {
		t.Errorf("error = %v, want to contain 'parse blueprint'", err)
	}
}

func TestParseBlueprintNoDependsOn(t *testing.T) {
	raw := `{
		"tasks": [{
			"id": "task-1",
			"title": "Solo task",
			"description": "No deps",
			"completion_criteria": [],
			"worktree_branch": "feature/solo",
			"agent_type": "claude"
		}]
	}`
	bp, err := ParseBlueprint(raw)
	if err != nil {
		t.Fatalf("ParseBlueprint() error = %v", err)
	}
	if bp.Tasks[0].DependsOn != nil {
		t.Errorf("depends_on = %v, want nil", bp.Tasks[0].DependsOn)
	}
}

func TestContextFileName(t *testing.T) {
	tests := []struct {
		agentType string
		want      string
	}{
		{"claude", "CLAUDE.md"},
		{"Claude", "CLAUDE.md"},
		{"claude-code", "CLAUDE.md"},
		{"Claude-Code", "CLAUDE.md"},
		{"codex", "AGENTS.md"},
		{"opencode", "AGENTS.md"},
		{"custom-agent", "AGENTS.md"},
		{"", "AGENTS.md"},
	}
	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			got := ContextFileName(tt.agentType)
			if got != tt.want {
				t.Errorf("ContextFileName(%q) = %q, want %q", tt.agentType, got, tt.want)
			}
		})
	}
}

func TestGenerateContextFileForClaude(t *testing.T) {
	task := BlueprintTask{
		ID:                 "task-1",
		Title:              "Build the API",
		Description:        "Create REST endpoints for the project",
		CompletionCriteria: []string{"All tests pass", "Code reviewed"},
		WorktreeBranch:     "feature/api",
		AgentType:          "claude",
	}

	content := GenerateContextFile("claude", "MyProject", "Use Go conventions\nRun go vet", task)

	if !strings.HasPrefix(content, "# CLAUDE.md") {
		t.Error("expected content to start with '# CLAUDE.md'")
	}
	if !strings.Contains(content, "## Project") {
		t.Error("expected '## Project' section")
	}
	if !strings.Contains(content, "MyProject") {
		t.Error("expected project name 'MyProject'")
	}
	if !strings.Contains(content, "## Task") {
		t.Error("expected '## Task' section")
	}
	if !strings.Contains(content, "**Build the API**") {
		t.Error("expected task title 'Build the API'")
	}
	if !strings.Contains(content, "Create REST endpoints for the project") {
		t.Error("expected task description")
	}
	if !strings.Contains(content, "## Completion Criteria") {
		t.Error("expected '## Completion Criteria' section")
	}
	if !strings.Contains(content, "- All tests pass") {
		t.Error("expected criterion 'All tests pass'")
	}
	if !strings.Contains(content, "- Code reviewed") {
		t.Error("expected criterion 'Code reviewed'")
	}
	if !strings.Contains(content, "## Rules") {
		t.Error("expected '## Rules' section")
	}
	if !strings.Contains(content, "Do not ask for user input") {
		t.Error("expected autonomous rule")
	}
	if !strings.Contains(content, "BLOCKED.md") {
		t.Error("expected BLOCKED.md rule")
	}
	if !strings.Contains(content, "## Conventions") {
		t.Error("expected '## Conventions' section")
	}
	if !strings.Contains(content, "Use Go conventions") {
		t.Error("expected context template content")
	}
}

func TestGenerateContextFileForOtherAgent(t *testing.T) {
	task := BlueprintTask{
		ID:    "task-1",
		Title: "Build UI",
	}

	content := GenerateContextFile("codex", "WebApp", "", task)

	if !strings.HasPrefix(content, "# AGENTS.md") {
		t.Error("expected content to start with '# AGENTS.md'")
	}
	if !strings.Contains(content, "WebApp") {
		t.Error("expected project name")
	}
	if !strings.Contains(content, "**Build UI**") {
		t.Error("expected task title")
	}
	// No completion criteria section when list is empty.
	if strings.Contains(content, "## Completion Criteria") {
		t.Error("expected no completion criteria section for empty list")
	}
	// No conventions section when template is empty.
	if strings.Contains(content, "## Conventions") {
		t.Error("expected no conventions section for empty template")
	}
}

func TestGenerateContextFileNoDescription(t *testing.T) {
	task := BlueprintTask{
		ID:                 "task-1",
		Title:              "Simple Task",
		CompletionCriteria: []string{"Done"},
	}

	content := GenerateContextFile("claude", "Project", "", task)

	if !strings.Contains(content, "**Simple Task**") {
		t.Error("expected task title")
	}
	// Should contain the rules section.
	if !strings.Contains(content, "## Rules") {
		t.Error("expected rules section")
	}
}

func TestWritePermissionConfigClaude(t *testing.T) {
	dir := t.TempDir()
	config := `{"allow": ["read", "write"], "deny": ["exec"]}`

	err := WritePermissionConfig(dir, "claude", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal settings.json error = %v", err)
	}

	perms, ok := parsed["permissions"]
	if !ok {
		t.Fatal("expected 'permissions' key in settings.json")
	}

	var permObj map[string]any
	if err := json.Unmarshal(perms, &permObj); err != nil {
		t.Fatalf("Unmarshal permissions error = %v", err)
	}
	allow, ok := permObj["allow"].([]any)
	if !ok || len(allow) != 2 {
		t.Fatalf("allow = %v, want [read, write]", permObj["allow"])
	}
}

func TestWritePermissionConfigClaudeCode(t *testing.T) {
	dir := t.TempDir()
	config := `{"mode": "auto"}`

	err := WritePermissionConfig(dir, "claude-code", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("settings.json not created at %s", path)
	}
}

func TestWritePermissionConfigCodex(t *testing.T) {
	dir := t.TempDir()
	config := "allow all file operations\ndeny network access"

	err := WritePermissionConfig(dir, "codex", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(dir, ".codex", "rules", "default.rules")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != config {
		t.Errorf("content = %q, want %q", string(data), config)
	}
}

func TestWritePermissionConfigOpencode(t *testing.T) {
	dir := t.TempDir()
	config := `{"permissions": {"file_access": true}}`

	err := WritePermissionConfig(dir, "opencode", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(dir, "opencode.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != config {
		t.Errorf("content = %q, want %q", string(data), config)
	}
}

func TestWritePermissionConfigDefault(t *testing.T) {
	dir := t.TempDir()
	config := `{"default_perms": true}`

	err := WritePermissionConfig(dir, "custom-agent", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(dir, ".agents", "permissions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != config {
		t.Errorf("content = %q, want %q", string(data), config)
	}
}

func TestWritePermissionConfigCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	config := `{"test": true}`

	err := WritePermissionConfig(dir, "Claude", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected settings.json for agent type 'Claude'")
	}
}

func TestWritePermissionConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	err := WritePermissionConfig(dir, "claude", "not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON config with claude agent type")
	}
}

func TestWritePermissionConfigCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	config := `{"test": true}`

	// Write to a nested path that doesn't exist.
	nested := filepath.Join(dir, "deep", "nested")
	err := WritePermissionConfig(nested, "codex", config)
	if err != nil {
		t.Fatalf("WritePermissionConfig() error = %v", err)
	}

	path := filepath.Join(nested, ".codex", "rules", "default.rules")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected file to be created with nested directories")
	}
}
