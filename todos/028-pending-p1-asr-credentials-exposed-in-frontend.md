---
status: pending
priority: p1
issue_id: "028"
tags: [code-review, security, frontend, api]
dependencies: []
---

# ASR Credentials Exposed In Frontend

Volcengine `accessKey` is collected and stored in browser `localStorage`, then sent from frontend on every transcription call. This exposes long-lived credentials to any XSS, browser extension, or local machine compromise.

## Problem Statement

The current ASR integration requires users to paste provider secrets into the web UI and persists them in local storage. This violates least-privilege and secret-management expectations, even for local deployments.

## Findings

- `frontend/src/settings/asr.ts:1` stores `appID` and `accessKey` in `localStorage` under `agenterm_asr_settings`.
- `frontend/src/hooks/useSpeechToText.ts:94` reads those secrets in-browser.
- `frontend/src/api/client.ts:86` and `frontend/src/api/client.ts:87` sends `app_id` and `access_key` in multipart form body.
- Backend already supports env fallback (`VOLCENGINE_APP_ID`, `VOLCENGINE_ACCESS_KEY`) in `internal/api/asr.go:39` and `internal/api/asr.go:43`, so client-side secret storage is avoidable.

## Proposed Solutions

### Option 1: Server-Side Credentials Only

**Approach:** Remove credential fields from frontend settings and have backend read ASR credentials only from environment/secret manager.

**Pros:**
- Eliminates browser-side secret exposure.
- Simplifies client API and UX.

**Cons:**
- Requires server env configuration.

**Effort:** Small

**Risk:** Low

---

### Option 2: Scoped Short-Lived Token Exchange

**Approach:** Keep provider credentials server-side and mint short-lived scoped tokens for ASR calls.

**Pros:**
- Strong security posture.
- Supports per-user usage controls.

**Cons:**
- Additional backend complexity.

**Effort:** Medium

**Risk:** Medium

## Recommended Action


## Technical Details

- `frontend/src/settings/asr.ts`
- `frontend/src/hooks/useSpeechToText.ts`
- `frontend/src/api/client.ts`
- `internal/api/asr.go`

## Resources

- `docs/TASK-17-pm-chat-ui.md`
- Branch: `feature/pm-chat-ui`

## Acceptance Criteria

- [ ] ASR provider secrets are not stored in browser storage.
- [ ] Frontend no longer transmits long-lived provider secrets.
- [ ] Backend reads credentials from secure server-side configuration.
- [ ] ASR transcription still works end-to-end in local mode.

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed ASR frontend and backend data flow.
- Confirmed secret persistence in localStorage and request payload.
- Confirmed backend env-based credential path already exists.

**Learnings:**
- Existing implementation can be hardened without changing ASR protocol logic.

## Notes

- This is a merge-blocking security concern due to credential exposure.
