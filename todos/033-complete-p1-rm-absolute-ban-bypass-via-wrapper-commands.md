---
status: complete
priority: p1
issue_id: "033"
tags: [code-review, security, automation]
dependencies: []
---

# Absolute Recursive Delete Ban Is Bypassable via Wrapper Commands

The policy intends to ban `rm -rf` on absolute paths, but the check only executes when the first token is exactly `rm`.

## Problem Statement

Hard-coded dangerous command bans are incomplete. Wrapper invocations (for example `sudo rm -rf /abs/path`, `command rm -rf /abs/path`, `env rm -rf /abs/path`) can bypass the `no_rm_rf_absolute` rule when target paths are still policy-permitted.

## Findings

- The absolute `rm` block is gated by `strings.EqualFold(fields[0], "rm")` (`internal/session/command_policy.go:82`).
- Wrapped command forms are not normalized before rule evaluation.
- Path-scope checks do not replace this rule because requirement is to ban absolute recursive delete patterns explicitly, not only out-of-root paths.
- Missing tests for wrapper command forms in `internal/session/command_policy_test.go`.

## Proposed Solutions

### Option 1: Normalize Wrapper Prefixes Before Rule Checks

**Approach:** Strip known wrappers (`sudo`, `env`, `command`, `nohup`) and evaluate effective executable/args.

**Pros:**
- Preserves current design
- Targets known bypass vectors quickly

**Cons:**
- Needs maintenance for additional wrappers
- Can miss complex shell patterns

**Effort:** Small

**Risk:** Medium

---

### Option 2: Token-Scan for Recursive rm Pattern Anywhere in Command Segment

**Approach:** Detect `rm` + recursive flags + absolute target regardless of token index.

**Pros:**
- More robust against wrapper ordering
- Easier to test comprehensively

**Cons:**
- Higher false-positive risk on text-like commands

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Positive Allowlist for Command Families

Allow only approved safe commands and deny everything else.

## Recommended Action

To be filled during triage.

## Technical Details

Affected files:
- `internal/session/command_policy.go:82`
- `internal/session/command_policy_test.go`

Related call paths:
- `internal/session/manager.go:254`
- `internal/api/sessions.go:249`

## Resources

- Task context: `docs/TASK-16-automation.md`
- Audit path: `internal/session/command_policy.go:205`

## Acceptance Criteria

- [ ] Wrapper forms of recursive absolute delete are blocked with `CommandPolicyError`
- [ ] Unit tests include `sudo rm -rf /...`, `env rm -rf /...`, and `command rm -rf /...`
- [ ] Blocked commands are audited to `.orchestra/command-policy-audit.log`
- [ ] No regression for safe non-destructive commands

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Audited hard-coded delete-ban logic
- Found wrapper-based bypass due to first-token-only detection
- Captured remediation options and acceptance criteria

**Learnings:**
- Current implementation enforces rule intent only for direct `rm` invocations

## Notes

- Marked P1 because this weakens explicit destructive-command safeguards.
