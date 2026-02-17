---
status: complete
priority: p2
issue_id: "007"
tags: [code-review, api, database, consistency]
dependencies: []
---

# Worktree Deletion Leaves Stale `task.worktree_id`

`DELETE /api/worktrees/{id}` removes the worktree row and filesystem worktree, but it does not clear any task linkage that was set when the worktree was created.

## Problem Statement

`createWorktree` links a task to a worktree by writing `task.WorktreeID` (`internal/api/worktrees.go:101`).

On deletion, `deleteWorktree` deletes the worktree record (`internal/api/worktrees.go:204`) but does not clear `task.worktree_id` on the linked task. This creates stale references and inconsistent task state.

## Findings

- Task linkage is created during worktree creation: `internal/api/worktrees.go:101`.
- Worktree deletion removes DB row but does not update linked task: `internal/api/worktrees.go:186` and `internal/api/worktrees.go:204`.
- The `tasks` table keeps `worktree_id` as plain text (no DB-level FK cleanup), so stale values persist unless explicitly cleared.

## Proposed Solutions

### Option 1: Clear linked task before deleting worktree row

**Approach:** In `deleteWorktree`, if `worktree.TaskID` is set, load task and clear `task.WorktreeID` before `worktreeRepo.Delete`.

**Pros:**
- Minimal code change
- Keeps task/worktree model consistent

**Cons:**
- Requires extra DB read/write

**Effort:** Small

**Risk:** Low

---

### Option 2: Add repository helper to clear by worktree ID

**Approach:** Add `TaskRepo.ClearWorktreeIDByWorktreeID(ctx, worktreeID)` and call it in deletion path.

**Pros:**
- Centralized consistency helper
- Easier reuse for future cleanup flows

**Cons:**
- Slightly more repository surface area

**Effort:** Small-Medium

**Risk:** Low

## Recommended Action

Use **Option 1** now for fast correctness, then consider Option 2 if more lifecycle operations need bulk cleanup.

## Technical Details

Affected files:
- `internal/api/worktrees.go`
- `internal/db/task_repo.go` (if Option 2 is chosen)

## Resources

- Review target: commit `a8b0e6c4c5a44dab0b0393df7c0dc83d1af64ce4`
- Spec: `docs/TASK-11-worktree-management.md`

## Acceptance Criteria

- [ ] Deleting a worktree clears any linked task's `worktree_id`
- [ ] API test verifies `DELETE /api/worktrees/{id}` leaves task without stale `worktree_id`
- [ ] Existing worktree delete behavior (filesystem + DB worktree row removal) remains intact

## Work Log

### 2026-02-17 - Review Discovery

**By:** Codex

**Actions:**
- Compared worktree create/delete DB side effects against TASK-11 lifecycle expectations
- Confirmed create path sets `task.worktree_id`, delete path does not unset it

**Learnings:**
- Without explicit cleanup, tasks can retain orphaned worktree references after deletion.
