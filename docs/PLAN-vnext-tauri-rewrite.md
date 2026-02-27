---
title: "PLAN: agenterm vNext tauri-first rewrite"
status: active
date: 2026-02-26
owner: platform
---

# Agenterm vNext Tauri-First Rewrite Plan

## 1. Final Product Direction

Build a desktop-first system for orchestrated multi-agent development:

1. Primary app: Tauri desktop.
2. Companion app: lightweight mobile web (status, approvals, reports only).
3. No full-featured browser-first terminal surface in vNext.

This is a rewrite plan, not an incremental web polish plan.

## 2. Why This Direction

Terminal-heavy orchestration needs:

1. stable PTY lifecycle,
2. deterministic command delivery,
3. stream replay and recovery,
4. low-latency local control.

Desktop runtime is the right default for those constraints. Mobile remains control-plane lite.

## 3. Core Capability Target

The system must complete this loop with minimal human interruption:

1. brainstorm,
2. plan,
3. split worktrees,
4. build in parallel with TDD and code/review loops,
5. post-test and summarize.

Human interrupts only for:

1. stage gates,
2. policy exceptions,
3. true blockers.

## 4. Orchestrator Contract (Strict)

Orchestrator is manager only. It can:

1. produce discussion for user,
2. schedule and control sessions,
3. request/resolve confirmations,
4. report state transitions and evidence.

It cannot claim direct coding or shell execution outside managed sessions.

## 5. Runtime Architecture

## 5.1 Project-first isolation

Every project has isolated:

1. run state,
2. stage/lane graph,
3. session set,
4. command queue,
5. artifacts and reports.

## 5.2 Session runtime

1. One in-flight command per session queue.
2. Command states: `queued -> sent -> acked -> completed|failed|timeout`.
3. Explicit operations: `send_text`, `send_key`, `resize`, `interrupt`, `close`.
4. Ready handshake before dispatching role prompts.

## 5.3 Event model

Typed event taxonomy:

1. `run_state`,
2. `stage_state`,
3. `lane_state`,
4. `assignment_state`,
5. `session_command`,
6. `session_output`,
7. `confirmation_required`,
8. `confirmation_resolved`,
9. `exception`.

All UI views consume typed events, not raw tool JSON.

## 5.4 Agent pool control

1. Real-time slots: `idle`, `working`, `reviewing`, `planner`, `offline`.
2. User-configurable role constraints before dispatch.
3. Assignment matrix must be confirmed before execution.
4. Orchestrator can allocate only inside confirmed constraints.

## 6. UX Model

## 6.1 Desktop

Three-pane workspace:

1. Left: projects + agent pool + demand entry.
2. Center: PM chat timeline and orchestrator actions.
3. Right: stage graph, lanes, assignments, reports.

## 6.1.1 UI Refactor Update (2026-02-27)

Implemented direction in current frontend:

1. Workspace is now the default entry and primary working surface.
2. Three-pane desktop structure is active:
   - left: project list + project agent terminals,
   - center: orchestrator chat timeline/composer,
   - right: project overview (roadmap, details, progress, exceptions, task group).
3. Demand pool is no longer treated as a primary standalone workflow page; it is opened from PM chat (modal flow).
4. Dashboard, Session Console, Connect Mobile, and Settings are utility navigation items.
5. Settings scope is intentionally simplified to Agent Registry only (no playbook designer in vNext UI baseline).

Remaining UI work under this plan:

1. replace any remaining raw tool/result leakage with structured timeline cards everywhere,
2. harden session console terminal UX to Superset-style interaction quality,
3. finalize mobile companion views for approvals, blocker alerts, and report timeline.

Session view:

1. list/detail split,
2. xterm-only terminal rendering,
3. role/lane badges and quick controls,
4. parsed transcript and raw output tabs.

## 6.2 Mobile companion

Pages:

1. project list,
2. approvals inbox,
3. progress/report timeline,
4. blocker alerts.

No full multi-terminal editing on mobile.

## 7. Workflow Engine (Built-in Packs)

First-class templates:

1. `pairing-coding`,
2. `tdd-coding`,
3. `compound-engineering`.

Each template defines:

1. stage enablement,
2. role contracts,
3. required inputs,
4. action policies,
5. evidence gates,
6. handoff rules.

## 8. Rewrite Slices

## Slice 1: Foundation

1. Tauri shell + project-first state boundaries.
2. typed event bus and schema package.
3. replacement PM timeline renderer for structured envelopes.

Acceptance:

1. no raw JSON leakage in PM chat,
2. project-scoped events replay correctly after restart.

## Slice 2: Terminal runtime

1. session command queue and ack model,
2. readiness probes,
3. reliable output replay and scrollback restoration.

Acceptance:

1. command ordering deterministic under parallel load,
2. reconnect preserves visible terminal continuity.

## Slice 3: Orchestrator engine

1. stage state machine persistence,
2. assignment preview/confirm APIs,
3. policy gates and exception inbox.

Acceptance:

1. orchestrator cannot execute outside approved command scope,
2. stage transitions are evidence-backed and auditable.

## Slice 4: Workflow and quality loops

1. coders/reviewers/testers role execution loops,
2. review-fix-review until pass,
3. post-test and final summary artifacts.

Acceptance:

1. one project can complete `plan -> build -> test` end-to-end.

## Slice 5: Mobile companion

1. approvals/status/report web app,
2. push-friendly alerting for blockers and gate requests.

Acceptance:

1. mobile works as low-friction control plane for long-running projects.

## 9. Merge Decision With Existing Plan

Reference doc:

- `docs/plans/2026-02-26-feat-agenterm-v3-ui-orchestrator-terminal-plan.md`

Decision:

1. Keep and reuse its external research and reliability findings.
2. Supersede its incremental "keep current web as main surface" direction.
3. Treat this document as the authoritative strategy for implementation sequencing.

Operationally:

1. old plan becomes historical context,
2. this vNext plan drives execution backlog and acceptance criteria.
