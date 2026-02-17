---
status: complete
priority: p3
issue_id: "030"
tags: [code-review, quality, backend, api]
dependencies: []
---

# Playbook API Error Mapping Uses Fragile String Matching

## Problem Statement

Playbook create/update handlers determine 400 vs 500 using `strings.Contains(err.Error(), "write playbook")`. This is brittle and can silently break if error text changes.

## Findings

- Error classification appears in `internal/api/playbooks.go:47` and `internal/api/playbooks.go:78`.
- Classification is based on message text instead of typed/sentinel errors.

## Proposed Solutions

### Option 1: Introduce typed/sentinel errors (recommended)

**Approach:** Return wrapped sentinel errors from playbook package (validation vs storage), and map with `errors.Is`.

**Pros:**
- Stable status mapping
- Better long-term maintainability

**Cons:**
- Small refactor across package boundaries

**Effort:** Small

**Risk:** Low

---

### Option 2: Map by dedicated helper in playbook package

**Approach:** Provide helper that returns canonical error kind/status and keep handler thin.

**Pros:**
- Centralizes classification
- Keeps API handlers simple

**Cons:**
- Introduces extra API surface

**Effort:** Small

**Risk:** Low

## Recommended Action

Implemented Option 1: introduced typed sentinel errors in the playbook package and mapped HTTP status codes via `errors.Is` in the API layer.

## Technical Details

**Affected files:**
- `internal/api/playbooks.go:47`
- `internal/api/playbooks.go:78`

## Resources

- Branch: `feature/playbook-system`

## Acceptance Criteria

- [x] Error-to-status mapping no longer depends on substring checks
- [x] Validation and storage failures are mapped deterministically
- [x] Tests cover mapping behavior

## Work Log

### 2026-02-17 - Review finding created

**By:** Codex

**Actions:**
- Reviewed playbook API handlers and error mapping logic.
- Logged brittleness risk and refactor options.

**Learnings:**
- Current behavior works but is fragile to message text changes.

### 2026-02-17 - Fix implemented

**By:** Codex

**Actions:**
- Added `ErrInvalidPlaybook`, `ErrPlaybookStorage`, and `ErrPlaybookNotFound` in `internal/playbook/playbook.go`.
- Updated API handlers to use `playbookStatusCode(err)` with `errors.Is` checks.
- Added `internal/api/playbooks_test.go` for status-mapping coverage.
- Ran `go test ./internal/playbook ./internal/api`.

**Learnings:**
- Typed error classification provides stable HTTP mapping and cleaner handler logic.
