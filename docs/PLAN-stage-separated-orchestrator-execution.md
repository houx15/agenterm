# PLAN: Stage-Separated Orchestrator Execution

## Goal

Make orchestrator behavior strictly stage-driven (`plan` -> `build` -> `test`) and coordinator-only.

The orchestrator must:
1. operate via tmux/tui tools instead of self-performing coding;
2. use stage-specific command/tool allowlists;
3. request explicit confirmations at stage gates;
4. expose stage/command/confirmation status clearly in PM Chat UI.

## Target Workflow

### 1) Plan Stage

Expected behavior:
1. Start one planning session (tmux + planning TUI).
2. Ask planner to understand codebase and generate a feature plan that includes:
   - stages
   - per-stage parallelizable worktrees
   - suggested worktree names
   - spec documents under repo `docs/`
3. Orchestrator asks user to confirm plan.
4. On confirm, close planning session and transition to build stage.

Stage tool focus:
- `create_task`, `create_worktree`, `write_task_spec`, `create_session`, `wait_for_session_ready`, `send_command`, `read_session_output`, `is_session_idle`, `close_session`, read/status tools.

### 2) Build Stage

Expected behavior:
1. Use plan output (task graph + worktrees).
2. Allocate agents by role and parallel capacity.
3. For each worktree/role lane:
   - create session
   - enter correct directory/worktree
   - start TUI with proper command
   - dispatch role prompt
4. Run execution loop:
   - monitor status/output
   - respond automatically when deterministic
   - ask user for ambiguous decisions
5. Review loop for each edit:
   - commit/push request
   - reviewer audit
   - fix -> review until pass
6. Merge finished worktrees and mark done.
7. Move to next stage when current stage worktrees complete.

Stage tool focus:
- session lifecycle + command dispatch + merge/conflict tools + review/status tools.

### 3) Test Stage

Expected behavior:
1. Verify all required changes are committed/pushed.
2. Start testing TUI session.
3. Generate test plan mapped to specs.
4. Execute automatable tests and summarize manual next steps.

Stage tool focus:
- status/read tools + session command tools; no planning-heavy mutations.

## Design Changes

### A. Stage Runtime Derivation

Add deterministic stage resolution in orchestrator runtime:
- `plan`: no actionable task/worktree graph yet;
- `build`: tasks/worktrees in progress;
- `test`: implementation done, finishing validation/checklist.

(Initial implementation uses current project/task/worktree/session state as source of truth.)

### B. Stage Tool Allowlist

Add a stage-level allowlist independent of role templates.
A tool must satisfy both:
1. role contract (`actions_allowed` / defaults)
2. stage allowlist

If blocked, return structured reason: `stage_tool_not_allowed`.

### C. Stage-Aware Prompt Contract

Extend system prompt with:
1. current active stage;
2. stage objective and completion target;
3. stage-specific required operating behavior;
4. reminder that orchestrator is coordinator-only and must use tools.

### D. Confirmation Gates

Maintain explicit confirmation before mutating actions and stage transitions.
UI should render confirmation as actionable choices (not raw error payloads).

### E. PM Chat Status Icons

Add explicit visual markers in chat stream:
1. discussion
2. command
3. confirmation needed

This makes orchestrator behavior legible and auditable.

## Implementation Slices

### Slice 1: Stage Runtime + Tool Gate (Backend)

1. Add runtime stage derivation helper.
2. Add stage tool allowlists.
3. Enforce allowlist in orchestrator execution path.
4. Return clear structured block errors.

Deliverables:
- `internal/orchestrator/orchestrator.go`
- tests for stage gate behavior.

### Slice 2: Stage-Aware Prompting (Backend)

1. Inject current stage and stage contract into prompt.
2. Add stage-specific planning/build/test operating instructions.
3. Keep coordinator-only rules strict.

Deliverables:
- `internal/orchestrator/prompt.go`
- prompt tests.

### Slice 3: PM Chat Status Icons (Frontend)

1. Render message badges/icons for:
   - discussion
   - command/tool
   - confirmation
2. Keep current stream semantics (`token`, `tool_call`, `tool_result`, `error`).

Deliverables:
- `frontend/src/components/ChatMessage.tsx`
- related styles.

### Slice 4: Guardrail/UX Tightening

1. Ensure approval-required outputs are surfaced as confirmation UI (already implemented).
2. Keep role action allowlists in settings as checkboxes (already implemented).
3. Validate no timer-based auto reports (already removed).

## Acceptance Criteria

1. Orchestrator cannot execute stage-incompatible tools.
2. Prompt clearly communicates stage and expected behavior.
3. PM chat visually distinguishes discussion/command/confirmation.
4. Execution remains tool-driven and coordinator-only.
5. Existing orchestrator test suite passes.

## Out of Scope (this iteration)

1. full persisted stage-state machine table/migration;
2. workflow compiler that auto-converts planner artifacts into DB-native stage graph;
3. automatic role-specific dispatch planner UI.

These can be added after stage gate correctness is stable.
