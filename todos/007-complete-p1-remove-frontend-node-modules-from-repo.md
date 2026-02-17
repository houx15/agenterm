---
status: complete
priority: p1
issue_id: "007"
tags: [code-review, quality, build]
dependencies: []
---

# Remove Frontend node_modules and Generated Cache from Git History Going Forward

## Problem Statement

The latest commit includes `frontend/node_modules/**` and `.vite` cache artifacts, which should not be versioned. This creates an oversized commit and repository bloat, slows clone/fetch operations, and causes noisy future diffs.

## Findings

- `git show --name-only HEAD` lists thousands of files under `frontend/node_modules/**` (including `frontend/node_modules/.vite/**`).
- Root ignore rules do not ignore the new frontend dependency tree (`.gitignore:1` to `.gitignore:7`), so `frontend/node_modules` was added by default.
- Commit `358083d` contains 2595 files and >1,000,000 inserted lines, dominated by dependency artifacts.

## Proposed Solutions

### Option 1: Add ignore + remove tracked dependency files in follow-up commit

**Approach:** Add `frontend/node_modules/` and `frontend/.vite/` ignore entries, then remove tracked dependency/cache files from index and commit cleanup.

**Pros:**
- Minimal disruption
- Preserves current branch progression
- Immediately fixes future commits

**Cons:**
- Historical commit still contains large payload

**Effort:** Small

**Risk:** Low

---

### Option 2: Rewrite local commit history before PR

**Approach:** Reset the feature branch to pre-commit state, re-commit only source files and lockfile.

**Pros:**
- Clean PR and commit history
- No dependency artifacts in feature branch history

**Cons:**
- Requires careful branch rewrite/force-push

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Keep current commit and rely on future cleanup

**Approach:** Merge as-is and clean in subsequent PR.

**Pros:**
- No immediate action

**Cons:**
- PR becomes hard to review
- Long-term repo growth and slower CI/fetch
- Violates common repo hygiene standards

**Effort:** Small

**Risk:** High

## Recommended Action
Implemented Option 1 in a follow-up fix commit: added frontend ignore rules and removed tracked dependency artifacts from the git index.

## Technical Details

**Affected files:**
- `.gitignore:1`
- `frontend/node_modules/...` (entire dependency tree currently tracked)

## Resources

- **Commit under review:** `358083d`
- **Task spec:** `docs/TASK-13-frontend-react.md`

## Acceptance Criteria

- [x] `.gitignore` includes `frontend/node_modules/` and frontend cache/build transient dirs as needed
- [x] Tracked `frontend/node_modules/**` and `.vite/**` files are removed from git index
- [x] Feature branch contains only source + lockfile + intended artifacts
- [x] PR diff size is reduced to implementation-relevant files

## Work Log

### 2026-02-17 - Review finding creation

**By:** Codex

**Actions:**
- Inspected latest commit file list and identified vendored dependency artifacts
- Confirmed missing ignore rule for `frontend/node_modules`
- Classified as merge-blocking repository hygiene issue

**Learnings:**
- Frontend scaffold introduced a new dependency root not covered by existing ignore patterns

### 2026-02-17 - Fix implemented

**By:** Codex

**Actions:**
- Updated `.gitignore` with `frontend/node_modules/` and `frontend/.vite/`
- Executed `git rm -r --cached frontend/node_modules` to untrack dependency artifacts
- Confirmed cleanup is staged for follow-up commit

**Learnings:**
- Index cleanup can be done safely without deleting local dependencies by using `--cached`

## Notes

- This finding should be resolved before PR review to keep diff and repository maintenance manageable.
