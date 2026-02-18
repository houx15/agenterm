---
status: complete
priority: p1
issue_id: "035"
tags: [code-review, automation, reliability]
dependencies: []
---

# Claude Hook Commands Reference Scripts Not Present In Managed Worktrees

The hook commands injected into `.claude/settings.json` point to `scripts/*.sh` in the target worktree, but managed project worktrees do not contain those scripts by default.

## Problem Statement

Task-16 requires hook automation to run in each coder worktree. Current settings inject `bash scripts/auto-commit.sh` and `bash scripts/on-agent-stop.sh`, but those files are not provisioned into arbitrary project repos/worktrees. The result is silent hook no-op at runtime, so auto-commit and done/ready signals do not execute when Claude hook events fire.

## Findings

- Hook commands are hardcoded to relative repo paths: `internal/automation/hooks.go:27`, `internal/automation/hooks.go:28`.
- Hook setup only writes `.claude/settings.json`; it does not install hook scripts into the worktree: `internal/automation/hooks.go:39`.
- Spec expects executable hook commands in each worktree context (`docs/TASK-16-automation.md`, Step 2 + files list).

## Proposed Solutions

### Option 1: Inline Shell Commands In Hook Entries

**Approach:** Replace script paths with self-contained command strings (git status/add/commit and done-marker/ready commit logic).

**Pros:**
- No file provisioning dependency
- Works in any managed repo/worktree

**Cons:**
- Longer command strings in JSON
- Harder to maintain than script files

**Effort:** Small

**Risk:** Low

---

### Option 2: Materialize Hook Scripts Into `.orchestra/hooks/` Per Worktree

**Approach:** During session creation, write executable scripts into `<worktree>/.orchestra/hooks/` and reference those paths in settings.

**Pros:**
- Matches task spec closely
- Keeps hook logic readable and testable

**Cons:**
- Requires file lifecycle/permissions handling
- Must keep generated scripts in sync

**Effort:** Medium

**Risk:** Medium

## Recommended Action

Completed: do not auto-manage Claude hooks; keep hook configuration user/tool managed.

## Technical Details

Affected files:
- `internal/automation/hooks.go:27`
- `internal/automation/hooks.go:28`
- `internal/session/manager.go:168`

## Resources

- `docs/TASK-16-automation.md`

## Acceptance Criteria

- [x] No automatic `.claude/settings.json` hook mutation is performed by session creation
- [x] Existing `.claude/settings.json` remains unchanged
- [x] Tests validate non-intrusive/no-op behavior

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed hook injection implementation against TASK-16 spec
- Verified command paths are hardcoded to non-provisioned `scripts/` paths
- Documented runtime impact and remediation options

**Learnings:**
- Current tests only assert hook JSON structure, not command executability in managed repos

### 2026-02-18 - Resolution

**By:** Codex

**Actions:**
- Removed automatic hook injection from session creation flow
- Switched `EnsureClaudeCodeAutomation` to explicit no-op behavior
- Updated tests to verify no `.claude/settings.json` mutation

**Learnings:**
- Non-intrusive behavior avoids cross-repo assumptions and aligns with user-managed hook policy
