---
name: session-bootstrap
description: Start worker sessions in the correct worktree/folder and launch the correct agent TUI command for the selected model.
---
# Session Bootstrap

## Use When
- Starting plan/build/test workers.

## Inputs
- task/worktree context
- selected agent and model
- startup command template

## Procedure
1. Create session bound to task role.
2. Verify working directory is correct.
3. Launch agent command.
4. Read startup output and ensure session is live.

## Output Contract
- `session_id`
- `cwd`
- `startup_status`

## Guardrails
- Never start a worker in the wrong directory.
- Never rely on active terminal state; always use explicit `session_id`.
