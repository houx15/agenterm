---
status: complete
priority: p2
issue_id: "005"
tags: [code-review, api, worktree, security]
dependencies: []
---

# Worktree Path Can Escape Project Repository

`POST /api/projects/{id}/worktrees` accepts arbitrary absolute paths and joins relative paths without enforcing repository boundaries.

## Problem Statement

Although command injection is avoided with `exec.Command`, unrestricted path selection allows creating worktrees outside expected project-controlled directories. This increases operational and security risk.

## Findings

- `internal/api/worktrees.go:71` trusts user-provided `path`.
- `internal/api/worktrees.go:75` only normalizes relative paths, but absolute paths are accepted unchanged.
- Notes in task spec require input sanitization; this currently lacks path-scope enforcement.

## Proposed Solutions

### Option 1: Enforce `.worktrees/` under `project.repo_path`

**Approach:** Ignore user absolute paths and always derive canonical path under repo-local `.worktrees/<slug>`.

**Pros:**
- Strong containment
- Predictable filesystem layout

**Cons:**
- Less flexibility for advanced users

**Effort:** Small

**Risk:** Low

---

### Option 2: Allow custom paths with allowlist validation

**Approach:** Permit only paths under `project.repo_path` after `filepath.Clean` + prefix checks.

**Pros:**
- Flexibility retained
- Prevents escaping repository root

**Cons:**
- Slightly more validation complexity

**Effort:** Small

**Risk:** Low

## Recommended Action


## Technical Details

Affected files:
- `internal/api/worktrees.go:71`
- `internal/api/worktrees.go:75`

## Resources

- Spec: `docs/TASK-08-rest-api.md`

## Acceptance Criteria

- [ ] Worktree creation rejects paths outside approved project boundaries
- [ ] API tests cover traversal/escape attempts
- [ ] Error response explains path validation failure

## Work Log

### 2026-02-17 - Review Discovery

**By:** Codex

**Actions:**
- Reviewed worktree path normalization logic
- Assessed boundary validation coverage

**Learnings:**
- Command safety and path safety are separate controls.
