---
status: complete
priority: p2
issue_id: "031"
tags: [code-review, automation, configuration, claude-code]
dependencies: []
---

# Claude hook injection overwrites existing `.claude/settings.json`

## Problem Statement

Hook injection currently rewrites `.claude/settings.json` from scratch. Any pre-existing Claude settings are lost, which can break project-specific behavior and user configurations.

## Findings

- `internal/automation/hooks.go:99` constructs a fresh settings object.
- `internal/automation/hooks.go:124` writes it directly to `.claude/settings.json`.
- No read/merge path exists for existing settings.

## Proposed Solutions

### Option 1: Read-merge-write preserving unknown fields (recommended)

**Approach:** Load existing JSON into a generic map or typed struct with raw extensions, merge required hooks idempotently, then write back.

**Pros:**
- Preserves user/project configuration.
- Re-running hook setup is safe.

**Cons:**
- Slightly more code complexity.

**Effort:** 2-4 hours

**Risk:** Medium

---

### Option 2: Write separate automation settings file and document include

**Approach:** Keep automation hooks in a dedicated file and require explicit include/merge behavior.

**Pros:**
- Avoids modifying existing settings directly.

**Cons:**
- Depends on external include support and user setup.

**Effort:** 3-5 hours

**Risk:** Medium

## Recommended Action

**To be filled during triage.**

## Technical Details

**Affected files:**
- `internal/automation/hooks.go:93`
- `internal/automation/hooks_test.go:12`

**Related components:**
- Session creation (`claude-code` path)
- Worktree hook provisioning

**Database changes (if any):**
- Migration needed? No

## Resources

- **Branch:** `feature/automation`
- **Spec:** `docs/TASK-16-automation.md`

## Acceptance Criteria

- [ ] Existing `.claude/settings.json` keys remain intact after hook injection.
- [ ] Required automation hooks are present exactly once (idempotent merge).
- [ ] Add tests covering existing settings preservation + repeated invocation.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed hook settings generation and write path.
- Verified current logic always overwrites file.

**Learnings:**
- Current implementation is functionally correct for empty configs but unsafe for existing ones.

## Notes

- This is an operational reliability/configuration safety issue, not a compile/runtime panic.
