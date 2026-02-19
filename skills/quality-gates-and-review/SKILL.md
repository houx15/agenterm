---
name: quality-gates-and-review
description: Enforce test/review verdict gates before closing sessions, merging branches, or finalizing stages.
---
# Quality Gates And Review

## Use When
- Before merge/close/finalize decisions.

## Inputs
- review verdict
- open issue count
- required checks status

## Procedure
1. Read latest review cycle + issue status.
2. Verify required checks pass.
3. Block progression if gates fail.

## Output Contract
- `gate_passed`: boolean
- `failed_checks`

## Guardrails
- No finalize/merge when open critical issues remain.
