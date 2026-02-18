---
status: complete
priority: p1
issue_id: "036"
tags: [code-review, security, automation]
dependencies: []
---

# Absolute `rm -rf` Ban Is Bypassable With Quoted Absolute Paths

The command policy can miss absolute-path `rm -rf` operations when target paths are shell-quoted.

## Problem Statement

The new hard ban for recursive delete on absolute paths is intended to block dangerous commands regardless of location. Current detection uses `strings.Fields` and `filepath.IsAbs` on raw tokens. Quoted absolute paths (for example `rm -rf '/tmp/x'`) are treated as non-absolute tokens by `filepath.IsAbs`, allowing the command past `no_rm_rf_absolute`.

## Findings

- Absolute-delete ban relies on tokenized fields and direct `filepath.IsAbs`: `internal/session/command_policy.go:98`, `internal/session/command_policy.go:116`.
- Tokenization with `strings.Fields` does not preserve shell semantics for quoted args: `internal/session/command_policy.go:81`.
- Path-scope fallback joins non-abs tokens to root, so quoted absolute paths can be misclassified as in-scope paths: `internal/session/command_policy.go:213`.

## Proposed Solutions

### Option 1: Normalize Shell Quote Wrappers Before Path Classification

**Approach:** Strip matching leading/trailing single/double quotes (and common escapes) before `looksLikePath`/`IsAbs` checks.

**Pros:**
- Small targeted patch
- Closes known bypass quickly

**Cons:**
- Still not a full shell parser

**Effort:** Small

**Risk:** Low

---

### Option 2: Use Shell Lexer For Command Parsing

**Approach:** Parse command with shell-aware lexer (quotes/escapes) before policy checks.

**Pros:**
- Correct behavior for quoting and spacing edge cases
- Better long-term policy correctness

**Cons:**
- More implementation complexity
- Additional dependency or parser maintenance

**Effort:** Medium

**Risk:** Medium

## Recommended Action

Completed: normalize shell-quoted path tokens before rm/scope policy checks.

## Technical Details

Affected files:
- `internal/session/command_policy.go:81`
- `internal/session/command_policy.go:98`
- `internal/session/command_policy.go:188`

## Resources

- `docs/TASK-16-automation.md`
- `todos/033-complete-p1-rm-absolute-ban-bypass-via-wrapper-commands.md`

## Acceptance Criteria

- [x] `rm -rf '/abs/path'` is blocked by `no_rm_rf_absolute`
- [x] `rm -rf "/abs/path"` is blocked by `no_rm_rf_absolute`
- [x] Scope checks treat quoted absolute paths as absolute before root-join logic
- [x] Unit tests cover quoted path variants

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Re-reviewed command policy hard-ban implementation
- Identified quoted absolute-path bypass in rm detection path
- Documented remediation options and acceptance criteria

**Learnings:**
- Wrapper handling was improved, but argument normalization remains incomplete for shell-quoted paths

### 2026-02-18 - Resolution

**By:** Codex

**Actions:**
- Added quoted-token normalization in command policy path handling
- Applied normalization to rm absolute-ban checks and scope validation
- Added tests for single/double-quoted absolute rm targets

**Learnings:**
- Lightweight normalization closes practical bypasses without introducing a full shell parser
