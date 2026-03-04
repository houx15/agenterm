# Task: agent-registry

## Context
AgenTerm needs to know what AI coding agents are available (Claude Code, Codex, Gemini CLI, OpenCode, Kimi CLI, etc.) and their capabilities. The Agent Registry is a YAML-based configuration system where users define their available agents. The system loads these configs and exposes them via API.

## Objective
Implement Agent Registry: YAML config loading from `~/.config/agenterm/agents/`, validation, API endpoints, and a frontend settings page for managing agent configs.

## Dependencies
- Depends on: TASK-08 (rest-api)
- Branch: feature/agent-registry
- Base: main (after TASK-08 merge)

## Scope

### Files to Create
- `internal/registry/registry.go` — Agent registry: load YAML dir, validate, cache, reload
- `internal/registry/types.go` — AgentConfig struct with YAML tags
- `internal/registry/defaults.go` — Default agent configs (claude-code, codex, gemini-cli) created on first run
- `configs/agents/claude-code.yaml` — Example agent config (shipped with binary)
- `configs/agents/codex.yaml` — Example agent config
- `configs/agents/gemini-cli.yaml` — Example agent config

### Files to Modify
- `internal/api/agents.go` — Wire to registry instead of DB
- `internal/config/config.go` — Add `AgentsDir` config field
- `cmd/agenterm/main.go` — Initialize registry, pass to API

### Files NOT to Touch
- `internal/tmux/` — No changes
- `internal/hub/` — No changes

## Implementation Spec

### Step 1: Define AgentConfig type
```go
type AgentConfig struct {
    ID                    string   `yaml:"id" json:"id"`
    Name                  string   `yaml:"name" json:"name"`
    Command               string   `yaml:"command" json:"command"`
    ResumeCommand         string   `yaml:"resume_command,omitempty" json:"resume_command,omitempty"`
    HeadlessCommand       string   `yaml:"headless_command,omitempty" json:"headless_command,omitempty"`
    Capabilities          []string `yaml:"capabilities" json:"capabilities"`
    Languages             []string `yaml:"languages" json:"languages"`
    CostTier              string   `yaml:"cost_tier" json:"cost_tier"`
    SpeedTier             string   `yaml:"speed_tier" json:"speed_tier"`
    SupportsSessionResume bool     `yaml:"supports_session_resume" json:"supports_session_resume"`
    SupportsHeadless      bool     `yaml:"supports_headless" json:"supports_headless"`
    AutoAcceptMode        string   `yaml:"auto_accept_mode,omitempty" json:"auto_accept_mode,omitempty"`
}
```

### Step 2: Registry implementation
```go
type Registry struct {
    dir    string
    agents map[string]*AgentConfig
    mu     sync.RWMutex
}

func NewRegistry(dir string) (*Registry, error)  // Load all YAML files
func (r *Registry) Get(id string) *AgentConfig
func (r *Registry) List() []*AgentConfig
func (r *Registry) Reload() error                // Re-read directory
func (r *Registry) Save(config *AgentConfig) error // Write YAML file
func (r *Registry) Delete(id string) error
```

### Step 3: Create default configs on first run
If `~/.config/agenterm/agents/` is empty, create default YAML files for:
- claude-code (Claude Code with sonnet model)
- codex (OpenAI Codex CLI)
- gemini-cli (Google Gemini CLI)
- opencode (OpenCode for GLM/MiniMax)
- kimi-cli (Kimi CLI)

### Step 4: API endpoints
- `GET /api/agents` — List all agents from registry
- `GET /api/agents/{id}` — Get specific agent config
- `POST /api/agents` — Create/save new agent config
- `PUT /api/agents/{id}` — Update agent config
- `DELETE /api/agents/{id}` — Delete agent config

## Testing Requirements
- Test loading valid YAML configs
- Test validation (missing required fields)
- Test default config creation
- Test reload after file changes

## Acceptance Criteria
- [x] YAML agent configs loaded from configurable directory
- [x] Default configs created on first run
- [x] API CRUD for agent configs
- [x] Validation: id, name, command are required
- [x] Thread-safe access to registry

## Notes
- Use `gopkg.in/yaml.v3` for YAML parsing
- Agent IDs should be filesystem-safe (lowercase, hyphens only)
- The registry is the source of truth — DB just references agent IDs
