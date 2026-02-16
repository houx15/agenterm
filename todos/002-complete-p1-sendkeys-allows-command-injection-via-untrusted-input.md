---
status: complete
priority: p1
issue_id: "002"
tags: [code-review, security, tmux, quality]
dependencies: []
resolution: "Added windowID regex validation; use -l literal mode for send-keys"
---

# SendKeys Allows Command Injection Via Untrusted Input

## Problem Statement

`SendKeys` builds tmux command strings by interpolation with insufficient sanitization, allowing command-shape manipulation through `windowID`/`keys` values.

## Findings

- `internal/tmux/gateway.go:188` builds command string with `fmt.Sprintf("send-keys -t %s %s\n", windowID, escaped)`.
- `windowID` is not validated at all (`internal/tmux/gateway.go:182`).
- `escapeKeys` uses shell-like single-quote escaping (`internal/tmux/gateway.go:207`) which is not a robust tmux command-grammar contract.
- Newlines/control chars can alter command boundaries if not strictly encoded.

## Proposed Solutions

### Option 1: Strict tokenized command encoder

**Approach:** Validate `windowID` against `^@[0-9]+$`; encode key payload through a strict builder that emits only allowed tmux key tokens or literal-safe mode.

**Pros:**
- Strong safety guarantees
- Predictable behavior

**Cons:**
- Requires explicit encoder logic

**Effort:** 3-6 hours

**Risk:** Low

---

### Option 2: Literal mode by default

**Approach:** Use tmux literal key sending (`-l`) for text; append specific control tokens (`Enter`, `C-c`) separately.

**Pros:**
- Greatly reduces parsing ambiguity

**Cons:**
- Mixed literal/control payload handling needed

**Effort:** 2-4 hours

**Risk:** Medium

---

### Option 3: Keep current interpolation strategy

**Approach:** No changes.

**Pros:**
- Zero effort

**Cons:**
- Security and correctness risks remain

**Effort:** 0

**Risk:** High

## Recommended Action


## Technical Details

**Affected files:**
- `internal/tmux/gateway.go:182`
- `internal/tmux/gateway.go:188`
- `internal/tmux/gateway.go:207`

**Related components:**
- Websocket input pipeline into `SendKeys`
- Tmux control-mode command writer

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review scope:** Task-01/02 implementation code

## Acceptance Criteria

- [ ] `windowID` is validated before command emission
- [ ] Key encoding contract is explicit and injection-resistant
- [ ] Unit tests include malicious input attempts and verify safe output

## Work Log

### 2026-02-16 - Initial Code Review Finding

**By:** Codex

**Actions:**
- Reviewed `SendKeys` command construction and escaping
- Identified untrusted interpolation path and weak escaping model

**Learnings:**
- Control-mode command construction must be grammar-aware, not shell-escape-inspired

## Notes

- This is a security-critical issue.
