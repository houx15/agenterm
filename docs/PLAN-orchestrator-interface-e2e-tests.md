# Plan: Orchestrator Interface E2E Test Flow

## Goal

Build a code-level, repeatable test suite that verifies the orchestrator can execute the real project lifecycle through API/tool interfaces:

1. Preflight repository and run `git init` when needed.
2. Run planning with planner agent session, send prompts in TUI session, write specs, and read outputs.
3. Enter build stage, create worktrees, dispatch coder/reviewer sessions, send prompts, merge worktree, and expose agent occupancy.
4. Enter test stage, dispatch tester session, run test prompts, and return human follow-up guidance.

## Test Scope

### In Scope

1. Orchestrator runtime tool loop (`Chat`) with staged playbook contracts.
2. REST interface behaviors for projects/tasks/worktrees/sessions/agents status.
3. Real filesystem/git side effects in temp directories.
4. Event-level assertions: `tool_call`, `tool_result`, `token`, `done`, and no unexpected `error`.

### Out of Scope (for this slice set)

1. Real tmux process behavior (simulated via fake gateway).
2. Real external model behavior (simulated via deterministic fake LLM server).
3. Frontend rendering behavior (covered in frontend tests).

## Test Architecture

1. **API Server (httptest):**
   - Start `NewRouter(...)` with SQLite temp DB.
   - Use real API handlers for tools.
2. **Fake LLM Server (httptest):**
   - Deterministic multi-call script returns `tool_use` and final text.
   - Extract previous `tool_result` payloads to carry dynamic IDs across rounds.
3. **Orchestrator Runtime:**
   - Real `orchestrator.New(...)`.
   - Tool calls target the test API server.
4. **Fake Gateway:**
   - Captures `SendRaw` commands to verify prompt dispatch to sessions.
5. **Git/FS:**
   - Temp directories with local git init/commit where required.

## Slices

### Slice 1: Harness + Plan Preflight Flow

1. Create project with non-git repo path.
2. Trigger orchestrator plan run:
   - `create_session(requirements-analyst)`
   - `send_command(planner prompt)`
   - `read_session_output`
   - `write_task_spec`
3. Assertions:
   - `.git` exists in repo path after run.
   - spec file exists under `docs/...`.
   - at least one prompt sent via gateway (`SendRaw`).
   - no orchestrator error events.

### Slice 2: Build Stage Flow

1. Use initialized repo with initial commit.
2. Project status in build stage.
3. Trigger orchestrator build run:
   - `create_worktree`
   - `create_session(coder)` + `send_command`
   - `create_session(reviewer)` + `send_command`
   - `merge_worktree`
4. Assertions:
   - worktree record created and merged.
   - worktree path exists.
   - coder/reviewer prompts were sent.
   - `/api/agents/status` reports expected busy assignments.

### Slice 3: Test Stage Flow

1. Project status in test stage.
2. Trigger orchestrator test run:
   - `create_session(test-role)`
   - `send_command(test plan prompt)`
   - `read_session_output`
3. Assertions:
   - tester session created.
   - tester prompt sent.
   - assistant final text includes human follow-up guidance.
   - `/api/agents/status` reflects tester assignment.

### Slice 4: Guardrails and Failure Regression

1. Ensure stage-gated restrictions still apply while lifecycle tests run.
2. Ensure required-input hydration path does not block planner/coder creation in flow tests.
3. Ensure no silent execution:
   - `tool_result` errors surface as error events in stream.
   - tests fail if an expected tool call is missing.

## Acceptance Criteria

1. One test suite runs all slices in CI-style fashion.
2. All tests are deterministic and independent.
3. Failures clearly indicate which stage/tool contract broke.

## Test Command

```bash
GOCACHE=$(pwd)/.cache/go-build go test ./internal/api -run OrchestratorInterfaceFlow -count=1
```

