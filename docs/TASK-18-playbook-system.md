# Task: playbook-system

## Context
Playbooks define reusable orchestration patterns — how tasks should be phased, which agents to use for which roles, and parallelism strategies. They let users codify their preferred development workflows (e.g., "always use Codex for scaffolding, Claude for implementation, then cross-review").

## Objective
Implement the Playbook system: YAML config loading, API endpoints, integration with the orchestrator, and a settings page for managing playbooks.

## Dependencies
- Depends on: TASK-09 (agent-registry), TASK-15 (orchestrator)
- Branch: feature/playbook-system
- Base: main (after dependencies merge)

## Scope

### Files to Create
- `internal/playbook/playbook.go` — Playbook loading, validation, matching
- `internal/playbook/types.go` — Playbook data structures
- `internal/api/playbooks.go` — API handlers for playbook CRUD
- `configs/playbooks/default.yaml` — Default playbook
- `configs/playbooks/go-backend.yaml` — Go backend example playbook
- `frontend/src/pages/Settings.tsx` — Settings page with agent + playbook management

### Files to Modify
- `internal/orchestrator/prompt.go` — Include matched playbook in system prompt
- `internal/config/config.go` — Add `PlaybooksDir` config field
- `cmd/agenterm/main.go` — Initialize playbook loader

## Implementation Spec

### Step 1: Playbook types (from SPEC 3.6)
```go
type Playbook struct {
    Name                string  `yaml:"name" json:"name"`
    Description         string  `yaml:"description" json:"description"`
    Match               Match   `yaml:"match" json:"match"`
    Phases              []Phase `yaml:"phases" json:"phases"`
    ParallelismStrategy string  `yaml:"parallelism_strategy" json:"parallelism_strategy"`
}

type Match struct {
    Languages       []string `yaml:"languages" json:"languages"`
    ProjectPatterns []string `yaml:"project_patterns" json:"project_patterns"`
}

type Phase struct {
    Name        string `yaml:"name" json:"name"`
    Agent       string `yaml:"agent" json:"agent"`
    Role        string `yaml:"role" json:"role"`
    Description string `yaml:"description" json:"description"`
}
```

### Step 2: Playbook loader
- Load YAML files from `~/.config/agenterm/playbooks/`
- Auto-match playbook to project based on Match criteria
- Provide default playbook when no match

### Step 3: Integration with orchestrator
- When building system prompt, include the matched playbook's phases
- The orchestrator uses phase descriptions to guide task decomposition and agent assignment

### Step 4: API
```
GET    /api/playbooks          — List all playbooks
GET    /api/playbooks/{id}     — Get playbook detail
POST   /api/playbooks          — Create playbook
PUT    /api/playbooks/{id}     — Update playbook
DELETE /api/playbooks/{id}     — Delete playbook
```

### Step 5: Settings page
- Tab 1: Agent Registry — list/edit/add agent configs
- Tab 2: Playbooks — list/edit/add playbook configs
- YAML editor with syntax highlighting for advanced editing

## Acceptance Criteria
- [x] Playbooks loaded from YAML directory
- [x] Auto-match playbook to project based on language/patterns
- [x] API CRUD for playbooks
- [x] Orchestrator system prompt includes playbook phases
- [x] Settings page for managing agents and playbooks
- [x] Default playbook created on first run

## Notes
- Playbook matching is best-effort — user can override via project config
- The parallelism_strategy is natural language guidance for the orchestrator, not executable code
- Keep the YAML format simple and human-editable
