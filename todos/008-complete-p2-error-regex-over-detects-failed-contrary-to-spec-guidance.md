---
status: complete
priority: p2
issue_id: "008"
tags: [code-review, quality, parser, reliability]
dependencies: []
resolution: "Anchored 'failed|FAILED' at line start to avoid false positives"
---

# Error Regex Over-Detects "failed" Contrary To Spec Guidance

## Problem Statement

Error detection currently matches `failed` anywhere, which conflicts with spec guidance to avoid over-detection and require stronger delimiters/line starts.

## Findings

- `internal/parser/patterns.go:20` includes unanchored `(?:failed|FAILED)`.
- Spec note warns against false positives for words like `failed` used in non-error contexts (`TASK-03-output-parser.md:187`).
- Current classifier may mark benign lines (for example test summaries or variable names) as `ClassError`.

## Proposed Solutions

### Option 1: Anchor and contextualize error patterns

**Approach:** Require `failed` at line start or adjacent to explicit error tokens (`^failed:`, `\bfailed\s+to\b`, etc.) and keep case-insensitive mode.

**Pros:**
- Reduces false positives
- Better aligns with spec guidance

**Cons:**
- Slightly more regex complexity

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 2: Split error detection into multiple compiled regexes

**Approach:** Use separate strict patterns for line-start errors, stack traces, and panic forms.

**Pros:**
- More maintainable and testable pattern intent

**Cons:**
- Additional refactor effort

**Effort:** 2-4 hours

**Risk:** Low

---

### Option 3: Keep current regex

**Approach:** No changes.

**Pros:**
- No effort

**Cons:**
- Persistent classification false positives

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/parser/patterns.go:20`
- `internal/parser/parser.go:131`

**Related components:**
- Message classification quality
- Prompt/error UI rendering

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-03-output-parser.md`

## Acceptance Criteria

- [ ] `failed` detection uses anchored/contextual form
- [ ] Unit tests cover false-positive examples explicitly
- [ ] Error classification remains correct for real error lines

## Work Log

### 2026-02-16 - Task 03 Implementation Review

**By:** Codex

**Actions:**
- Compared regex implementation to spec anti-false-positive note
- Identified unanchored `failed` token as over-detection source

**Learnings:**
- Broad regex shortcuts quickly degrade classifier precision in terminal output

## Notes

- Quality/reliability issue affecting UX trust.
