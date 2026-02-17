---
status: complete
priority: p2
issue_id: "020"
tags: [code-review, data-integrity, workflow]
dependencies: []
---

# Workflow update is not transactional and can drop phases

## Problem Statement

Workflow update deletes all phases and reinserts them without a transaction. If reinsertion fails mid-way, workflow metadata may persist while phases are partially or fully lost.

## Findings

- `internal/db/orchestrator_governance_repos.go:304` deletes all phases before reinsertion.
- `internal/db/orchestrator_governance_repos.go:308` reinserts phases one-by-one without transaction guard.

## Proposed Solutions

### Option 1: Wrap workflow update + phase replacement in a DB transaction

Pros:
- Ensures atomicity.
- Prevents partial state corruption.

Cons:
- Slightly more code.

Effort: Small
Risk: Low

### Option 2: Upsert phases with versioned phase set and swap pointer

Pros:
- Strong consistency and auditability.

Cons:
- Higher schema complexity.

Effort: Large
Risk: Medium

## Recommended Action

## Technical Details

Affected files:
- `internal/db/orchestrator_governance_repos.go`

## Acceptance Criteria

- [x] `WorkflowRepo.Update` is fully transactional.
- [x] Failure during phase insert leaves old phase set intact.
- [x] Regression test verifies rollback on injected insert failure.

## Work Log

### 2026-02-17 - Reported during workflows-review

- Identified non-atomic delete+insert replacement path.

### 2026-02-17 - Fixed

- `WorkflowRepo.Update` now wraps metadata + phase replacement in a single DB transaction.
- Added rollback regression test with duplicate phase IDs to force insertion failure.

## Resources

- `internal/db/orchestrator_governance_repos.go:304`
- `internal/db/orchestrator_governance_repos.go:313`
