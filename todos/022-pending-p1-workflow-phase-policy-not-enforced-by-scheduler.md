---
status: pending
priority: p1
issue_id: "022"
tags: [code-review, architecture, orchestrator, governance]
dependencies: []
---

# Enforce Workflow Phase Policy In Scheduling

Scheduler admissions ignore workflow phase definitions (`phase_type`, `role`, `max_parallel`, entry/exit gates), so configured workflow/style does not actually govern orchestration.

## Problem Statement

The governance plan defines workflow phases as executable policy, but current scheduling only checks global/project/role-binding/agent limits. This allows orchestration decisions that violate phase-level policy and undermines workflow/style as a first-class control surface.

## Findings

- `internal/orchestrator/scheduler.go:16` performs admission checks without loading workflow phases.
- `internal/orchestrator/scheduler.go:130` resolves model and role binding only; no phase lookup by project workflow.
- `internal/orchestrator/orchestrator.go:346` loads workflow as prompt/playbook context only, not as enforcement.
- `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md` requires constraint-aware enforcement of workflow policy, including phase max parallel.

## Proposed Solutions

### Option 1: Add phase-aware admission checks in scheduler

Pros: Minimal architectural churn; immediate policy enforcement.
Cons: Scheduler grows in complexity.
Effort: Medium
Risk: Medium

### Option 2: Introduce dedicated governance validator before tool execution

Pros: Clear separation of policy and execution.
Cons: Larger refactor touching orchestrator tool loop.
Effort: Large
Risk: Medium

## Recommended Action


## Technical Details

Affected files:
- `internal/orchestrator/scheduler.go`
- `internal/orchestrator/orchestrator.go`
- `internal/api/orchestrator_governance.go`
- `internal/orchestrator/scheduler_test.go`

Database changes:
- No

## Resources

- `docs/TASK-15-orchestrator.md`
- `docs/PLAN-v2-orchestrator.md`
- `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md`

## Acceptance Criteria

- [ ] Session admission enforces active workflow phase constraints.
- [ ] Phase `max_parallel` and role/phase compatibility are validated.
- [ ] Violations return deterministic rejection reasons.
- [ ] Unit tests cover allow/deny cases for phase policy.

## Work Log

### 2026-02-17 - Review Discovery

By: Codex

Actions:
- Reviewed scheduler, prompt, and workflow-loading paths.
- Verified workflow policy is prompt-only and not enforced in admissions.

Learnings:
- Existing limits are enforced, but workflow phase governance is not yet binding.

## Notes

- Keep enforcement deterministic and explainable to preserve operator trust.
