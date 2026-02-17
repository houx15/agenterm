---
status: complete
priority: p2
issue_id: "004"
tags: [code-review, api, sessions, config]
dependencies: []
---

# Session Records Ignore Configured tmux Session Name

Session records are persisted with `tmux_session_name: "agenterm"` instead of the configured session from runtime config.

## Problem Statement

The app supports configurable tmux session names (`cfg.TmuxSession`). Persisting a hardcoded name can break observability and tooling relying on accurate session metadata.

## Findings

- `internal/api/sessions.go:79` hardcodes `TmuxSessionName: "agenterm"`.
- Runtime gateway is created from config (`cmd/agenterm/main.go:49`), so DB metadata may diverge from reality.

## Proposed Solutions

### Option 1: Pass configured tmux session into API handler

**Approach:** Extend router/handler config and set `TmuxSessionName` from config.

**Pros:**
- Correct, explicit source of truth
- Minimal runtime overhead

**Cons:**
- Requires constructor signature changes

**Effort:** Small

**Risk:** Low

---

### Option 2: Infer from gateway state

**Approach:** Add gateway interface method to expose current session name.

**Pros:**
- No extra config plumbed through router

**Cons:**
- Requires tmux package interface changes

**Effort:** Medium

**Risk:** Medium

## Recommended Action


## Technical Details

Affected files:
- `internal/api/sessions.go:79`
- `cmd/agenterm/main.go:81`

## Resources

- Spec: `docs/TASK-08-rest-api.md`

## Acceptance Criteria

- [ ] `sessions.tmux_session_name` matches configured tmux session
- [ ] Unit/API tests validate non-default configured session values

## Work Log

### 2026-02-17 - Review Discovery

**By:** Codex

**Actions:**
- Compared session persistence fields against runtime config wiring
- Identified hardcoded value divergence

**Learnings:**
- Metadata drift is subtle but problematic for downstream orchestration.
