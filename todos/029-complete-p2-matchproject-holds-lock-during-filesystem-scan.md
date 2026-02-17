---
status: complete
priority: p2
issue_id: "029"
tags: [code-review, performance, architecture, backend]
dependencies: []
---

# MatchProject Holds Registry Lock During Filesystem Scan

## Problem Statement

`MatchProject` acquires `RLock` and then scans the repository filesystem. On large repos this can hold the lock for a long time, delaying `Save/Delete/Reload` and increasing contention under concurrent API use.

## Findings

- `MatchProject` acquires `r.mu.RLock()` before calling `inspectProject(repoPath)` in `internal/playbook/playbook.go:151` and `internal/playbook/playbook.go:159`.
- `inspectProject` can walk up to 3000 files and inspect directory entries, which is potentially expensive I/O.
- Lock scope currently includes both expensive I/O and scoring logic.

## Proposed Solutions

### Option 1: Snapshot playbooks under lock, scan outside lock (recommended)

**Approach:** Copy playbook pointers/clones into a local slice/map while holding `RLock`, then release lock and run `inspectProject` + scoring on the snapshot.

**Pros:**
- Minimizes lock hold time
- Preserves deterministic matching behavior

**Cons:**
- Small extra allocations for snapshot

**Effort:** Small

**Risk:** Low

---

### Option 2: Use atomic immutable snapshot field

**Approach:** Store the playbook set in an atomic immutable structure and replace wholesale on reload/save/delete.

**Pros:**
- Very low read contention
- Scales well for frequent reads

**Cons:**
- Larger refactor complexity

**Effort:** Medium

**Risk:** Medium

## Recommended Action

Implemented Option 1: snapshot playbooks under lock, release lock, then perform filesystem inspection and scoring on the snapshot.

## Technical Details

**Affected files:**
- `internal/playbook/playbook.go:151`
- `internal/playbook/playbook.go:159`

## Resources

- Branch: `feature/playbook-system`

## Acceptance Criteria

- [x] `MatchProject` does not hold registry lock during filesystem walk
- [x] Existing matching behavior remains unchanged
- [x] Regression tests continue passing for playbook matching flows

## Work Log

### 2026-02-17 - Review finding created

**By:** Codex

**Actions:**
- Analyzed lock scope and I/O path in `MatchProject`.
- Identified lock contention risk due to filesystem scan under `RLock`.

**Learnings:**
- Correctness is preserved today, but lock granularity can be improved without changing external behavior.

### 2026-02-17 - Fix implemented

**By:** Codex

**Actions:**
- Updated `MatchProject` to copy playbooks into a snapshot while under `RLock`, then release the lock before running `inspectProject`.
- Preserved fallback and tie-break behavior using the snapshot map.
- Ran `go test ./internal/playbook ./internal/api`.

**Learnings:**
- Lock contention was reduced without changing public API behavior.
