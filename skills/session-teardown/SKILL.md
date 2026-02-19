---
name: session-teardown
description: Close completed or aborted sessions safely after gate checks and release worker capacity.
---
# Session Teardown

## Use When
- Worker task is done/aborted or stage ends.

## Inputs
- `session_id`

## Procedure
1. Run close safety check if required.
2. Close session and mark ended.
3. Confirm capacity slot released.

## Output Contract
- `session_id`
- `closed`: boolean
- `reason`

## Guardrails
- Do not close sessions that still require human response/review handling.
