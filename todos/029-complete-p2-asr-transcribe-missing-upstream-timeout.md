---
status: complete
priority: p2
issue_id: "029"
tags: [code-review, reliability, api]
dependencies: []
---

# ASR Transcribe Missing Upstream Timeout

ASR transcription forwards request context directly to Volcengine websocket calls without applying a server-side timeout budget. A stalled upstream can keep handler goroutines and connections open too long.

## Problem Statement

Long-running or hanging upstream ASR sessions can reduce API availability under load. The handler should enforce deterministic timeout boundaries.

## Findings

- `internal/api/asr.go:82` calls `h.asrTranscriber.Transcribe(r.Context(), ...)` with no timeout wrapper.
- `internal/api/asr_volc.go:116` loops on provider responses until `IsLastPackage`, with no local timeout fallback.

## Proposed Solutions

### Option 1: Add Request Timeout In Handler

**Approach:** Wrap `r.Context()` with `context.WithTimeout` (e.g., 20-30s) before calling transcriber.

**Pros:**
- Minimal code change.
- Caps stuck request duration.

**Cons:**
- One timeout fits all request sizes.

**Effort:** Small

**Risk:** Low

---

### Option 2: Configurable Timeout + Metrics

**Approach:** Add configurable ASR timeout and emit timeout counters/latency metrics.

**Pros:**
- Better operational control and observability.

**Cons:**
- Slightly larger change.

**Effort:** Medium

**Risk:** Low

## Recommended Action


## Technical Details

- `internal/api/asr.go`
- `internal/api/asr_volc.go`

## Resources

- `docs/TASK-17-pm-chat-ui.md`

## Acceptance Criteria

- [ ] ASR requests have a bounded server-side timeout.
- [ ] Timeout errors are returned as clear API errors.
- [ ] Tests cover timeout path.

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Inspected ASR handler and transcriber control flow.
- Verified no timeout budget enforcement exists.

**Learnings:**
- Current flow assumes provider responsiveness and can hang until client disconnect.

## Notes

- Priority is reliability and resource control.
