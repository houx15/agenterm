# Plan: Playbook Role Contracts for Orchestrator

## Objective

Move playbooks from prompt-only guidance to enforceable role contracts, so orchestrator behavior is predictable across planning, coding, review loops, and finalize gates.

## Problems to Solve

1. Roles currently rely too much on `suggested_prompt`, which is advisory and not enforceable.
2. Review/fix/re-review loops are not encoded explicitly as machine-readable policy.
3. Users do not have clear guidance in UI/README on how to configure robust playbooks.

## Target Outcomes

1. Each role has a contract: required inputs, allowed actions, required outputs, and exit gates.
2. Orchestrator enforces role policy before/after tool calls.
3. UI guides users through role configuration with templates and validation.
4. README teaches practical playbook authoring patterns (`tdd`, `pairing`, `compound engineering`).

## Proposed Playbook Schema Changes

## 1) Role-level contract

Extend each stage role with:

- `mode`: `planner | worker | reviewer | tester`
- `inputs_required`: string[]
- `outputs_contract`:
  - `type`: string (e.g. `review_result`, `work_result`)
  - `required`: string[] (required output fields)
- `actions_allowed`: string[] (tool names)
- `gates`:
  - `requires_user_approval`: bool
  - `pass_condition`: string (expression/policy key)
- `handoff_to`: string[] (next roles)
- `retry_policy`:
  - `max_iterations`: int
  - `escalate_on`: string[]
- `completion_criteria`: string[]

Keep existing fields:

- `name`
- `responsibilities`
- `allowed_agents`
- `suggested_prompt` (optional)

## 2) Stage-level policy

Extend stage with:

- `stage_policy.enter_gate`
- `stage_policy.exit_gate`
- `stage_policy.max_parallel_worktrees`

## Prompt Changes

Add a contract-first execution policy block in system prompt:

1. Read role contract before acting.
2. Verify `inputs_required`; if missing, ask/fetch first.
3. Respect approval and action constraints.
4. Return structured output matching `outputs_contract`.
5. Evaluate `gates`; if failed, do not advance stage.
6. Follow `handoff_to` and `retry_policy`.

Add explicit loop guidance:

- Reviewer verdict not pass -> handoff back to worker with actionable issues.
- Continue until pass or retry limit reached; then escalate.

## Orchestrator Enforcement Plan

## 1) Pre-execution checks

- Validate current role has required inputs.
- Validate requested tool is in `actions_allowed`.
- Validate approval requirement before mutating tools.

## 2) Post-execution checks

- Validate assistant output structure against `outputs_contract`.
- Evaluate gate conditions (pass/fail).
- Decide next role/stage via `handoff_to` and stage policies.

## 3) Review loop implementation

- Track iteration count per task/worktree/role.
- Route `worker -> reviewer -> worker` until pass.
- Escalate when `retry_policy` triggers.

## UI Plan (Settings > Playbooks)

## 1) Role editor sections

- Intent: `mode`, responsibilities, allowed agents
- Inputs: `inputs_required`
- Execution: `actions_allowed`
- Output: `outputs_contract`
- Control: `gates`, `handoff_to`, `retry_policy`, `completion_criteria`

## 2) Templates

Add quick role templates:

- Planner template
- Worker template
- Reviewer template
- Tester template

## 3) Validation hints

Inline warnings for common misconfigurations:

- Reviewer missing `spec_path` or `commit_sha` in inputs
- Stage enabled with no roles
- Role with no `actions_allowed`
- Role with outputs contract but missing required fields

## README Plan

Add sections:

1. "Role Contracts"
  - Explain each contract field
  - Include one complete reviewer example
2. "Execution Patterns"
  - Pairing coding
  - TDD
  - Compound engineering review loop
3. "Common Mistakes"
  - Prompt-only roles
  - Missing gates
  - Missing handoffs/retry policy

## Rollout Slices

## Slice 1 (Schema + Backward Compatibility)

- Add new schema fields to backend/frontend types.
- Keep old playbooks valid (default empty contract values).
- Add validators for minimum safe defaults.

## Slice 2 (Prompt + Enforcement Core)

- Inject contract block into system prompt.
- Enforce `actions_allowed`, `inputs_required`, approval.
- Add contract-aware error messages in tool results.

## Slice 3 (Review Loop Engine)

- Add iteration tracking and role handoff logic.
- Enforce retry limits and escalation paths.
- Surface loop state in progress report.

## Slice 4 (UI)

- Build role contract editor panels.
- Add templates and validation hints.
- Improve discoverability with compact helper text.

## Slice 5 (README + Examples)

- Publish docs and examples.
- Update default playbooks to contract-rich versions.

## Acceptance Criteria

1. Orchestrator can run `plan -> build -> test` with explicit role contracts.
2. Review loop works without manual orchestration glue:
   - coder commit -> reviewer verdict -> fix loop -> pass.
3. Invalid role configs are blocked by UI/backend validation.
4. Users can configure a practical workflow without reading source code.
