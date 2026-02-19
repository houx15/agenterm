---
name: prompt-dispatch
description: Send prompts/commands to a specific session through serialized command routing and ledger tracking.
---
# Prompt Dispatch

## Use When
- A worker must execute a command or task prompt.

## Inputs
- `session_id`
- prompt/command text

## Procedure
1. Validate target session exists and is not in human takeover.
2. Enqueue command for that session.
3. Send command and track lifecycle state.

## Output Contract
- `command_id`
- `session_id`
- `status`: `queued|running|done|failed`

## Guardrails
- One command stream per session.
- Record result snippet for auditability.
