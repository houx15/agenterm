---
name: session-output-reading
description: Retrieve bounded worker output and summarize actionable status for orchestration decisions.
---
# Session Output Reading

## Use When
- Checking progress, blockers, or completion evidence.

## Inputs
- `session_id`
- optional `lines`

## Procedure
1. Read recent output chunk.
2. Identify state: running/idle/waiting/error.
3. Extract critical result lines.

## Output Contract
- `output`
- `state`
- `summary`

## Guardrails
- Keep reads bounded.
- Prefer concise summaries with raw excerpt references.
