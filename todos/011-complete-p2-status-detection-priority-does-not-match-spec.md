---
status: complete
priority: p2
issue_id: "011"
tags: [code-review, architecture, lifecycle]
dependencies: []
---

# Align status detection priority with spec 5.3

Detection order in `Monitor.detectStatus()` does not follow the priority sequence required by TASK-12/SPEC 5.3.

## Problem Statement

`docs/TASK-12-session-lifecycle.md` Step 4 references SPEC 5.3 ordering: shell prompt, idle timeout, marker file, git commit. Current implementation checks marker file first, which can produce `completed` before prompt/idle states, changing expected lifecycle semantics.

## Findings

- `internal/session/monitor.go:145` starts with `isMarkerDone()`.
- `internal/session/monitor.go:149` checks prompt after marker.
- `internal/session/monitor.go:152` checks idle after prompt.
- `internal/session/monitor.go:155` checks `[READY_FOR_REVIEW]` commit last.

## Proposed Solutions

### Option 1: Reorder checks to exact spec priority

**Approach:** Implement priority as: prompt -> idle -> marker -> ready-for-review commit.

**Pros:**
- Exact match with documented behavior.
- More predictable orchestrator input states.

**Cons:**
- May change behavior for sessions currently relying on marker-first completion.

**Effort:** <1 hour

**Risk:** Low

---

### Option 2: Keep current order but update spec/comments

**Approach:** If marker-first is intentional, update TASK-12 and code comments to declare that.

**Pros:**
- No runtime behavior change.

**Cons:**
- Diverges from current stated spec and likely orchestrator assumptions.

**Effort:** <1 hour

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/monitor.go`
- `docs/TASK-12-session-lifecycle.md` (if behavior stays as-is)

## Resources

- **Commit:** `b29a91a`
- **Spec:** `SPEC.md:314`
- **Task:** `docs/TASK-12-session-lifecycle.md`

## Acceptance Criteria

- [ ] Detection order matches agreed priority and is documented.
- [ ] Unit tests cover at least one conflicting-signal scenario (prompt + marker, idle + marker).

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Compared `detectStatus()` logic with spec priority list.
- Identified order mismatch with marker placed first.

**Learnings:**
- Priority ordering can materially alter emitted status and downstream automation behavior.

## Notes

- If orchestrator relies on `waiting_review` before `completed`, this mismatch is impactful.
