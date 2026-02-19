---
name: capacity-scheduler
description: Schedule worker creation according to global/project/role/agent capacity and current utilization.
---
# Capacity Scheduler

## Use When
- Creating sessions for parallel work.

## Inputs
- current active sessions
- global/project limits
- agent max parallel
- role limits

## Procedure
1. Compute active counts by scope.
2. Validate candidate allocation.
3. Approve or reject with reason.

## Output Contract
- `allowed`: boolean
- `reason`
- `next_available_hint`

## Guardrails
- Never bypass hard limits.
