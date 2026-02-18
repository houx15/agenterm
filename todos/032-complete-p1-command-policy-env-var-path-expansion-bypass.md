---
status: complete
priority: p1
issue_id: "032"
tags: [code-review, security, automation]
dependencies: []
---

# Command Policy Can Be Bypassed with Env-Expanded Paths

Path scoping is intended to keep orchestrator/session commands inside the session workdir, but commands using environment expansion (for example `$HOME/.ssh` or `$TMPDIR/file`) are currently misclassified as in-root relative paths and can execute against real paths outside the workdir at shell runtime.

## Problem Statement

`ValidateCommandPolicy` should enforce working-folder-only execution, but current path analysis does not resolve environment variables. This creates a bypass where a command appears compliant during validation and then expands to an out-of-scope absolute path in the shell.

## Findings

- `looksLikePath` treats any token containing `/` as a path (`internal/session/command_policy.go:140`), including `$HOME/.ssh/id_rsa`.
- `validatePathScope` joins non-absolute tokens to `allowedRoot` (`internal/session/command_policy.go:168-170`) without expanding `$VAR` tokens first.
- Example bypass shape: `cat $HOME/.ssh/id_rsa` can pass policy checks while reading outside the workdir after shell expansion.
- This directly conflicts with the intended “only access working folder” command-policy contract.

## Proposed Solutions

### Option 1: Block Variable-Interpolated Path Tokens

**Approach:** Reject any path token containing `$`, `${`, or `%VAR%` patterns before scope checks.

**Pros:**
- Simple and safe default
- Hard to bypass accidentally

**Cons:**
- Rejects some legitimate in-workdir variable usage
- Needs clear error messaging

**Effort:** Small

**Risk:** Low

---

### Option 2: Expand Variables Then Validate Canonical Path

**Approach:** Resolve environment variables per platform (`os.ExpandEnv`) and validate expanded path against workdir.

**Pros:**
- Preserves useful variable-based workflows
- Better developer ergonomics

**Cons:**
- Expansion semantics differ across shells
- More edge cases for quoting/escaping

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Parse and Validate via Shell AST

Use a command parser for shell grammar and evaluate token expansion rules conservatively.

## Recommended Action

To be filled during triage.

## Technical Details

Affected files:
- `internal/session/command_policy.go:132`
- `internal/session/command_policy.go:147`
- `internal/session/command_policy_test.go`

Related components:
- `internal/session/manager.go:253`
- `internal/api/sessions.go:248`

## Resources

- Task spec: `docs/TASK-16-automation.md`
- Policy enforcement call path: `internal/session/manager.go:254`

## Acceptance Criteria

- [ ] Command policy rejects variable-expanded path tokens that can resolve outside workdir
- [ ] New tests cover `$HOME/...`, `${HOME}/...`, and similar patterns
- [ ] Existing allowed in-workdir commands continue to work as expected
- [ ] API returns `403` and command is not forwarded for blocked cases

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed command policy path token extraction and scope resolution
- Identified env-variable path expansion bypass
- Documented fix options and acceptance criteria

**Learnings:**
- Current path validation assumes literal token paths and does not model shell expansion

## Notes

- This is a merge-blocking issue for strict workdir confinement guarantees.
