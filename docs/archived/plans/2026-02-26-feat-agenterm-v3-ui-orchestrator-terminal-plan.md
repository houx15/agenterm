---
title: "feat: agenterm v3 ui orchestrator terminal refactor"
type: feat
status: superseded
date: 2026-02-26
owner: platform
---

# Agenterm V3 Refactor Plan (UI, Interaction, TUI Reliability)

Superseded by:

- `docs/PLAN-vnext-tauri-rewrite.md`

## 1. Objective

Refactor agenterm so it works as a true multi-agent control plane:

1. Users can see available agent capacity in real time and join allocation decisions.
2. Orchestrator focuses on management/execution coordination, not pretending to do coding itself.
3. Long workflows (TDD, multi-task, multi-worktree) become auditable and controllable.
4. TUI command injection/output handling becomes reliable enough for unattended loops.
5. UX quality reaches a "daily driver" bar on desktop, with lightweight mobile control.

## 2. External Research Summary

## 2.1 Superset (code-level findings, GitHub MCP)

Key implementation patterns (validated in repository code):

1. Persistent terminal host daemon with UDS + auth token and NDJSON IPC.
   - `apps/desktop/src/main/terminal-host/index.ts`
2. Split client roles (`control` vs `stream`) and centralized socket/session detach on disconnect.
   - `apps/desktop/src/main/terminal-host/index.ts`
3. PTY subprocess isolation with framed binary IPC and explicit spawn/ready lifecycle.
   - `apps/desktop/src/main/terminal-host/session.ts`
   - `apps/desktop/src/main/terminal-host/pty-subprocess.ts`
4. Backpressure-aware input/output flow control:
   - write queues
   - EAGAIN/EWOULDBLOCK retries
   - high/low watermarks
   - pause/resume on drain.
   - `apps/desktop/src/main/terminal-host/pty-subprocess.ts`
   - `apps/desktop/src/main/terminal-host/session.ts`
5. Snapshot-boundary attach and terminal readiness gates before consuming stream output.
   - `apps/desktop/src/main/terminal-host/session.ts`
   - `apps/desktop/src/renderer/screens/main/components/WorkspaceView/ContentView/TabsContent/Terminal/hooks/useTerminalStream.ts`
6. Stream durability and cursor protocol for chat/session event replay.
   - `apps/api/src/app/api/chat/lib.ts`
   - `apps/api/src/app/api/chat/[sessionId]/stream/route.ts`
7. MCP tool registry separated by domain (tasks/org/devices), with device-routed execution.
   - `packages/mcp/src/tools/index.ts`
   - `packages/mcp/src/tools/devices/start-agent-session/start-agent-session.ts`
   - `packages/mcp/src/tools/devices/create-workspace/create-workspace.ts`

Implication for agenterm:

1. Move from best-effort tmux writes to queued command protocol with ACK state.
2. Split control and stream channels to prevent read/write contention.
3. Add explicit session readiness and attach snapshot semantics.
4. Add durable event stream with cursor replay for PM chat and session timelines.

Source:

- https://github.com/superset-sh/superset

## 2.2 golutra (code-level findings, GitHub MCP)

Key implementation patterns:

1. Local PTY ownership and session registry in desktop backend (Tauri/Rust).
   - `src-tauri/src/lib.rs`
2. Bounded session buffers + global memory caps, with explicit session lifecycle state.
   - `src-tauri/src/lib.rs`
3. Initial command write strategy with timeout + first-output fallback trigger.
   - `src-tauri/src/lib.rs`
4. Session status transitions (`online`/`working`/`offline`) emitted to UI.
   - `src-tauri/src/lib.rs`

Implication for agenterm:

1. Keep explicit per-session status model and expose it consistently in dashboard + PM chat.
2. Add bounded output buffering and deterministic initial prompt dispatch policy.
3. Preserve desktop-first control path for heavy terminal interactions.

Source:

- https://github.com/golutra/golutra

## 2.3 vibekanban (GitHub repo inspected)

Note: `vibekanban.com` product docs and `shahriarb/vibekanban` are not the same codebase.

Observed from repository:

1. Lightweight Kanban + MCP server pattern centered on task state visibility.
2. Strong emphasis on agent-readable process instructions and ticket state transitions.
   - `README.md`
   - `kanban_mcp_server.py`

Implication for agenterm:

1. Demand pool/task intake can stay simple and structured.
2. Keep orchestration and task management loosely coupled via explicit APIs/events.

Source:

- https://github.com/shahriarb/vibekanban

## 3. Current Agenterm Gap Analysis

## 3.1 UX/IA gaps

Current code locations:

- `frontend/src/pages/PMChat.tsx`
- `frontend/src/pages/Sessions.tsx`
- `frontend/src/pages/Dashboard.tsx`
- `frontend/src/components/ChatPanel.tsx`

Gaps:

1. Data refresh is still partially poll-driven and causes UI jumps.
2. PM chat, session control, and agent allocation are not a single coherent workflow shell.
3. User cannot easily inspect/override role->agent assignment before dispatch.
4. Chat event rendering still leaks low-level tool payloads in awkward forms.

## 3.2 Orchestrator contract gaps

Current code locations:

- `internal/orchestrator/orchestrator.go`
- `internal/orchestrator/prompt.go`
- `internal/orchestrator/tools.go`

Gaps:

1. Mixed concerns: discussion generation, approvals, and execution routing still interleave loosely.
2. Stage semantics are present but need stricter persisted run state and transition evidence.
3. Human collaboration model (approve/modify/reassign/escalate) needs more explicit protocol.

## 3.3 TUI reliability gaps

Current code locations:

- `internal/tmux/gateway.go`
- `internal/session/manager.go`
- `internal/session/monitor.go`
- `internal/parser/parser.go`
- `frontend/src/components/Terminal.tsx`

Gaps:

1. Input submission semantics still fragile across different TUIs and prompt states.
2. Output stream + history replay can look duplicated/noisy in some terminal themes.
3. Session readiness and confirmation prompts need stronger deterministic handling.
4. Mobile websocket churn remains sensitive to connectivity/background transitions.

## 4. Product Direction (V3)

## 4.1 Core principle

Orchestrator is an operational manager with strict output envelope:

1. `discussion`: human-facing reasoning.
2. `commands`: planned and/or executed operational steps.
3. `confirmation`: required user choice when policy requires.
4. `state_update`: machine-readable changes (assignments, stage transitions, blockers).

## 4.2 User mental model

1. Project has stages: `plan -> build -> test`.
2. Each stage has lanes (worktrees) and roles.
3. Each role in a lane is bound to one concrete agent slot.
4. Every command sent to TUI is traceable and replayable.

## 5. Target Architecture

## 5.1 Control Plane Layers

1. **Execution State Machine**
   - Persisted `stage_runs`, `lane_runs`, `role_runs`.
   - Explicit transitions with evidence checks.

2. **Allocation Engine**
   - Turns playbook role constraints + live agent capacity into assignment proposals.
   - Supports user override and lock-in before execution.

3. **Session Runtime**
   - Session bootstrap/readiness handshake.
   - Serialized command queue per session (with ACK/timeout/retry policy).

4. **Interaction Layer**
   - Structured orchestrator envelope stream to UI.
   - Rendered confirmations, command cards, and lane status badges.

## 5.2 New/updated data entities

1. `project_runs` (active run metadata)
2. `stage_runs` (stage start/end/status/approval)
3. `lanes` (worktree lane, branch, spec path, dependencies)
4. `role_assignments` (lane+role -> agent slot)
5. `session_commands` (command_id, payload, ack, result, duration)
6. `session_events` (ready, prompt_detected, review_required, blocked)

## 5.3 API additions

1. `POST /api/projects/{id}/orchestrator/plan-proposal`
2. `POST /api/projects/{id}/orchestrator/assignments/preview`
3. `POST /api/projects/{id}/orchestrator/assignments/confirm`
4. `POST /api/sessions/{id}/commands` (queued command API)
5. `GET /api/sessions/{id}/commands/{cmd_id}`
6. `GET /api/projects/{id}/runs/current`
7. `POST /api/projects/{id}/runs/{run_id}/transition`

## 5.4 WebSocket event normalization

Define stable event taxonomy:

1. `run_state`
2. `stage_state`
3. `lane_state`
4. `assignment_state`
5. `session_command_ack`
6. `session_output_chunk`
7. `confirmation_required`
8. `confirmation_resolved`

This removes most polling from dashboard/pm chat/session list.

## 6. UI/Interaction Refactor

## 6.1 Desktop IA (new shell)

Three-pane shell:

1. Left: project/workspace list + agent pool snapshot.
2. Center: PM chat timeline (sticky composer + command/confirmation cards).
3. Right: foldable project operations panel:
   - stage graph
   - lane table
   - assignment matrix
   - progress/report artifacts

## 6.2 Session experience

1. Session list and terminal detail are split cleanly.
2. Terminal page shows:
   - session purpose
   - assigned role/lane
   - status badges (`working`, `needs-response`, `waiting_review`, `idle`)
   - quick actions (`respond`, `pause`, `handoff`, `close`)
3. Add output rendering options:
   - raw terminal
   - parsed transcript
   - filtered prompts/errors

## 6.3 Mobile strategy

1. Mobile scope is control-first, not full multi-terminal management.
2. Mobile pages:
   - project list
   - PM chat
   - approvals inbox
   - session alerts + minimal response controls
3. Keep heavy terminal editing on desktop/Tauri.

## 6.4 Command UX (inspired by Vibekanban)

Add command palette + slash actions:

1. `/plan`
2. `/allocate`
3. `/dispatch`
4. `/review-loop`
5. `/merge-lane`
6. `/report`
7. `/handoff`

Each command opens a parameterized UI sheet before execution.

## 7. TUI Injection and Output Reliability Plan

## 7.1 Command delivery contract

1. Session command queue enforces one in-flight command per session.
2. Command has phases: `queued -> sent -> acked -> completed|timeout|failed`.
3. Distinguish text vs key operations explicitly:
   - `send_text`
   - `send_key` (`C-m`, `C-c`, etc.)
4. Avoid implicit newline interpretation ambiguities.

## 7.2 Readiness and confirmation handling

1. After `create_session`, require `wait_for_session_ready` gate.
2. Read parser classifications + heuristics before task prompt dispatch.
3. Add configurable per-agent readiness signatures in registry.
4. Add confirmation policy map per agent/TUI prompt class.

## 7.3 Output pipeline hardening

1. Keep raw stream and parsed stream as separate channels.
2. Store rolling transcript chunks with sequence numbers.
3. Frontend applies idempotent append by `(session, seq)` to avoid duplication.
4. Introduce terminal rendering modes to mitigate visual noise from shell themes.

## 8. Orchestrator Behavioral Refactor

## 8.1 Strict manager behavior

Orchestrator may only:

1. plan/discuss,
2. call tools,
3. request confirmations,
4. report state.

It may not claim direct coding or shell execution.

## 8.2 Stage execution templates

For each stage, compile role cards into executable plans:

1. role purpose
2. required inputs
3. allowed agents
4. dispatch prompt template
5. completion evidence
6. escalation policy

## 8.3 Human-in-the-loop controls

1. User can edit assignment proposal before execution.
2. User can pin/ban specific agents for current run.
3. User can force stage pause/resume.
4. User can directly chat to any active session from PM context.

## 9. Performance and Scalability

## 9.1 Backend

1. Move dashboard/pm updates to ws events; reduce periodic full refresh.
2. Add query endpoints for incremental sync (`since` cursors).
3. Add bounded caches for project/session snapshots.

## 9.2 Frontend

1. Virtualize long message lists and session lists.
2. Keep composer fixed; isolate scroll containers.
3. Use optimistic state updates for approvals and assignment edits.

## 9.3 Observability

1. Metrics: command latency, command failure ratio, ws reconnect rate, stage dwell time.
2. Traces per orchestrator turn and per session command.
3. Structured logs with run_id/stage/lane/session correlation IDs.

## 10. Tauri/Desktop + Web Split

## 10.1 Recommendation

Yes: adopt a dual-surface strategy.

1. **Desktop (Tauri):** heavy terminal ops, multi-panel orchestration, persistent local state.
2. **Web/Mobile:** approvals, PM chat, status tracking, lightweight interventions.

## 10.2 Why this split

1. Terminal-heavy UX is significantly better in desktop shell.
2. Mobile reliability for long-running xterm streams is inherently weaker.
3. Keeps mobile fast and usable under unstable networks.

## 11. Implementation Roadmap (Slices)

## Slice A: Interaction Protocol Foundation (1-2 weeks)

1. Finalize orchestrator response envelope + ws event taxonomy.
2. Normalize frontend parsing/rendering around typed envelopes.
3. Remove raw JSON leakage in PM chat.

Acceptance:

1. PM chat never shows raw tool JSON except expandable debug blocks.
2. Confirm/modify/cancel actions are rendered as native controls.

## Slice B: Assignment-first Build Flow (1-2 weeks)

1. Add assignment preview/confirm endpoints.
2. Build assignment matrix UI in PM panel.
3. Enforce execution only after assignment confirmation.

Acceptance:

1. User sees exact role->agent mapping before dispatch.
2. Dashboard agent status reflects real assignment state in near real-time.

## Slice C: Session Command Queue + Ready Handshake (2 weeks)

1. Implement `session_commands` queue + ack states.
2. Replace implicit newline behavior with explicit text/key ops.
3. Add ready probe and per-agent readiness signatures.

Acceptance:

1. Command delivery success rate > 99% in local tests.
2. No command interleaving chaos across simultaneous sessions.

## Slice D: Stage/Lane Persistence and UI Graph (2 weeks)

1. Add stage/lane/run persistence tables.
2. Build stage graph + lane table components.
3. Hook orchestrator transitions to persisted evidence checks.

Acceptance:

1. A refresh/restart preserves execution state accurately.
2. Stage transitions are reproducible/auditable.

## Slice E: Mobile-lite + Desktop polish (1-2 weeks)

1. Mobile approvals inbox + lightweight session actions.
2. Optional Tauri shell prototype for desktop heavy mode.
3. UX polish pass (layout clarity, typography, iconography, feedback).

Acceptance:

1. Mobile no longer tries to be a full terminal IDE.
2. Desktop orchestrator flow can run full plan->build->test with clear UX.

## 12. Testing Strategy

1. Protocol tests for envelope and ws event schema compatibility.
2. Integration tests for assignment confirmation and stage transitions.
3. Replay tests for session command queue ordering/ack behavior.
4. Browser E2E tests for PM chat + assignment matrix + approval lifecycle.
5. Chaos tests for websocket disconnect/reconnect and session recovery.

## 13. Success Metrics

1. Orchestrator managed runs reaching build completion without manual low-level fixes.
2. Reduced PM chat confusion events (raw JSON leakage, ambiguous approvals).
3. Session command error rate and retry rate.
4. Agent utilization visibility correctness (dashboard vs actual sessions).
5. Mobile reconnect instability incidents.

## 14. Risks and Mitigations

1. **Risk:** Refactor scope too broad.
   - Mitigation: deliver by slices, each with hard acceptance gates.
2. **Risk:** Breaking existing orchestrator prompts.
   - Mitigation: version prompt contracts and keep backward-compatible parser window.
3. **Risk:** Terminal behavior differences across agent CLIs.
   - Mitigation: per-agent readiness/interaction profiles in registry.

## 15. Immediate Next Step

Start Slice A only:

1. freeze orchestrator envelope schema,
2. implement typed renderer in PM chat,
3. add event taxonomy adapter in backend,
4. ship with regression tests before touching allocation/session runtime internals.
