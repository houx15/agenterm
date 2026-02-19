---
name: stage-gates-and-approval
description: Enforce explicit user approvals for stage transitions and major mutating execution.
---
# Stage Gates And Approval

## Use When
- Transitioning `plan -> build -> test`.
- Executing major mutating actions.

## Inputs
- current stage
- proposal payload (who/how many/outputs)
- latest user response

## Procedure
1. Propose plan and expected effects.
2. Ask explicit confirmation.
3. Only execute after confirmation.

## Output Contract
- `proposal`
- `approved`: boolean
- `next_action`

## Guardrails
- Ambiguous responses are not approval.
- Block mutating actions until explicit confirmation.
