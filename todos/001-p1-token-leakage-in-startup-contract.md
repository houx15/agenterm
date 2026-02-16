---
status: resolved
priority: p1
issue_id: "001"
tags: [code-review, security, quality]
dependencies: []
resolution: "Added --print-token flag; token not printed by default"
---

# Avoid Token Leakage In Startup Contract

## Problem Statement

The task spec requires exposing the auth token in terminal/log output and URL query examples, which creates avoidable credential leakage risk.

## Findings

- `TASK-01-project-scaffold.md:74` asks to log startup message with URL and token.
- `TASK-01-project-scaffold.md:80` requires banner output including `?token=<token>`.
- Console logs are often persisted in CI logs, shell history tools, or screen recordings.
- Query-string tokens are routinely captured by analytics, proxies, and browser history.

## Proposed Solutions

### Option 1: Keep token secret by default

**Approach:** Update requirements so startup output omits token by default and prints a redacted hint only.

**Pros:**
- Removes accidental secret disclosure at source
- Minimal implementation complexity

**Cons:**
- Slightly less copy-paste convenience for first-run access

**Effort:** 30-60 minutes

**Risk:** Low

---

### Option 2: Keep optional explicit reveal flag

**Approach:** Require token masking by default and allow `--print-token` for explicit local debugging.

**Pros:**
- Maintains convenience when intentionally needed
- Better operational security baseline

**Cons:**
- Adds one CLI branch and docs complexity

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 3: Move auth token to header/cookie contract

**Approach:** Update task contract to avoid query tokens entirely and use header/cookie auth in later tasks.

**Pros:**
- Better long-term security posture
- Avoids URL-based secret handling entirely

**Cons:**
- Requires cross-task contract updates

**Effort:** 2-4 hours

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `TASK-01-project-scaffold.md:74`
- `TASK-01-project-scaffold.md:80`

**Related components:**
- Startup banner/output contract
- Future websocket authentication contract

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review target:** `TASK-01-project-scaffold.md`

## Acceptance Criteria

- [ ] Task text no longer requires logging plaintext token by default
- [ ] Auth example avoids query-string token exposure or clearly marks it as temporary/debug-only
- [ ] Security rationale is documented in task notes

## Work Log

### 2026-02-16 - Initial Review Finding

**By:** Codex

**Actions:**
- Reviewed startup/logging requirements in `TASK-01-project-scaffold.md`
- Identified token leakage points in startup contract
- Drafted remediation options and acceptance criteria

**Learnings:**
- Security defaults need to be explicit at scaffold stage because later tasks inherit this contract

## Notes

- This is a spec-level finding; fix in task text before downstream implementation tasks.
