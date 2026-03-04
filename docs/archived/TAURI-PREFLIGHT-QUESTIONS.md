---
title: "TAURI vNext Preflight Questions"
status: draft
date: 2026-02-26
owner: platform
---

# Goal

Lock all high-impact decisions before Slice 1 (`foundation`) so implementation can run with minimal backtracking.

## 1. Already Decided (from `docs/DECISION-vnext.md`)

1. Stages and definition of done are explicit (`brainstorm -> plan -> build -> test -> summarize`).
2. Plan outputs must include worktree graph, spec docs, role assignment proposal.
3. Build merge gate: tests green + reviewer sign-off + no unresolved critical findings.
4. Safety boundary: outside-project write/delete requires approval; project-local edits can be autonomous.
5. In-app notifications only.
6. Core metrics: cycle time, interruption count, failure rate.
7. Diagnostic bundle is required on failures.
8. Worktree naming preference: `feature/*`, `bug/*`.

## 2. Open Product Questions (must lock before Slice 1)

1. Scope of vNext desktop release:
- Option A: planner/build/test + run visibility only.
- Option B: include full settings editing (agent registry/playbook editor) in first release.
- Proposed default: A (faster path to stable orchestrator loop).
- answer: I'd like to select Option B. but we can finish A first, and we test it. then move on to complement B. (besides, why we called our app vNext. I still like it to be called agenTerm)


2. Project concurrency model:
- Should multiple projects run orchestration simultaneously, or single active run globally?
- Proposed default: multi-project allowed, per-project isolation + global agent pool caps.
- answer: yes multi-project allowed. default is good.


1. Human approval UX semantics:
- Confirm/Reject only, or Confirm/Edit/Reject (edit means user can modify assignment/command plan first)?
- Proposed default: Confirm/Edit/Reject.
- answer: I agree with you.

1. Demand pool integration:
- Desktop left nav entry should remain read-only shortcut, with edits only inside PM chat project view (as decided before). Confirm still valid?
- Proposed default: yes, keep this behavior.

## 3. Open Runtime Questions (must lock before Slice 2)

1. Execution substrate for agenTerm:
- Keep tmux as primary runtime now, or move directly to native PTY runtime?
- answer: use PTY.

2. Session readiness contract:
- Ready signal should be: parser prompt detection, or first-output + timeout fallback?
- Proposed default: prompt detection primary, first-output/timeout fallback.

3. Command queue retry policy:
- On timeout/failure, retry count per command?
- Proposed default: 2 retries with exponential backoff, then emit exception.

4. Session output persistence window:
- How much scrollback per session should be persisted in DB (or file store)?
- Proposed default: 2,000 lines/session + compressed archive file per stage.

## 4. Open Orchestrator Questions (must lock before Slice 3)

1. Autonomy model for handoffs:
- Auto-advance across role handoffs after evidence checks, or always request user confirmation?
- Proposed default: auto-advance within stage; user approval required for stage transitions only.

2. Agent assignment authority:
- Orchestrator proposes and auto-assigns within constraints, or always waits user click-confirm for each stage assignment matrix?
- Proposed default: always require one confirmation per stage assignment matrix.

3. Confirmation handling inside TUIs:
- When a worker TUI asks for interactive confirmation, should orchestrator auto-answer via policy patterns?
- Proposed default: policy-driven auto-answer for safe patterns, otherwise raise confirmation card.

4. Forbidden actions policy location:
- Central policy in backend only, or duplicated in playbook + backend?
- Proposed default: backend as source of truth; playbook can only tighten, never loosen.

## 5. Open Workflow Questions (must lock before Slice 4)

1. TDD enforcement mode:
- Strict in `tdd-coding`, advisory in other templates?
- Proposed default: strict in `tdd-coding`, advisory elsewhere.

2. Review loop termination:
- Reviewer marks pass/fail; maximum loops per lane is 20 (already decided). On limit reached, what happens?
- Proposed default: auto-escalate to human with diagnostic bundle and block lane.

3. Commit granularity:
- One commit per accepted fix cycle vs squashed per lane completion?
- Proposed default: one commit per accepted fix cycle, merge squash optional at stage close.

4. Test stage ownership:
- Dedicated tester role only, or allow reviewer fallback if no tester available?
- Proposed default: tester preferred, reviewer fallback permitted.

## 6. Open UX Questions (must lock before Slice 1 desktop shell)

1. Default layout widths:
- left/center/right split percentages?
- Proposed default: 22 / 48 / 30.
- answer: adjustable. can fold/expand.

1. Session terminal placement:
- Separate full-page terminal route, or dockable panel inside workspace?
- Proposed default: separate route with fast back navigation.

1. Chat timeline command cards:
- Show both planned and executed commands, or executed only?
- Proposed default: executed by default, planned toggle on.

1. Keyboard-first shortcuts:
- Minimum set for vNext: project switch, open approvals, jump to active session, send command.
- Proposed default: implement this minimum set in Slice 1.


besides: 
- we can set light/dark of the whole app.
- we only need to consider mac.


## 7. Open Mobile Questions (must lock before Slice 5)

1. Authentication for mobile companion:
- Reuse existing token login only, or add QR pair from desktop?
- Proposed default: token login now, QR pair later.
- answer: QR pair.

2. Mobile message scope:
- PM discussion only, or include lightweight command cards and approval actions?
- Proposed default: PM discussion + approvals + progress only (no direct command dispatch).

## 8. Data and Migration Questions

1. Legacy web data compatibility:
- Should existing DB schema be migrated in-place, or new vNext tables with compatibility adapters?
- Proposed default: additive vNext tables + adapters, no destructive migration.
- answer: we don't need to consider any migration - the old one, just destroy it, never mind. I'm the only user.

2. Event replay cursor:
- Per project stream cursor only, or per pane/channel cursors?
- Proposed default: per project + per channel cursor.

3. Audit retention:
- How long to retain command/event audit records?
- Proposed default: 30 days hot, archived thereafter.

## 9. Testing Questions (must lock before implementation finishes)

1. E2E harness target:
- Backend API-level deterministic tests first, then UI automation?
- Proposed default: yes.

2. Golden scenarios required:
1. Plan-only project.
2. Plan -> build with review loops and one rejection cycle.
3. Build conflict + resolution.
4. Build -> test -> summarize.
5. Recovery after service restart.

3. Acceptance gate for release:
- All golden scenarios pass + no raw JSON in PM timeline + deterministic command ordering.

## 10. One-shot Decision Format

Reply in this format to unblock Slice 1 quickly:

1. Accept defaults for all sections except: [list ids]
2. Overrides:
- Q2.2: ...
- Q3.1: ...
- Q6.1: ...

If no overrides, reply: `Accept all defaults; start Slice 1.`
