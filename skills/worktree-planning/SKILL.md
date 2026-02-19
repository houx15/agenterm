---
name: worktree-planning
description: Convert build plan into concrete worktrees and task-to-worktree mapping.
---
# Worktree Planning

## Use When
- Entering build stage with parallel branches.

## Inputs
- stage plan
- dependency graph
- max parallel worktrees

## Procedure
1. Group independent tasks.
2. Create branch/worktree per stream.
3. Assign tasks and worker roles.

## Output Contract
- `worktrees`: `{id, branch, path, task_ids}`
- `parallelism_used`

## Guardrails
- Keep dependent work in ordered streams.
- Avoid excessive worktree churn.
