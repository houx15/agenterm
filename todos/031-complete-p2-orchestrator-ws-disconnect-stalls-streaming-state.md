---
status: complete
priority: p2
issue_id: "031"
tags: [code-review, reliability, frontend, websocket]
dependencies: []
---

# PM Chat streaming state can stall after websocket disconnect

The PM Chat websocket hook can leave the UI in a permanent streaming state when the orchestrator socket disconnects mid-response.

## Problem Statement

When `/ws/orchestrator` drops during an active response, `isStreaming` is never cleared in the frontend hook. Users can see a stuck "PM is working..." indicator and an unfinished placeholder assistant message, even though no more tokens can arrive from that socket. This creates incorrect runtime status and a confusing chat experience.

## Findings

- `frontend/src/hooks/useOrchestratorWS.ts`: `ws.onclose` updates only `connectionStatus` and reconnection timers; it does not call `finishActiveAssistant()` or set `isStreaming(false)`.
- `ws.onerror` also only sets connection status and does not resolve active stream state.
- The hook sets `isStreaming(true)` in `send()` before websocket delivery is guaranteed; if close/error occurs before `done`, UI remains inconsistent.

## Proposed Solutions

### Option 1: Finalize active stream on close/error

**Approach:** In `onclose` and `onerror`, flush buffered tokens and call `finishActiveAssistant()` (or equivalent safe finalizer).

**Pros:**
- Smallest change and lowest risk
- Keeps UI consistent with connection state

**Cons:**
- Partial responses remain partial (expected)

**Effort:** Small

**Risk:** Low

---

### Option 2: Add explicit interrupted system message

**Approach:** Option 1 plus append a non-user message like "Connection lost before response completed".

**Pros:**
- Better user clarity on failure mode

**Cons:**
- Adds UX copy and message-state behavior to maintain

**Effort:** Small

**Risk:** Low

---

### Option 3: Implement request resume/replay

**Approach:** Track pending requests and replay them after reconnect.

**Pros:**
- Stronger recovery behavior

**Cons:**
- Larger protocol and state complexity
- Potential duplicate side effects on orchestrator actions

**Effort:** Large

**Risk:** High

## Recommended Action
Implemented Option 1. Finalize the active assistant stream in websocket `onclose` and `onerror` handlers so `isStreaming` is always cleared on interruption.

## Technical Details

**Affected files:**
- `frontend/src/hooks/useOrchestratorWS.ts`

**Related components:**
- `frontend/src/components/ChatPanel.tsx`
- `frontend/src/pages/PMChat.tsx`

**Database changes (if any):**
- No

## Resources

- **Spec:** `docs/TASK-17-pm-chat-ui.md`
- **Branch:** `feature/pm-chat-ui`

## Acceptance Criteria

- [x] If websocket closes/errors during active streaming, `isStreaming` is cleared immediately.
- [x] No permanent "PM is working..." indicator after disconnect.
- [x] Existing token buffering behavior remains intact for normal `token` + `done` flow.
- [x] Frontend build passes.

## Work Log

### 2026-02-18 - Review Discovery

**By:** Codex

**Actions:**
- Reviewed PM chat websocket lifecycle and streaming state management.
- Identified disconnect/error path missing stream finalization.
- Documented remediations with effort/risk tradeoffs.

**Learnings:**
- Current implementation handles normal `done` completion, but not connection-interrupted completion.

### 2026-02-18 - Fix Implemented

**By:** Codex

**Actions:**
- Updated `frontend/src/hooks/useOrchestratorWS.ts` to call `finishActiveAssistant()` in both `ws.onclose` and `ws.onerror`.
- Built frontend and ran `go test ./internal/api -count=1` to validate no regressions.

**Learnings:**
- Stream finalization helper is safe to invoke across both done and disconnect paths.

## Notes

- This is a reliability issue, not a functional blocker for happy-path usage.
