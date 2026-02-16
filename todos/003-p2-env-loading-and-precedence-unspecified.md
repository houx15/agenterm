---
status: resolved
priority: p2
issue_id: "003"
tags: [code-review, architecture, quality]
dependencies: []
resolution: "Added explicit env var names (AGENTERM_PORT, AGENTERM_SESSION, AGENTERM_TOKEN) and precedence order (flags > env > file > defaults)"
---

# Specify Env Loading And Precedence Rules

## Problem Statement

The task requires config loading from flags/env/file but does not define env variable names or precedence order, risking inconsistent behavior.

## Findings

- `TASK-01-project-scaffold.md:21` claims loading from flags/env/file.
- No environment variable names are specified (for example `AGENTERM_PORT`, `AGENTERM_TOKEN`, `AGENTERM_SESSION`).
- No precedence contract is defined (common pattern: flags > env > file > defaults).

## Proposed Solutions

### Option 1: Add explicit env key and precedence table

**Approach:** Extend task with concrete env var names and strict precedence order.

**Pros:**
- Deterministic behavior across implementations
- Easier testing and debugging

**Cons:**
- Slightly longer task doc

**Effort:** 30-60 minutes

**Risk:** Low

---

### Option 2: Remove env requirement from this task

**Approach:** Keep v1 to defaults+file+flags only and move env support to later task.

**Pros:**
- Smaller scope for scaffold milestone
- Fewer edge cases now

**Cons:**
- Delays operational ergonomics

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 3: Keep env optional and non-normative

**Approach:** Mark env behavior implementation-defined for task 01.

**Pros:**
- Maximum flexibility

**Cons:**
- Creates compatibility issues between contributors

**Effort:** 10-15 minutes

**Risk:** High

## Recommended Action


## Technical Details

**Affected files:**
- `TASK-01-project-scaffold.md:21`
- `TASK-01-project-scaffold.md:50`

**Related components:**
- CLI/config loading contract
- Test plan for startup behavior

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review target:** `TASK-01-project-scaffold.md`

## Acceptance Criteria

- [ ] Env variable names are explicitly listed
- [ ] Precedence order is explicitly listed
- [ ] At least one test requirement covers precedence behavior

## Work Log

### 2026-02-16 - Initial Review Finding

**By:** Codex

**Actions:**
- Inspected configuration requirements for determinism
- Logged missing env and precedence contract

**Learnings:**
- Undefined precedence is a common source of production misconfiguration

## Notes

- This directly affects operator confidence and reproducibility.
