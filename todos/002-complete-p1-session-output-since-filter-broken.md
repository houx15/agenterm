---
status: complete
priority: p1
issue_id: "002"
tags: [code-review, api, sessions, correctness]
dependencies: []
---

# Session Output Ignores `since` Contract

The `GET /api/sessions/{id}/output` endpoint does not honor `?since=` semantics and returns synthetic timestamps, breaking API correctness for orchestrator/frontend incremental polling.

## Problem Statement

`docs/TASK-08-rest-api.md` defines output retrieval with `?since=timestamp`. The current implementation returns all captured lines (except future `since`) and assigns the same `time.Now()` timestamp to every line. Clients cannot fetch incremental deltas reliably.

## Findings

- `internal/api/sessions.go:230` parses `since`, but filtering is not applied to actual line timestamps.
- `internal/api/sessions.go:253` stamps each returned line with `now` instead of source timestamp.
- Behavior violates Step 6 output contract in `docs/TASK-08-rest-api.md`.

## Proposed Solutions

### Option 1: Use parser-backed ring buffer with real timestamps

**Approach:** Store session output events with timestamps in memory and filter by `since` before response.

**Pros:**
- Matches spec cleanly
- Supports reliable incremental polling

**Cons:**
- Requires buffer lifecycle management
- Slight memory overhead

**Effort:** Medium

**Risk:** Low

---

### Option 2: Use tmux capture + synthetic monotonic offsets

**Approach:** Keep tmux capture but add sequence IDs and filter by ID instead of timestamp.

**Pros:**
- Lower integration surface
- Avoids parser coupling

**Cons:**
- Deviates from timestamp API contract
- Harder to align with existing `since` param

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Deprecate `since` and document full-buffer only

**Approach:** Return full output always and remove incremental behavior from contract.

**Pros:**
- Minimal code

**Cons:**
- Breaks spec objective
- Increases client load and latency

**Effort:** Small

**Risk:** High

## Recommended Action


## Technical Details

Affected files:
- `internal/api/sessions.go:207`
- `internal/api/sessions.go:230`
- `internal/api/sessions.go:253`

## Resources

- Spec: `docs/TASK-08-rest-api.md`
- Commit: `8d62614664c2502ddf0c95a52b034d9ac20c4bfd`

## Acceptance Criteria

- [ ] `since` returns only entries newer than the provided timestamp
- [ ] Output entries carry stable source timestamps (not response-time timestamps)
- [ ] API tests cover incremental polling behavior and edge cases
- [ ] Existing clients continue to work without regressions

## Work Log

### 2026-02-17 - Review Discovery

**By:** Codex

**Actions:**
- Compared session output implementation against task spec
- Verified timestamp assignment and `since` handling path
- Classified as P1 API contract break

**Learnings:**
- Current output response shape cannot support deterministic incremental reads

## Notes

- This affects orchestrator decision loops and frontend polling efficiency.
