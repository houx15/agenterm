---
status: complete
priority: p2
issue_id: "002"
tags: [code-review, architecture, quality]
dependencies: []
resolution: "Renamed config file from .toml to config (no extension) to match key=value format"
---

# Resolve Config Format Contradiction

## Problem Statement

The task specifies a `.toml` config path while also requiring a non-TOML key=value format, creating implementation ambiguity.

## Findings

- `TASK-01-project-scaffold.md:47` states `~/.config/agenterm/config.toml`.
- `TASK-01-project-scaffold.md:53` explicitly asks for simple `key=value` format and no TOML library.
- Engineers may either misuse `.toml` extension with non-TOML content or diverge on parser behavior.

## Proposed Solutions

### Option 1: Keep key=value and rename file

**Approach:** Update path to `~/.config/agenterm/config` or `config.env` to match format.

**Pros:**
- Eliminates semantic mismatch
- Keeps v1 parser simple

**Cons:**
- Slightly less familiar filename for some users

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 2: Keep `.toml` and require valid TOML subset

**Approach:** Keep `config.toml` but require valid TOML syntax (still parse minimally).

**Pros:**
- Future-friendly extension
- Consistent user expectation

**Cons:**
- Slightly stricter parser behavior needed

**Effort:** 1-2 hours

**Risk:** Medium

---

### Option 3: Explicitly label `.toml` as placeholder

**Approach:** Keep text but state extension is legacy placeholder and content is line-based key=value for v1 only.

**Pros:**
- Minimal doc changes

**Cons:**
- Retains technical debt and confusion

**Effort:** 15 minutes

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `TASK-01-project-scaffold.md:47`
- `TASK-01-project-scaffold.md:53`

**Related components:**
- Config parser contract
- Cross-task compatibility for future config loading

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review target:** `TASK-01-project-scaffold.md`

## Acceptance Criteria

- [ ] Config filename and format are semantically aligned in spec
- [ ] Parsing expectations are explicitly documented
- [ ] Future migration path (if any) is noted

## Work Log

### 2026-02-16 - Initial Review Finding

**By:** Codex

**Actions:**
- Cross-checked config path and file format requirements
- Identified contradiction and documented options

**Learnings:**
- Ambiguous file-format contracts cause divergent implementations early in greenfield projects

## Notes

- Prefer naming that reflects actual file format to reduce onboarding friction.
