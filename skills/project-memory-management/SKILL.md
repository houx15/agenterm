---
name: project-memory-management
description: Persist and retrieve durable project knowledge artifacts across orchestration stages.
---
# Project Memory Management

## Use When
- Recording important decisions and evidence.
- Loading context before planning/execution.

## Inputs
- `project_id`
- `kind`, `title`, `content`, optional `source_uri`

## Procedure
1. Read relevant entries before major actions.
2. Write concise durable artifacts:
   - decisions
   - risks
   - review findings
   - finalize summary

## Output Contract
- `entries_written`
- `entries_read`

## Guardrails
- Avoid noisy logs; store high-value summaries only.
