---
status: complete
priority: p2
issue_id: "034"
tags: [code-review, security, reliability, automation]
dependencies: []
---

# Path Scope Check Fails Open When Session Workdir Cannot Be Resolved

When workdir resolution fails, command validation receives an empty root and skips path-boundary enforcement.

## Problem Statement

Workdir resolution is not guaranteed (missing task/project/worktree rows, transient DB errors). In these cases, the policy currently falls back to partial checks rather than deny-by-default, allowing commands with unrestricted path scope.

## Findings

- `validatePathScope` returns success when `allowedRoot` is empty (`internal/session/command_policy.go:149-150`).
- Manager command path uses resolved workdir directly without fail-closed guard (`internal/session/manager.go:253-254`).
- API fallback path does the same (`internal/api/sessions.go:248-249`).
- This creates inconsistent enforcement depending on repository lookup health.

## Proposed Solutions

### Option 1: Fail Closed on Missing Workdir

**Approach:** If command contains any path-like token and `allowedRoot` is empty, return a policy error.

**Pros:**
- Strong safety default
- Minimal code change

**Cons:**
- May block some command-only operations during partial outages

**Effort:** Small

**Risk:** Low

---

### Option 2: Hard Requirement Upstream

**Approach:** In `SendCommand` and API handler, reject command execution when session workdir cannot be resolved.

**Pros:**
- Clear operational behavior
- Avoids ambiguous policy state

**Cons:**
- More visible failures in inconsistent DB states

**Effort:** Small

**Risk:** Low

---

### Option 3: Temporary Safe Mode

Allow only no-path commands (`pwd`, `echo`, status checks) when root is unavailable.

## Recommended Action

To be filled during triage.

## Technical Details

Affected files:
- `internal/session/command_policy.go:147`
- `internal/session/manager.go:253`
- `internal/api/sessions.go:248`

## Resources

- Task spec context: `docs/TASK-16-automation.md`

## Acceptance Criteria

- [ ] Command execution fails safely when workdir cannot be resolved
- [ ] Tests cover missing task/project/worktree resolution scenarios
- [ ] Error surfaced to caller is explicit and actionable
- [ ] Audit log captures blocked attempts in fail-closed mode

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed workdir resolution and policy invocation paths
- Identified fail-open behavior when root is missing
- Documented options and criteria

**Learnings:**
- Current policy correctly enforces scope only when root is present

## Notes

- Important reliability/security gap; prioritize after P1 items.
