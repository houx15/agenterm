---
status: complete
priority: p2
issue_id: "010"
tags: [code-review, quality, frontend, dashboard]
dependencies: []
---

# Filter Dashboard To Active Projects

Project cards currently render every project returned by `/api/projects`, while the spec calls for showing active projects and an active-project count.

## Problem Statement

The dashboard can over-report project count and include inactive/archived projects. This breaks the TASK-14 acceptance intent: "Dashboard shows all active projects with progress." 

## Findings

- `frontend/src/pages/Dashboard.tsx:187` maps over all `projects` without status filtering.
- `frontend/src/pages/Dashboard.tsx:289` labels the count as "active" using `projectSummaries.length`, which currently includes non-active projects.
- `docs/TASK-14-dashboard-ui.md` specifies active projects in both objective and layout text.

## Proposed Solutions

### Option 1: Filter at render layer

**Approach:** derive `activeProjects = projects.filter(p => normalizeStatus(p.status) === 'active')` and use that for summaries/count.

**Pros:**
- Minimal, localized change
- Keeps API contract unchanged

**Cons:**
- Requires agreement on what statuses are "active"

**Effort:** Small

**Risk:** Low

---

### Option 2: Add explicit status filter in API query (if backend supports)

**Approach:** request only active projects from backend endpoint.

**Pros:**
- Less client-side post-processing
- Scales better with many projects

**Cons:**
- Depends on backend capability/API change

**Effort:** Medium

**Risk:** Medium

## Recommended Action

Implemented in dashboard: active projects are filtered in the main grid and inactive projects are still accessible via a dedicated toggle section.

## Technical Details

**Affected files:**
- `frontend/src/pages/Dashboard.tsx:187`
- `frontend/src/pages/Dashboard.tsx:289`

## Resources

- Spec: `docs/TASK-14-dashboard-ui.md`
- Commits reviewed: `d1c0404`, `2169644`

## Acceptance Criteria

- [ ] Dashboard excludes inactive/archived projects from project cards
- [ ] Project header count reflects only active projects
- [ ] Progress and active-agent metrics still compute correctly after filtering

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Compared TASK-14 acceptance text to dashboard implementation
- Verified unfiltered project usage in summary/count code paths

**Learnings:**
- Current UI labeling says "active" but source set is all projects

## Notes

- Confirm canonical active status values (`active`, `running`, etc.) before final patch.
