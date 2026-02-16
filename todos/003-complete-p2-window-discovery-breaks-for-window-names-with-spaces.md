---
status: complete
priority: p2
issue_id: "003"
tags: [code-review, reliability, tmux, quality]
dependencies: []
resolution: "Changed list-windows format to use tab delimiter instead of space"
---

# Window Discovery Breaks For Window Names With Spaces

## Problem Statement

Initial window discovery parses `list-windows` output with `strings.Fields`, which truncates names containing spaces and can corrupt `active` parsing.

## Findings

- `internal/tmux/gateway.go:65` requests `#{window_id} #{window_name} #{window_active}`.
- `internal/tmux/gateway.go:120` parses response using `strings.Fields(line)`.
- `internal/tmux/gateway.go:130` assigns `name := parts[1]`, losing all subsequent tokens of multi-word names.
- `internal/tmux/gateway.go:131` reads `parts[2]` as active flag, which is wrong when name contains spaces.

## Proposed Solutions

### Option 1: Use a safe delimiter in tmux format

**Approach:** Request output with a delimiter unlikely in names (for example tab `\t` or ASCII unit separator), then split on that delimiter.

**Pros:**
- Correct parsing for all normal names
- Minimal code churn

**Cons:**
- Must ensure delimiter escaping rules are consistent

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 2: Emit structured key-value format

**Approach:** Use format like `id=<...> name=<...> active=<...>` with robust parser.

**Pros:**
- Self-describing and extensible

**Cons:**
- Slightly more parsing logic

**Effort:** 2-3 hours

**Risk:** Low

---

### Option 3: Keep current parser

**Approach:** Assume no spaces in names.

**Pros:**
- No implementation effort

**Cons:**
- Incorrect behavior in common user setups

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/tmux/gateway.go:65`
- `internal/tmux/gateway.go:120`
- `internal/tmux/gateway.go:130`

**Related components:**
- Window list synchronization
- UI window naming/display

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review scope:** Task-01/02 implementation code

## Acceptance Criteria

- [ ] Multi-word window names are parsed losslessly
- [ ] `Active` flag remains correct regardless of name content
- [ ] Integration test covers at least one window name containing spaces

## Work Log

### 2026-02-16 - Initial Code Review Finding

**By:** Codex

**Actions:**
- Traced list-windows format command and parser behavior
- Identified tokenization bug with spaced names

**Learnings:**
- Field-splitting raw terminal protocol output is brittle without explicit delimiters

## Notes

- Important correctness issue for real-world sessions.
