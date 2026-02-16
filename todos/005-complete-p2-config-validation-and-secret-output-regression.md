---
status: complete
priority: p2
issue_id: "005"
tags: [code-review, security, quality, config]
dependencies: []
resolution: "Added port validation (1-65535); added --print-token flag; token hidden by default"
---

# Config Validation And Secret Output Handling Need Hardening

## Problem Statement

Configuration parsing accepts invalid values silently and startup output prints authentication token unconditionally, increasing misconfiguration and secret-exposure risk.

## Findings

- `internal/config/config.go:77` parses port via `fmt.Sscanf` and ignores parse error; malformed values can yield `0` without warning.
- `internal/config/config.go:50` prints generated token to stdout unconditionally.
- `cmd/agenterm/main.go:84` prints access URL including token on every startup.
- No `--print-token`/redaction control exists in current code.

## Proposed Solutions

### Option 1: Validate config + redact secrets by default

**Approach:** Validate parsed port range and return clear errors on invalid input; remove default token printing and require explicit opt-in flag.

**Pros:**
- Safer defaults and clearer operator feedback
- Prevents secret leakage in logs/history

**Cons:**
- Minor behavior change for users relying on printed token

**Effort:** 2-4 hours

**Risk:** Low

---

### Option 2: Keep printing but mask token

**Approach:** Print only partial token (for example first/last 4 chars) and provide copy command for full reveal.

**Pros:**
- Some usability preserved

**Cons:**
- Still leaks credential metadata and weakens security posture

**Effort:** 1-2 hours

**Risk:** Medium

---

### Option 3: Keep current behavior

**Approach:** No changes.

**Pros:**
- No implementation effort

**Cons:**
- Ongoing risk of secret exposure and invalid config surprises

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/config/config.go:50`
- `internal/config/config.go:77`
- `cmd/agenterm/main.go:84`

**Related components:**
- Startup UX and security posture
- Config loading reliability

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review scope:** Task-01/02 implementation code

## Acceptance Criteria

- [ ] Invalid port values fail with explicit error
- [ ] Startup output omits full token by default
- [ ] Explicit opt-in exists for printing full token

## Work Log

### 2026-02-16 - Initial Code Review Finding

**By:** Codex

**Actions:**
- Reviewed config parsing and startup output behavior
- Identified silent parse failure and unconditional secret output

**Learnings:**
- Strict config validation and secret-safe defaults reduce early operational incidents

## Notes

- Security/quality issue; should be addressed before broader rollout.
