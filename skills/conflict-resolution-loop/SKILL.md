---
name: conflict-resolution-loop
description: Run deterministic conflict resolution workflow and verify clean merge completion.
---
# Conflict Resolution Loop

## Use When
- Merge reports conflict.

## Inputs
- `worktree_id`
- optional `session_id`

## Procedure
1. Assign/launch resolver worker.
2. Send explicit conflict-fix instructions.
3. Re-check mergeability.
4. Loop until resolved or blocked.

## Output Contract
- `conflict_state`: `resolved|blocked|needs_human`
- `evidence`: key output snippets

## Guardrails
- Stop and request human intervention if repeated failures exceed threshold.
