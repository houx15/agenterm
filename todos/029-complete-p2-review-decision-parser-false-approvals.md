---
status: complete
priority: p2
issue_id: "029"
tags: [code-review, automation, coordinator, quality]
dependencies: []
---

# Review decision parser can incorrectly approve rejected reviews

## Problem Statement

The coordinator marks a review as approved whenever output contains `APPROVED` or `LGTM` as a substring. This creates false positives for phrases like “not approved” and can mark tasks completed incorrectly.

## Findings

- `internal/automation/coordinator.go:325` uppercases the full text and uses substring matching for approval tokens.
- The parser does not require token boundaries or a structured verdict line.
- `internal/automation/coordinator_test.go` does not cover negative phrases that include approval substrings.

## Proposed Solutions

### Option 1: Explicit verdict protocol (recommended)

**Approach:** Require reviewer output to include a dedicated first-line verdict (`VERDICT: APPROVED|CHANGES_REQUESTED`) and parse only that line.

**Pros:**
- Deterministic and testable.
- Avoids language ambiguity.

**Cons:**
- Requires prompt+parser update.

**Effort:** 1-3 hours

**Risk:** Low

---

### Option 2: Safer heuristic regex

**Approach:** Accept approval only when a full-line token matches `^\\s*(APPROVED|LGTM)\\s*$` (or similar), and reject negated forms.

**Pros:**
- Minimal change footprint.

**Cons:**
- Still heuristic and brittle.

**Effort:** 1-2 hours

**Risk:** Medium

## Recommended Action

**To be filled during triage.**

## Technical Details

**Affected files:**
- `internal/automation/coordinator.go:325`
- `internal/automation/coordinator_test.go:9`

**Related components:**
- Coordinator review loop
- Task completion state transitions

**Database changes (if any):**
- Migration needed? No

## Resources

- **Branch:** `feature/automation`
- **Spec:** `docs/TASK-16-automation.md`

## Acceptance Criteria

- [ ] Negative phrases like “not approved” or “cannot LGTM” do not produce approval.
- [ ] Positive verdicts still complete tasks.
- [ ] Parser behavior is covered by unit tests with approval/negation edge cases.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed review-decision parsing logic and tests.
- Validated substring-based approval condition.

**Learnings:**
- Current parser is too permissive for production decisioning.

## Notes

- This can cause silent correctness failures in automation outcomes.
