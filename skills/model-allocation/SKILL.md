---
name: model-allocation
description: Select agent/model assignments by playbook role constraints, capacity limits, and requested parallelism, then report mapping before execution.
---
# Model Allocation

## Use When
- A stage starts and worker assignments are needed.
- Parallelism/worktree count changes.

## Inputs
- Stage role definitions (`name`, `allowed_agents`, responsibilities).
- Live capacity (`max_parallel_agents`, role/project/global limits).
- Planned worktree count.

## Procedure
1. Read stage roles and required worker count.
2. Filter candidate agents by `allowed_agents`.
3. Apply capacity constraints in this order:
   - global max
   - project max
   - role max
   - agent max
4. Produce deterministic assignment.

## Output Contract
- `role_assignments`: list of `{role, agent_id, model, count}`
- `blocked_roles`: list of `{role, reason}`
- `allocation_summary`: short explanation

## Guardrails
- Never over-allocate capacity.
- If blocked, return explicit reason and next action.
