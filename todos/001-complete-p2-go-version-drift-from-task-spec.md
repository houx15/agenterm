---
status: complete
priority: p2
issue_id: "001"
tags: [code-review, quality, tooling]
dependencies: []
---

# Align Go Toolchain Version With Task Spec

`go.mod` now requires Go 1.24.0, but the task spec for this feature explicitly states Go 1.22. This mismatch can break contributors/CI environments pinned to 1.22 and violates documented project constraints for this task.

## Problem Statement

The implementation introduces a toolchain version drift:
- Task spec expectation: Go 1.22
- Current committed module version: Go 1.24.0

If downstream environments are still on 1.22, builds and test runs can fail before runtime validation starts.

## Findings

- `docs/TASK-07-database-models.md:6` states: `Tech stack: Go 1.22`
- `go.mod:3` is now `go 1.24.0`
- This drift happened while introducing `modernc.org/sqlite`; dependency/tooling resolution forced the module version upward during `go mod tidy`.

Impact:
- Potential CI failure on environments pinned to Go 1.22
- Reproducibility risk across developer machines
- Violates task-level compatibility target

## Proposed Solutions

### Option 1: Pin SQLite stack compatible with Go 1.22

**Approach:** Find a `modernc.org/sqlite` + transitive set that can retain `go 1.22`; update `go.mod`/`go.sum` and verify full tests.

**Pros:**
- Restores task-spec compliance
- Minimizes environment churn for existing contributors

**Cons:**
- May require trial/error with versions
- Could lose fixes from newer dependency lines

**Effort:** Medium

**Risk:** Medium

---

### Option 2: Keep Go 1.24 and update project/task docs and CI

**Approach:** Accept the upgrade as intentional; update docs and pipelines to require 1.24 consistently.

**Pros:**
- No dependency rollback complexity
- Keeps latest dependency compatibility

**Cons:**
- Broader upgrade scope outside this task
- May disrupt contributors not ready for 1.24

**Effort:** Small-Medium

**Risk:** Medium

---

### Option 3: Switch SQLite driver

**Approach:** Replace `modernc.org/sqlite` with an alternative that preserves Go 1.22 requirement.

**Pros:**
- Could satisfy both persistence and version constraint

**Cons:**
- Larger refactor and behavior differences
- Possible CGO/runtime tradeoffs

**Effort:** Large

**Risk:** Medium-High

## Recommended Action


## Technical Details

Affected files:
- `go.mod:3`
- `docs/TASK-07-database-models.md:6`

Related components:
- CI toolchain setup
- Developer local Go runtime expectations

Database changes:
- None (toolchain/dependency concern only)

## Resources

- Commit under review: `bbcd389`
- Task spec: `docs/TASK-07-database-models.md`

## Acceptance Criteria

- [ ] Explicit decision documented: either preserve Go 1.22 or officially upgrade baseline to 1.24
- [ ] `go.mod` and docs are consistent with the chosen version
- [ ] CI and local setup instructions match chosen version
- [ ] `go test ./...` passes under the chosen toolchain

## Work Log

### 2026-02-17 - Review Finding Captured

**By:** Codex

**Actions:**
- Reviewed commit `bbcd389` against `docs/TASK-07-database-models.md`
- Identified toolchain mismatch between spec and committed `go.mod`
- Created tracking todo for triage and resolution

**Learnings:**
- `modernc.org/sqlite` integration can force module version updates during dependency resolution
- Toolchain constraints should be validated early when adding heavy transitive dependencies

## Notes

This is marked P2 (important) because it may not break runtime behavior directly, but it can block or destabilize contributor/CI workflows.

### 2026-02-17 - Resolution Applied

**By:** Codex

**Actions:**
- Pinned SQLite driver to `modernc.org/sqlite v1.22.1`
- Re-resolved dependencies with `go mod tidy -go=1.22`
- Verified `go.mod` now declares `go 1.22`
- Re-ran full test suite: `go test ./...` (pass)

**Learnings:**
- Recent `modernc.org/sqlite` lines pull transitive `x/*` modules that can force Go 1.24+
- Using the 1.22.x sqlite line keeps this project aligned with task-level Go 1.22 constraints
