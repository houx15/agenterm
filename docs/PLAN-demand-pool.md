# Plan: Demand Pool (Idea Backlog + Orchestrator-Assisted Triage)

## Objective

Provide a dedicated place to capture many user thoughts/feature ideas with strict isolation from active development flow. Demand Pool and Development flow share only project identity and model profile configuration.

## Product Decision

Use a hybrid model with hard boundary:

1. deterministic storage in database (source of truth)
2. dedicated demand-pool orchestration for suggestion/triage/prioritization
3. development orchestrator and demand-pool orchestrator are isolated contexts

No silent orchestrator mutations are allowed.

## Hard Separation Contract

Demand Pool must be separated from Development flow.

Shared:

1. `project_id`
2. model API/provider profile (optional shared settings)

Not shared:

1. prompt context
2. toolset permissions
3. UI surface
4. execution state transitions

## UX Scope

## 1) Dedicated panel/page

Add a separate `Demand Pool` panel/page (not mixed into Sessions/PM execution UI).

Purpose:

1. fast idea capture
2. triage/prioritize
3. select one item to promote into active task execution

## 2) UI elements

1. Quick Add input:
   - single-line thought capture
   - optional tags
2. Rich Add modal:
   - title, description, impact, effort, urgency, risk, tags
3. Demand list/table:
   - status, priority score, timestamps
4. Actions:
   - edit
   - move status
   - merge duplicate
   - promote to task
5. Optional demand chat tab in demand panel:
   - demand-only assistant actions
   - no execution/session commands

## Data Model

Table: `demand_pool_items`

Fields:

1. `id` (pk)
2. `project_id` (fk projects.id)
3. `title` (required)
4. `description` (text)
5. `status` (enum):
   - `captured`
   - `triaged`
   - `shortlisted`
   - `scheduled`
   - `done`
   - `rejected`
6. `priority` (int, sortable)
7. `impact` (int 1-5)
8. `effort` (int 1-5)
9. `risk` (int 1-5)
10. `urgency` (int 1-5)
11. `tags` (json array string)
12. `source` (e.g. `user`, `orchestrator`, `import`)
13. `created_by` (optional)
14. `selected_task_id` (nullable fk tasks.id)
15. `notes` (optional)
16. `created_at`
17. `updated_at`

Optional later:

1. `duplicate_of`
2. `archived_at`

## API Design

Base: `/api/projects/{id}/demand-pool`

Endpoints:

1. `GET /api/projects/{id}/demand-pool`
   - filters: `status`, `tag`, `q`, `limit`, `offset`
2. `POST /api/projects/{id}/demand-pool`
   - create demand item
3. `GET /api/demand-pool/{itemId}`
4. `PATCH /api/demand-pool/{itemId}`
5. `DELETE /api/demand-pool/{itemId}` (soft delete recommended)
6. `POST /api/demand-pool/{itemId}/promote`
   - creates task
   - sets `selected_task_id`
   - optionally sets status to `scheduled`
7. `POST /api/projects/{id}/demand-pool/reprioritize`
   - input: reordered IDs or scoring update

## Demand Orchestrator APIs (separate lane)

Add dedicated endpoints:

1. `POST /api/demand-orchestrator/chat`
2. `GET /api/demand-orchestrator/history`
3. `GET /api/demand-orchestrator/report`

Execution orchestrator endpoints remain unchanged and demand-blind.

## Orchestrator Integration

Demand-orchestrator tools:

1. `list_demand_pool(project_id, filters?)`
2. `create_demand_item(project_id, title, description?, tags?, ... )`
3. `update_demand_item(item_id, patch)`
4. `promote_demand_item(item_id, task_payload?)`
5. `reprioritize_demand_pool(project_id, strategy|order)`

Explicitly excluded from demand toolset:

1. `create_session`
2. `send_command`
3. `create_worktree`
4. `merge_worktree`
5. any session/task execution tool except promote operation

Execution orchestrator toolset excludes demand-pool mutations.

## Trigger points (demand lane only)

1. `demand_chat_message_received`
   - detect candidate demands
   - suggest only
2. `demand_capture_confirmed`
   - write selected items
3. `manual_reprioritize_requested` (button)
4. `periodic_digest_requested` (button, not cron)
Development events do not auto-trigger demand mutations.

## Approval Rules

Mutating demand-pool operations by orchestrator require explicit approval:

1. create
2. status change
3. reprioritize
4. promote to task
5. delete/reject

Detection/suggestion can be automatic.

Cross-lane promotion rule:

1. Demand -> Task promotion is the only bridge.
2. Promotion must be explicit user-approved action.

## Status Semantics

1. `captured`: raw thought, minimal detail
2. `triaged`: basic scoring and clarified description
3. `shortlisted`: candidate for near-term implementation
4. `scheduled`: selected for active plan/task conversion
5. `done`: implemented or no longer needed
6. `rejected`: intentionally not pursued

## Rollout Slices

## Slice A: Storage + API

1. migration + repo
2. CRUD endpoints
3. promote endpoint (task linkage)
4. tests

## Slice B: UI panel

1. navigation entry
2. list + quick add + edit modal (demand-only)
3. promote action
4. basic filtering

## Slice C: Orchestrator tools + approvals

1. demand-orchestrator tool definitions (separate toolset)
2. approval-gated mutations
3. demand-only chat/report surfaces

## Slice D: Prioritization helpers

1. manual reorder/re-score
2. orchestrator “re-rank now” action
3. digest summary output

## Acceptance Criteria

1. user can capture ideas without leaving current workflow context
2. demand pool is strictly separated from active execution panels and prompts
3. execution orchestrator cannot mutate demand pool
4. demand orchestrator cannot run development execution tools
5. demand orchestrator can suggest and (after approval) mutate demand pool
6. one-click promote creates linked task and marks demand as scheduled
7. backlog remains auditable and deterministic (DB as source of truth)
