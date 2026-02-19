---
name: git-integration
description: Perform commit and merge operations with policy checks and explicit conflict handling.
---
# Git Integration

## Use When
- Completing worker changes.
- Integrating worktrees.

## Inputs
- session/worktree context
- commit message or merge target

## Procedure
1. Validate working tree state.
2. Commit focused changes.
3. Merge worktree to target branch.
4. If conflict, switch to conflict resolution loop.

## Output Contract
- `commit_hash`
- `merge_status`: `merged|already_merged|conflict`

## Guardrails
- Never force merge without conflict resolution evidence.
