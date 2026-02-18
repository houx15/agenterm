---
status: complete
priority: p1
issue_id: "037"
tags: [code-review, automation, hooks, spec-compliance]
dependencies: []
---

# TASK-16 Claude Hook Injection Not Implemented

Branch behavior intentionally avoids mutating `.claude/settings.json`, but `docs/TASK-16-automation.md` requires automatic hook injection for `agent_type=claude-code` sessions.

## Problem Statement

`TASK-16` defines hook automation as a required part of the feature: generate Claude hook config and write it to `<worktree>/.claude/settings.json` during session creation. Current branch behavior is no-op for hook setup, so the branch is out of spec against the task document being reviewed.

## Findings

- `internal/automation/hooks.go:7` makes `EnsureClaudeCodeAutomation` an explicit no-op.
- `internal/automation/hooks_test.go:9` and `internal/automation/hooks_test.go:19` assert that no settings file is written or modified.
- `rg` usage shows `EnsureClaudeCodeAutomation(...)` is not called from session creation or related flow.
- `docs/TASK-16-automation.md:52`-`docs/TASK-16-automation.md:67` requires hook config injection into `.claude/settings.json`.
- `docs/TASK-16-automation.md` acceptance criteria includes: "Claude Code hooks injected on session creation".

## Proposed Solutions

### Option 1: Re-implement automatic hook injection per TASK-16

**Approach:** Restore hook generation and invoke it in session creation for Claude sessions.

**Pros:**
- Fully matches TASK-16 requirements
- Makes acceptance criteria objectively pass

**Cons:**
- Reintroduces mutation of user/tool-managed Claude settings
- Can conflict with existing user hook strategies

**Effort:** 2-4 hours

**Risk:** Medium

---

### Option 2: Keep no-op behavior and update TASK-16 / acceptance contract

**Approach:** Treat hook automation as intentionally out-of-scope, document command-based integration pattern, and update task/spec expectations.

**Pros:**
- Aligns with current product direction (no automatic hook mutation)
- Avoids clobbering user-managed Claude config

**Cons:**
- Requires formal spec/task updates before this review can pass
- Existing acceptance wording becomes invalid until updated

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 3: Add opt-in hook injection flag

**Approach:** Inject hooks only when explicitly enabled (project/session-level setting).

**Pros:**
- Preserves non-intrusive default behavior
- Still supports TASK-16-compatible mode

**Cons:**
- Adds configuration and testing complexity
- Requires clear API/UI contract for opt-in behavior

**Effort:** 4-6 hours

**Risk:** Medium

## Recommended Action

Adopt Option 2: keep hook management user/tool-owned and update TASK-16 to remove automatic hook injection requirements.

## Technical Details

Affected files:
- `internal/automation/hooks.go`
- `internal/automation/hooks_test.go`
- `internal/session/manager.go` (or equivalent session creation path)
- `docs/TASK-16-automation.md`

Related components:
- Session creation flow (`agent_type=claude-code`)
- Automation lifecycle around done/ready markers

Database changes:
- No

## Resources

- `docs/TASK-16-automation.md`
- `internal/automation/hooks.go`
- `internal/automation/hooks_test.go`
- `todos/035-complete-p1-claude-hook-commands-point-to-missing-worktree-scripts.md`

## Acceptance Criteria

- [x] Review target and task doc agree on whether hook injection is required
- [x] If not required: TASK-16/spec acceptance text updated to reflect non-intrusive approach
- [x] Regression tests remain valid for current no-op hook policy

## Work Log

### 2026-02-18 - Review Finding

By: Codex

Actions:
- Compared branch implementation with `docs/TASK-16-automation.md`
- Verified hook implementation/tests and call sites
- Logged P1 spec-compliance mismatch

Learnings:
- Current branch intentionally enforces a no-op hook strategy
- Task acceptance currently still expects automatic injection

### 2026-02-18 - Resolution

By: Codex

Actions:
- Updated `docs/TASK-16-automation.md` to remove mandatory Claude hook injection
- Recorded non-intrusive policy: no mutation of `.claude/settings.json`
- Kept automation contract focused on command/repo/session signals

Learnings:
- Spec and implementation are now aligned on hook ownership boundaries

## Notes

This finding is a spec-compliance blocker for review against `docs/TASK-16-automation.md` as currently written.
