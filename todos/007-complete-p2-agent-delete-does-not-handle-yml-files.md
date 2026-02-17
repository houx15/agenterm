---
status: complete
priority: p2
issue_id: "007"
tags: [code-review, quality, api]
dependencies: []
---

# Agent Delete Does Not Handle .yml Files

Deleting an agent via `DELETE /api/agents/{id}` fails for registry entries loaded from `.yml` files.

## Problem Statement

The registry loader accepts both `.yaml` and `.yml` files, but deletion always targets `<id>.yaml`. If a user config is stored as `<id>.yml`, API delete returns an error even though the agent exists and is listed.

This creates inconsistent behavior for supported file formats and breaks CRUD expectations from `docs/TASK-09-agent-registry.md`.

## Findings

- Loader accepts both extensions in `internal/registry/registry.go:136`.
- Delete hardcodes `.yaml` in `internal/registry/registry.go:113`.
- API delete path relies on registry delete directly in `internal/api/agents.go:97`, so the mismatch surfaces to clients.

## Proposed Solutions

### Option 1: Track source path per agent in memory

**Approach:** Extend registry cache to keep `{config, sourcePath}` so delete removes the actual loaded file.

**Pros:**
- Correct for both `.yaml` and `.yml`
- Future-proof if additional file handling is needed

**Cons:**
- Slightly larger refactor of internal map structure

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 2: Delete with extension fallback

**Approach:** Try `<id>.yaml`; if not found, try `<id>.yml`.

**Pros:**
- Small, targeted fix
- Minimal code churn

**Cons:**
- Less explicit than source-path tracking

**Effort:** 20-40 minutes

**Risk:** Low

---

### Option 3: Normalize on write and migrate at reload

**Approach:** During reload, rewrite `.yml` files to `.yaml` once and remove old extension.

**Pros:**
- Single canonical extension

**Cons:**
- Implicit file mutation can surprise users
- Higher risk and larger behavior change

**Effort:** 2-3 hours

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/registry/registry.go:113`
- `internal/registry/registry.go:136`
- `internal/api/agents.go:97`

**Related components:**
- Agent Registry YAML filesystem backend
- Agents CRUD API

**Database changes (if any):**
- No

## Resources

- **Spec:** `docs/TASK-09-agent-registry.md`
- **Commit reviewed:** `12892479474108c70b85e8838460fda528efb20e`

## Acceptance Criteria

- [ ] Agent files loaded from `.yaml` can be deleted via API
- [ ] Agent files loaded from `.yml` can be deleted via API
- [ ] Delete behavior covered by automated tests
- [ ] Existing `.yaml` delete behavior remains unchanged

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed latest commit implementation against task spec
- Traced extension handling in registry load and delete paths
- Confirmed extension mismatch in delete path
- Drafted remediation options

**Learnings:**
- Registry currently supports dual extension on read but single extension on delete
- API behavior inherits filesystem extension assumptions from registry

## Notes

- This is a behavior consistency issue; not a security blocker.
