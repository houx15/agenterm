---
status: complete
priority: p2
issue_id: "009"
tags: [code-review, architecture, frontend, server]
dependencies: []
---

# Return 404 for Missing Static Assets Instead of SPA HTML Fallback

## Problem Statement

The SPA fallback currently serves `index.html` for any missing non-API/non-WS path, including missing static assets. Requests like `/assets/missing.js` should return 404, not HTML, to avoid confusing browser/runtime failures and cache diagnostics.

## Findings

- In static serving logic, any `fs.Stat` miss falls through to `index.html` (`internal/server/server.go:44` to `internal/server/server.go:53`).
- There is no path-type distinction (route path vs static file path with extension).
- This behavior can surface as MIME/runtime errors when a script or stylesheet URL is wrong.

## Proposed Solutions

### Option 1: Fallback only for route-like paths (no file extension)

**Approach:** If requested path has extension (e.g., `.js`, `.css`, `.png`) and missing from `subFS`, return `http.NotFound`; otherwise serve `index.html`.

**Pros:**
- Standard SPA behavior
- Improves debuggability and caching correctness

**Cons:**
- Slightly more routing logic

**Effort:** Small

**Risk:** Low

---

### Option 2: Reserve explicit frontend base routes and fallback only there

**Approach:** Maintain allowlist of app routes for fallback, 404 all others.

**Pros:**
- Tight control over fallback behavior

**Cons:**
- Requires route maintenance as app grows

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Keep current behavior

**Approach:** Continue serving `index.html` on all misses.

**Pros:**
- Simplest logic

**Cons:**
- Static asset failures are masked by HTML fallback
- Harder incident/debug workflows for broken asset paths

**Effort:** None

**Risk:** Medium

## Recommended Action
Implemented Option 1: route-like paths still fallback to SPA index, but missing asset-like paths (with extension) now return 404.

## Technical Details

**Affected files:**
- `internal/server/server.go:44`

## Resources

- **Commit under review:** `358083d`

## Acceptance Criteria

- [x] Missing static asset requests return HTTP 404
- [x] Client-side route refreshes still return `index.html`
- [x] Existing `/api/*` and `/ws` handling remains unchanged
- [x] Manual check verifies `/sessions` fallback works and `/assets/does-not-exist.js` returns 404

## Work Log

### 2026-02-17 - Review finding creation

**By:** Codex

**Actions:**
- Examined SPA fallback control flow in server
- Confirmed non-route/static path misses currently collapse to `index.html`

**Learnings:**
- Asset-aware fallback logic is needed to balance SPA UX and operational debuggability

### 2026-02-17 - Fix implemented

**By:** Codex

**Actions:**
- Updated `internal/server/server.go` fallback logic to return `http.NotFound` when a missing path has a file extension
- Kept SPA fallback for route-like paths without extensions
- Verified backend test suite still passes

**Learnings:**
- Extension-based branching is a simple and effective SPA/static asset split

## Notes

- This is important for production troubleshooting and correct client cache/error behavior.
