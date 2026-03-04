# agenTerm v2 — Human Orchestrator Rewrite Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite agenTerm from an AI orchestrator platform to a human orchestrator's control plane, keeping proven runtime packages and rebuilding the frontend with Tailwind CSS.

**Architecture:** Go backend (session/pty/hub/db/registry/parser kept, orchestrator/automation/playbook/tmux removed, scaffold package added) + React/TypeScript/Tailwind/xterm.js frontend in Tauri desktop shell.

**Tech Stack:** Go 1.22, SQLite, React 18, TypeScript, Tailwind CSS, xterm.js, Vite, Tauri 1

**Design doc:** `docs/plans/2026-03-04-pivot-rewrite-design.md`

---

## Phase 0: Archive & Clean (prep)

### Task 0.1: Create v1-archive branch

**Files:** None modified

**Step 1: Create archive branch from current main**

```bash
git branch v1-archive
```

**Step 2: Verify branch exists**

```bash
git branch --list v1-archive
```

Expected: `v1-archive` listed

**Step 3: Commit** — no commit needed, branch creation only

---

### Task 0.2: Delete orchestrator package

**Files:**
- Delete: `internal/orchestrator/` (entire directory)

**Step 1: Remove the directory**

```bash
rm -rf internal/orchestrator/
```

**Step 2: Fix compilation — update `cmd/agenterm/main.go`**

Remove the import `"agenterm/internal/orchestrator"` and all references:
- Remove `orchestratorInst` and `demandOrchestratorInst` creation
- Remove `orchestrator.EventTrigger` creation
- Remove orchestrator params from `api.NewRouter()`

**Step 3: Fix compilation — update `internal/api/router.go`**

Remove `orchestrator` and `demandOrchestrator` fields from the `handler` struct. Remove orchestrator params from `NewRouter()` constructor. Remove all orchestrator route registrations.

**Step 4: Delete orchestrator handler files**

```bash
rm internal/api/orchestrator.go
rm internal/api/orchestrator_governance.go
rm internal/api/orchestrator_exceptions.go
rm internal/api/orchestrator_interface_flow_test.go
rm internal/api/orchestrator_governance_test.go
```

**Step 5: Verify it compiles**

```bash
go build ./...
```

**Step 6: Run tests**

```bash
go test ./...
```

Fix any failures from removed dependencies.

**Step 7: Commit**

```bash
git add -A && git commit -m "refactor: remove orchestrator package and handlers"
```

---

### Task 0.3: Delete automation package

**Files:**
- Delete: `internal/automation/` (entire directory)

**Step 1: Remove the directory**

```bash
rm -rf internal/automation/
```

**Step 2: Fix `cmd/agenterm/main.go`**

Remove import `"agenterm/internal/automation"` and all AutoCommitter, Coordinator, MergeController creation/start calls.

**Step 3: Verify it compiles and tests pass**

```bash
go build ./... && go test ./...
```

**Step 4: Commit**

```bash
git add -A && git commit -m "refactor: remove automation package (autocommit, coordinator, merger)"
```

---

### Task 0.4: Delete playbook package and handlers

**Files:**
- Delete: `internal/playbook/` (entire directory)
- Delete: `internal/api/playbooks.go`, `internal/api/playbooks_test.go`

**Step 1: Remove the directories and files**

```bash
rm -rf internal/playbook/
rm -f internal/api/playbooks.go internal/api/playbooks_test.go
```

**Step 2: Fix `cmd/agenterm/main.go`**

Remove import and `playbookRegistry` references.

**Step 3: Fix `internal/api/router.go`**

Remove `playbookRegistry` field from handler struct and constructor. Remove playbook route registrations.

**Step 4: Verify and commit**

```bash
go build ./... && go test ./...
git add -A && git commit -m "refactor: remove playbook package and handlers"
```

---

### Task 0.5: Delete tmux package

**Files:**
- Delete: `internal/tmux/` (entire directory)

**Step 1: Remove and verify**

```bash
rm -rf internal/tmux/
go build ./... && go test ./...
```

**Step 2: Commit**

```bash
git add -A && git commit -m "refactor: remove legacy tmux package"
```

---

### Task 0.6: Delete ASR handlers

**Files:**
- Delete: `internal/api/asr.go`, `internal/api/asr_volc.go`, `internal/api/asr_test.go`

**Step 1: Remove files, fix router (remove ASR route and asrTranscriber field), verify**

```bash
rm -f internal/api/asr.go internal/api/asr_volc.go internal/api/asr_test.go
```

Fix `router.go` and `main.go` to remove ASR references.

```bash
go build ./... && go test ./...
git add -A && git commit -m "refactor: remove ASR transcription handlers"
```

---

### Task 0.7: Clean up router — remove governance/workflow/role-binding routes

**Files:**
- Modify: `internal/api/router.go` — remove governance, workflow, role-binding route registrations
- Modify: `internal/api/run_state.go` — remove workflow CRUD handlers (keep run state handlers)
- Delete: `internal/api/orchestrator_governance.go` (if not already deleted)

**Step 1: Remove from router.go**

Remove route registrations for:
- `/api/projects/{id}/orchestrator` (GET, PATCH)
- `/api/projects/{id}/orchestrator/assignments/*`
- `/api/projects/{id}/orchestrator/exceptions/*`
- `/api/workflows` (GET, POST)
- `/api/workflows/{id}` (PUT, DELETE)
- `/api/projects/{id}/role-bindings` (GET, PUT)

**Step 2: Clean handler struct**

Remove fields: `projectOrchestratorRepo`, `workflowRepo`, `roleBindingRepo`, `roleAgentAssignRepo`, `exceptionMu`, `resolvedException`

**Step 3: Clean main.go**

Remove corresponding repo instantiations and router params.

**Step 4: Verify and commit**

```bash
go build ./... && go test ./...
git add -A && git commit -m "refactor: remove governance, workflow, and role-binding routes"
```

---

### Task 0.8: Clean up DB repos for dropped tables

**Files:**
- Delete: `internal/db/orchestrator_governance_repos.go` (or similar files for dropped tables)
- Modify: `internal/db/models.go` — remove model structs for dropped tables

**Step 1: Identify and remove repo files for dropped tables**

Remove repo files/code for: `orchestrator_messages`, `workflows`, `workflow_phases`, `project_orchestrators`, `role_bindings`, `role_agent_assignments`, `role_loop_attempts`

Remove model structs: `ProjectOrchestrator`, `Workflow`, `WorkflowPhase`, `RoleBinding`, `RoleAgentAssignment`, `RoleLoopAttempt`

Keep: `ProjectKnowledgeEntry`, `ReviewCycle`, `ReviewIssue`, `DemandPoolItem`, `ProjectRun`, `StageRun`

**Step 2: Verify and commit**

```bash
go build ./... && go test ./...
git add -A && git commit -m "refactor: remove DB repos and models for dropped tables"
```

---

## Phase 1: New Data Model

### Task 1.1: Add migration v10 — new tables + schema changes

**Files:**
- Modify: `internal/db/migrations.go` — add migration v10

**Step 1: Write the failing test**

Create `internal/db/migrations_v10_test.go`:

```go
func TestMigrationV10CreatesRequirementsTable(t *testing.T) {
    db := openTestDB(t)
    // Insert a requirement row
    _, err := db.Exec(`INSERT INTO requirements (id, project_id, title, priority, status, created_at, updated_at) VALUES ('r1', 'p1', 'Test', 0, 'queued', datetime('now'), datetime('now'))`)
    if err != nil {
        t.Fatalf("insert requirement: %v", err)
    }
}

func TestMigrationV10CreatesPlanningSessionsTable(t *testing.T) {
    db := openTestDB(t)
    _, err := db.Exec(`INSERT INTO planning_sessions (id, requirement_id, status, created_at, updated_at) VALUES ('ps1', 'r1', 'active', datetime('now'), datetime('now'))`)
    if err != nil {
        t.Fatalf("insert planning_session: %v", err)
    }
}

func TestMigrationV10CreatesPermissionTemplatesTable(t *testing.T) {
    db := openTestDB(t)
    _, err := db.Exec(`INSERT INTO permission_templates (id, agent_type, name, config, created_at, updated_at) VALUES ('pt1', 'claude', 'standard', '{}', datetime('now'), datetime('now'))`)
    if err != nil {
        t.Fatalf("insert permission_template: %v", err)
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/db/ -run TestMigrationV10 -v
```

Expected: FAIL — tables don't exist

**Step 3: Write migration v10 in `migrations.go`**

```go
{
    Version: 10,
    Name:    "create requirements, planning sessions, permission templates",
    SQL: `
CREATE TABLE IF NOT EXISTS requirements (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    priority    INTEGER NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'queued',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_requirements_project_id ON requirements(project_id);
CREATE INDEX IF NOT EXISTS idx_requirements_status ON requirements(status);
CREATE INDEX IF NOT EXISTS idx_requirements_priority ON requirements(project_id, priority DESC);

CREATE TABLE IF NOT EXISTS planning_sessions (
    id                TEXT PRIMARY KEY,
    requirement_id    TEXT NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    agent_session_id  TEXT REFERENCES sessions(id) ON DELETE SET NULL,
    status            TEXT NOT NULL DEFAULT 'active',
    blueprint         TEXT DEFAULT '',
    created_at        DATETIME NOT NULL,
    updated_at        DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_requirement ON planning_sessions(requirement_id);

CREATE TABLE IF NOT EXISTS permission_templates (
    id          TEXT PRIMARY KEY,
    agent_type  TEXT NOT NULL,
    name        TEXT NOT NULL,
    config      TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_permission_templates_agent ON permission_templates(agent_type);

ALTER TABLE tasks ADD COLUMN requirement_id TEXT REFERENCES requirements(id) ON DELETE SET NULL;
ALTER TABLE projects ADD COLUMN context_template TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN knowledge TEXT DEFAULT '';
`,
},
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/db/ -run TestMigrationV10 -v
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat(db): add migration v10 — requirements, planning_sessions, permission_templates"
```

---

### Task 1.2: Add model structs and repos for new tables

**Files:**
- Modify: `internal/db/models.go` — add Requirement, PlanningSession, PermissionTemplate structs
- Create: `internal/db/requirement_repo.go`
- Create: `internal/db/planning_session_repo.go`
- Create: `internal/db/permission_template_repo.go`
- Create: `internal/db/requirement_repo_test.go`
- Create: `internal/db/planning_session_repo_test.go`
- Create: `internal/db/permission_template_repo_test.go`

**Step 1: Add model structs to `models.go`**

```go
type Requirement struct {
    ID          string    `json:"id"`
    ProjectID   string    `json:"project_id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Priority    int       `json:"priority"`
    Status      string    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type PlanningSession struct {
    ID             string    `json:"id"`
    RequirementID  string    `json:"requirement_id"`
    AgentSessionID string    `json:"agent_session_id,omitempty"`
    Status         string    `json:"status"`
    Blueprint      string    `json:"blueprint,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

type PermissionTemplate struct {
    ID        string    `json:"id"`
    AgentType string    `json:"agent_type"`
    Name      string    `json:"name"`
    Config    string    `json:"config"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type RequirementFilter struct {
    ProjectID string
    Status    string
}
```

**Step 2: Write RequirementRepo with TDD**

Test file `requirement_repo_test.go` — test CRUD + ListByProject + Reorder.
Implementation `requirement_repo.go` — Create, Get, Update, Delete, ListByProject, Reorder.

Follow TDD: write test → verify fail → implement → verify pass for each method.

**Step 3: Write PlanningSessionRepo with TDD**

Test: Create, Get, Update, GetByRequirement.
Implementation: same methods.

**Step 4: Write PermissionTemplateRepo with TDD**

Test: Create, Get, Update, Delete, ListByAgent.
Implementation: same methods.

**Step 5: Commit**

```bash
git add -A && git commit -m "feat(db): add repos for requirements, planning_sessions, permission_templates"
```

---

### Task 1.3: Update Project model for new fields

**Files:**
- Modify: `internal/db/models.go` — add `ContextTemplate` and `Knowledge` to `Project`
- Modify: `internal/db/project_repo.go` — update Create/Get/Update queries to include new columns
- Modify: existing project tests if needed

**Step 1: Add fields to Project struct**

```go
type Project struct {
    // ... existing fields ...
    ContextTemplate string `json:"context_template,omitempty"`
    Knowledge       string `json:"knowledge,omitempty"`
}
```

**Step 2: Update repo queries — add columns to SELECT/INSERT/UPDATE**

**Step 3: Run existing tests, fix any failures**

```bash
go test ./internal/db/ -run TestProject -v
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat(db): add context_template and knowledge fields to projects"
```

---

### Task 1.4: Update Task model for requirement_id

**Files:**
- Modify: `internal/db/models.go` — add `RequirementID` to `Task`
- Modify: `internal/db/task_repo.go` — update queries

**Step 1: Add field, update queries, run tests**

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(db): add requirement_id to tasks"
```

---

## Phase 2: New Backend Handlers

### Task 2.1: Add requirements API handlers

**Files:**
- Create: `internal/api/requirements.go`
- Create: `internal/api/requirements_test.go`
- Modify: `internal/api/router.go` — add requirement routes

**Step 1: Write failing tests for each endpoint**

Test: `POST /api/projects/{id}/requirements` — creates requirement
Test: `GET /api/projects/{id}/requirements` — lists by project, ordered by priority
Test: `GET /api/requirements/{id}` — get single
Test: `PATCH /api/requirements/{id}` — update title/description/status
Test: `DELETE /api/requirements/{id}` — delete
Test: `POST /api/projects/{id}/requirements/reorder` — reorder priorities

**Step 2: Implement handlers in `requirements.go`**

**Step 3: Register routes in `router.go`**

```go
mux.HandleFunc("POST /api/projects/{id}/requirements", h.createRequirement)
mux.HandleFunc("GET /api/projects/{id}/requirements", h.listRequirements)
mux.HandleFunc("GET /api/requirements/{id}", h.getRequirement)
mux.HandleFunc("PATCH /api/requirements/{id}", h.updateRequirement)
mux.HandleFunc("DELETE /api/requirements/{id}", h.deleteRequirement)
mux.HandleFunc("POST /api/projects/{id}/requirements/reorder", h.reorderRequirements)
```

**Step 4: Update handler struct — add `requirementRepo`**

**Step 5: Update `main.go` — instantiate RequirementRepo, pass to router**

**Step 6: Run all tests**

```bash
go test ./internal/api/ -run TestRequirement -v
```

**Step 7: Commit**

```bash
git add -A && git commit -m "feat(api): add requirements CRUD endpoints"
```

---

### Task 2.2: Add planning session API handlers

**Files:**
- Create: `internal/api/planning.go`
- Create: `internal/api/planning_test.go`
- Modify: `internal/api/router.go`

**Endpoints:**
- `POST /api/requirements/{id}/planning` — create planning session (spawns planner agent TUI session, links to requirement)
- `GET /api/requirements/{id}/planning` — get planning session for requirement
- `PATCH /api/planning-sessions/{id}` — update status, save blueprint
- `POST /api/planning-sessions/{id}/blueprint` — save/update blueprint JSON

**Step 1: Write failing tests for each endpoint**

The `POST` handler should:
1. Find the requirement
2. Find a suitable planner agent from registry (role includes "plan")
3. Call `session.Manager.CreateSession()` with the project's repo as working dir
4. Create PlanningSession row linking requirement → agent session
5. Update requirement status to `planning`

**Step 2: Implement, register routes, verify**

**Step 3: Commit**

```bash
git add -A && git commit -m "feat(api): add planning session endpoints"
```

---

### Task 2.3: Create scaffold package

**Files:**
- Create: `internal/scaffold/scaffold.go`
- Create: `internal/scaffold/scaffold_test.go`
- Create: `internal/scaffold/claudemd.go`
- Create: `internal/scaffold/permissions.go`

**Step 1: Define the interface**

```go
package scaffold

type Blueprint struct {
    Tasks []BlueprintTask `json:"tasks"`
}

type BlueprintTask struct {
    ID              string   `json:"id"`
    Title           string   `json:"title"`
    Description     string   `json:"description"`
    CompletionCriteria []string `json:"completion_criteria"`
    WorktreeBranch  string   `json:"worktree_branch"`
    AgentType       string   `json:"agent_type"`
    DependsOn       []string `json:"depends_on,omitempty"`
}

type SetupResult struct {
    TaskID     string
    WorktreeID string
    SessionID  string
    BranchName string
    Path       string
}

// Setup creates worktrees, writes CLAUDE.md/AGENTS.md, writes permission configs,
// and spawns agent sessions for each task in the blueprint.
func (s *Scaffolder) Setup(ctx context.Context, projectID string, blueprint Blueprint) ([]SetupResult, error)

// Teardown removes worktrees and branches for completed tasks.
func (s *Scaffolder) Teardown(ctx context.Context, results []SetupResult) error
```

**Step 2: Write tests for CLAUDE.md generation (`claudemd.go`)**

Test that `GenerateContextFile` produces correct content given project template + task spec + completion criteria + behavioral rules.

**Step 3: Implement `claudemd.go`**

```go
func GenerateContextFile(agentType string, project *db.Project, task BlueprintTask) string
```

- For Claude Code: generates `CLAUDE.md`
- For Codex/OpenCode/Kimi: generates `AGENTS.md`
- Content: project context template + task description + completion criteria + standard rules

**Step 4: Write tests for permission config writing (`permissions.go`)**

Test that `WritePermissionConfig` creates the correct file in the correct location per agent type.

**Step 5: Implement `permissions.go`**

```go
func WritePermissionConfig(worktreePath string, agentType string, template *db.PermissionTemplate) error
```

- Claude Code: `.claude/settings.json`
- Codex: `.codex/rules/default.rules`
- OpenCode: `opencode.json`

**Step 6: Write integration tests for `Setup()`**

Test the full flow: creates worktrees, writes context files, writes permission configs.
(Session spawning can be mocked via the `TerminalBackend` interface)

**Step 7: Implement `Setup()` and `Teardown()`**

**Step 8: Commit**

```bash
git add -A && git commit -m "feat(scaffold): add worktree setup, CLAUDE.md generation, permission config writing"
```

---

### Task 2.4: Add execution API handlers

**Files:**
- Create: `internal/api/execution.go`
- Create: `internal/api/execution_test.go`
- Modify: `internal/api/router.go`

**Endpoints:**
- `POST /api/requirements/{id}/launch` — one-click execution setup from blueprint
- `POST /api/requirements/{id}/transition` — stage transition (build→review, review→merge, merge→test, test→done)

**Step 1: Write failing tests**

The `POST /launch` handler should:
1. Load planning session + blueprint for the requirement
2. Call `scaffold.Setup()` to create worktrees + sessions
3. Create an execution run + stage run for "build"
4. Update requirement status to `building`
5. Return the list of created sessions/worktrees

The `POST /transition` handler should:
1. Validate the transition is legal (build→review, review→merge, etc.)
2. For build→review: spawn reviewer agents on completed worktrees
3. For review→merge: trigger git merge operations
4. For merge→test: update status, human takes over
5. For test→done: mark requirement done, call scaffold.Teardown()

**Step 2: Implement, register routes**

**Step 3: Commit**

```bash
git add -A && git commit -m "feat(api): add execution launch and stage transition endpoints"
```

---

### Task 2.5: Add agent capacity endpoint

**Files:**
- Modify: `internal/api/agents.go` — the `listAgentStatuses` handler already exists, verify it returns capacity info

**Step 1: Check existing `GET /api/agents/status`**

Verify it returns: agent type, max_parallel, active session count. If not, add the active count by querying sessions table.

**Step 2: Commit if changes needed**

```bash
git add -A && git commit -m "feat(api): add active session count to agent status endpoint"
```

---

### Task 2.6: Add onboarding/permission template endpoints

**Files:**
- Create: `internal/api/onboarding.go`
- Modify: `internal/api/router.go`

**Endpoints:**
- `GET /api/permission-templates` — list all templates
- `GET /api/permission-templates/{agent_type}` — list templates for an agent type
- `POST /api/permission-templates` — create template
- `PUT /api/permission-templates/{id}` — update template
- `DELETE /api/permission-templates/{id}` — delete template

**Step 1: TDD — write tests, implement, register routes**

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(api): add permission template CRUD endpoints"
```

---

### Task 2.7: Simplify run_state.go

**Files:**
- Modify: `internal/api/run_state.go` — remove workflow CRUD handlers, keep run state handlers

**Step 1: Remove workflow handlers**

Remove `listWorkflows`, `createWorkflow`, `updateWorkflow`, `deleteWorkflow` and their route registrations.

**Step 2: Verify remaining run state endpoints work**

```bash
go test ./internal/api/ -run TestRun -v
```

**Step 3: Commit**

```bash
git add -A && git commit -m "refactor(api): simplify run_state — remove workflow CRUD"
```

---

### Task 2.8: Add event detection to parser

**Files:**
- Modify: `internal/parser/parser.go` — add detection for `[READY_FOR_REVIEW]` and `[BLOCKED]` signals
- Modify: `internal/parser/parser_test.go`

**Step 1: Write failing tests**

Test that parser detects `[READY_FOR_REVIEW]` in text and classifies it as a "review_ready" event.
Test that parser detects `[BLOCKED]` in text and classifies it as a "blocked" event.
Test that parser detects BLOCKED.md file creation in git output.

**Step 2: Implement detection**

Add event classification alongside existing output classification. Events are surfaced via the hub broadcast so the frontend can show alerts.

**Step 3: Commit**

```bash
git add -A && git commit -m "feat(parser): detect READY_FOR_REVIEW and BLOCKED signals in output"
```

---

## Phase 3: Frontend Rewrite

### Task 3.1: Set up Tailwind CSS

**Files:**
- Modify: `frontend/package.json` — add tailwindcss, postcss, autoprefixer
- Create: `frontend/tailwind.config.js`
- Create: `frontend/postcss.config.js`
- Modify: `frontend/src/styles/workspace.css` — replace with Tailwind directives (keep as entry point)
- Modify: `frontend/vite.config.ts` — ensure PostCSS is configured

**Step 1: Install Tailwind**

```bash
cd frontend && npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
```

**Step 2: Configure `tailwind.config.js`**

```js
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        // Keep agenterm's dark terminal aesthetic
        'bg-primary': '#0a0a0f',
        'bg-secondary': '#12121a',
        'bg-tertiary': '#1a1a28',
        'border': '#2a2a3a',
        'text-primary': '#e0e0e0',
        'text-secondary': '#888',
        'accent': '#7c5cff',
        'status-working': '#4caf50',
        'status-waiting': '#ff9800',
        'status-error': '#f44336',
        'status-idle': '#666',
      }
    }
  },
  plugins: [],
}
```

**Step 3: Replace `workspace.css` content with Tailwind directives**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

/* Keep any truly global styles that can't be done with Tailwind utilities */
```

**Step 4: Verify build works**

```bash
cd frontend && npm run build
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat(frontend): add Tailwind CSS, remove monolithic stylesheet"
```

---

### Task 3.2: Rewrite App.tsx and create new routing

**Files:**
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/api/client.ts` — update/add API functions for new endpoints

**Step 1: Rewrite App.tsx**

Keep: `useWebSocket` hook, `AppContext` pattern, token auth.
Remove: `MobileCompanion` routing, window-centric state.
Add: Project-centric state, mode switching (workspace/demands/settings).

**Step 2: Add new API client functions**

```typescript
// Requirements
export function listRequirements(projectID: string) { ... }
export function createRequirement(projectID: string, data: {...}) { ... }
export function updateRequirement(id: string, data: {...}) { ... }
export function deleteRequirement(id: string) { ... }
export function reorderRequirements(projectID: string, ids: string[]) { ... }

// Planning
export function createPlanningSession(requirementID: string) { ... }
export function getPlanningSession(requirementID: string) { ... }
export function saveBlueprint(planningSessionID: string, blueprint: {...}) { ... }

// Execution
export function launchExecution(requirementID: string) { ... }
export function transitionStage(requirementID: string, transition: string) { ... }

// Permission templates
export function listPermissionTemplates(agentType?: string) { ... }
export function createPermissionTemplate(data: {...}) { ... }
export function updatePermissionTemplate(id: string, data: {...}) { ... }

// Agent capacity
export function getAgentStatuses() { ... }
```

**Step 3: Commit**

```bash
git add -A && git commit -m "feat(frontend): rewrite App.tsx routing, add new API client functions"
```

---

### Task 3.3: Build AppSidebar component

**Files:**
- Create: `frontend/src/components/AppSidebar.tsx`

**Step 1: Implement**

Foldable sidebar with three sections:
- **Projects** (top): List project names with session count badges and alert indicators. "+ New Project" button.
- **Agents** (bottom): Agent capacity display (e.g. "claude 1/2 working"). Hover shows which sessions.
- **Settings** (footer): Settings gear icon.

Uses Tailwind classes matching agenterm's dark aesthetic.

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add AppSidebar — projects list and agent capacity"
```

---

### Task 3.4: Build NewProjectModal component

**Files:**
- Create: `frontend/src/components/NewProjectModal.tsx`

**Step 1: Implement**

Modal with:
- Folder picker (uses existing `GET /api/fs/directories`)
- Project name input
- "Create" button → `POST /api/projects`

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add NewProjectModal — folder picker and name input"
```

---

### Task 3.5: Build DemandPool component

**Files:**
- Create: `frontend/src/components/DemandPool.tsx`

**Step 1: Implement**

- Input frame at top to add new demands
- Table below: all demands, drag to reorder, status badges
- Active demands: click → switch to workspace mode
- Completed demands: expandable → shows status graph + linked sessions

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add DemandPool — requirement queue with reorder and status"
```

---

### Task 3.6: Build StatusGraph component

**Files:**
- Create: `frontend/src/components/StatusGraph.tsx`

**Step 1: Implement**

Pipeline visualization: `[Plan] → [Build] → [Review] → [Merge] → [Test]`
- Current stage highlighted
- Under build stage: per-worktree status nodes (done/running/blocked)
- Nodes are clickable → emits navigation event to jump to agent terminal

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add StatusGraph — pipeline visualization with clickable nodes"
```

---

### Task 3.7: Build AgentSidebar component

**Files:**
- Create: `frontend/src/components/AgentSidebar.tsx`

**Step 1: Implement**

Left sidebar within workspace:
- Planner always on top
- Builders and reviewers grouped by worktree
- Status indicators: 🟢 working, 🔴 needs response, ⚫ idle
- Click to select → updates main content area

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add AgentSidebar — agent list grouped by worktree"
```

---

### Task 3.8: Build AgentView component (TUI/MD/Split)

**Files:**
- Create: `frontend/src/components/AgentView.tsx`
- Create: `frontend/src/components/MarkdownPane.tsx`

**Step 1: Implement AgentView**

Three-mode toggle: `[TUI] [MD] [Split]`
- **TUI mode:** Full xterm.js terminal (reuse existing `Terminal.tsx` component)
- **MD mode:** `MarkdownPane` component
- **Split mode:** TUI left, MarkdownPane right

**Step 2: Implement MarkdownPane**

- File tree showing only `.md` files in agent's worktree
- Uses `GET /api/fs/directories` scoped to worktree path (may need endpoint extension for file listing)
- Click file → renders content with edit support
- Save → writes back via new endpoint or Tauri file API

**Step 3: Commit**

```bash
git add -A && git commit -m "feat(frontend): add AgentView with TUI/MD/Split modes and MarkdownPane"
```

---

### Task 3.9: Build Workspace component

**Files:**
- Create: `frontend/src/components/WorkspaceView.tsx`

**Step 1: Implement**

Assembles the workspace layout:
- Left: `AgentSidebar`
- Top: `StatusGraph`
- Center: `AgentView` for selected agent

**Empty state** (new project, no demands): Centered input — "What do you want to build?" → creates first demand, starts planning flow.

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add WorkspaceView — agent sidebar + status graph + agent view"
```

---

### Task 3.10: Build ExecutionSetup component

**Files:**
- Create: `frontend/src/components/ExecutionSetup.tsx`

**Step 1: Implement**

Shown after planning session completes and blueprint is saved:
- Task list from blueprint
- Agent assignment dropdown per task (shows capacity)
- "Launch All" button → `POST /api/requirements/{id}/launch`
- Progress feedback during setup

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add ExecutionSetup — blueprint review and one-click launch"
```

---

### Task 3.11: Build StageControls component

**Files:**
- Create: `frontend/src/components/StageControls.tsx`

**Step 1: Implement**

Stage transition buttons displayed based on current stage:
- Build stage: "Start Review" (when all worktrees done or manually triggered)
- Review stage: "Merge All"
- Merge stage: "I'll Test Now"
- Test stage: "Mark Done"

Each button → `POST /api/requirements/{id}/transition`

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add StageControls — human-triggered stage transitions"
```

---

### Task 3.12: Build ProjectSettings component

**Files:**
- Create: `frontend/src/components/ProjectSettings.tsx`

**Step 1: Implement**

- Project name editor
- Context template editor (CLAUDE.md/AGENTS.md template textarea)
- Project knowledge viewer (read-only, from `/init`)
- Session history viewer (all sessions for this project)

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add ProjectSettings — name, context template, knowledge"
```

---

### Task 3.13: Build OnboardingWizard component

**Files:**
- Create: `frontend/src/components/OnboardingWizard.tsx`

**Step 1: Implement per `docs/2026-03-04-onboarding-flow.md`**

Full-screen modal, 3 steps:
1. **Set Up Your AI Team** — checklist of supported agents with defaults (command, parallel slots, roles). All fields editable.
2. **Permission Templates** — standard/strict/permissive per agent. Preview permission config. Editable.
3. **Done** — summary of what was set up. "Get Started" button.

On completion: saves agent configs via `POST /api/agents`, saves permission templates via `POST /api/permission-templates`. Shows on first launch only (check if any agents exist).

**Step 2: Commit**

```bash
git add -A && git commit -m "feat(frontend): add OnboardingWizard — 3-step first-run setup"
```

---

### Task 3.14: Build updated SettingsModal

**Files:**
- Modify: `frontend/src/components/SettingsModal.tsx`

**Step 1: Simplify**

Keep: Agent registry (add/edit/remove agents)
Add: Permission template management
Remove: ASR settings, appearance settings (orchestrator-specific), mobile pairing

**Step 2: Commit**

```bash
git add -A && git commit -m "refactor(frontend): simplify SettingsModal — agents and permission templates only"
```

---

### Task 3.15: Delete unused frontend components

**Files:**
- Delete: `frontend/src/components/OrchestratorPanel.tsx`
- Delete: `frontend/src/components/ChatPanel.tsx`
- Delete: `frontend/src/components/ChatMessage.tsx`
- Delete: `frontend/src/components/ConnectModal.tsx`
- Delete: `frontend/src/pages/MobileCompanion.tsx`
- Delete: `frontend/src/hooks/useOrchestratorWS.ts`
- Delete: `frontend/src/orchestrator/` (entire directory)

**Step 1: Delete files, remove imports from remaining components**

**Step 2: Verify build**

```bash
cd frontend && npm run build
```

**Step 3: Commit**

```bash
git add -A && git commit -m "refactor(frontend): remove orchestrator chat, mobile companion, and related components"
```

---

## Phase 4: Integration & Polish

### Task 4.1: Update main.go for v2 wiring

**Files:**
- Modify: `cmd/agenterm/main.go`

**Step 1: Clean up main.go**

Should now wire only:
- Config → DB → agent registry → PTY backend → hub → session manager → scaffold
- All repos: project, task, worktree, session, sessionCommand, requirement, planningSession, permissionTemplate, knowledge, review, run, demandPool
- API router with all v2 handlers
- Server start

No orchestrator, no automation, no playbook.

**Step 2: Verify full app starts**

```bash
go build -o agenterm ./cmd/agenterm && ./agenterm
```

**Step 3: Commit**

```bash
git add -A && git commit -m "refactor: rewire main.go for v2 — remove orchestrator/automation, add scaffold"
```

---

### Task 4.2: Update AGENTS.md for v2

**Files:**
- Modify: `AGENTS.md`

**Step 1: Rewrite to reflect v2 architecture**

Update platform summary, core capabilities, architecture pointers, and operating rules to match the human orchestrator model.

**Step 2: Commit**

```bash
git add -A && git commit -m "docs: update AGENTS.md for v2 human orchestrator model"
```

---

### Task 4.3: End-to-end smoke test

**Files:** None created

**Step 1: Manual verification checklist**

1. App launches, onboarding wizard appears
2. Set up agents (at least one)
3. Set permission templates
4. Create a project
5. Add a demand
6. Start planning session → planner agent TUI opens
7. Save blueprint
8. Launch execution → worktrees created, agents spawned
9. Monitor agents in workspace view
10. Trigger stage transitions
11. Mark done → cleanup

**Step 2: Fix any issues found**

**Step 3: Final commit**

```bash
git add -A && git commit -m "fix: address issues from end-to-end smoke test"
```

---

## Task Dependency Graph

```
Phase 0 (sequential):
  0.1 → 0.2 → 0.3 → 0.4 → 0.5 → 0.6 → 0.7 → 0.8

Phase 1 (sequential, depends on Phase 0):
  1.1 → 1.2 → 1.3 → 1.4

Phase 2 (partially parallel, depends on Phase 1):
  2.1 (requirements API)     ─┐
  2.2 (planning API)          ├→ 2.4 (execution API, depends on 2.1, 2.2, 2.3)
  2.3 (scaffold package)     ─┘
  2.5 (agent capacity)       — independent
  2.6 (permission templates) — independent
  2.7 (simplify run_state)   — independent
  2.8 (parser events)        — independent

Phase 3 (partially parallel, depends on Phase 2):
  3.1 (Tailwind setup)      → all other 3.x tasks
  3.2 (App.tsx + API client) → all other 3.x tasks
  3.3 (AppSidebar)           — independent after 3.1, 3.2
  3.4 (NewProjectModal)      — independent
  3.5 (DemandPool)           — independent
  3.6 (StatusGraph)          — independent
  3.7 (AgentSidebar)         — independent
  3.8 (AgentView + MD pane)  — independent
  3.9 (WorkspaceView)        → depends on 3.6, 3.7, 3.8
  3.10 (ExecutionSetup)      — independent
  3.11 (StageControls)       — independent
  3.12 (ProjectSettings)     — independent
  3.13 (OnboardingWizard)    — independent
  3.14 (SettingsModal)       — independent
  3.15 (Delete old components) — after all 3.x complete

Phase 4 (sequential, depends on all above):
  4.1 → 4.2 → 4.3
```
