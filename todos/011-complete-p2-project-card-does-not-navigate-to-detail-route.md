---
status: complete
priority: p2
issue_id: "011"
tags: [code-review, quality, frontend, ux]
dependencies: []
---

# Project Card Does Not Navigate To Detail Route

Project card clicks currently update query params on `/` and render an inline details block, but do not navigate to a project detail page/route.

## Problem Statement

TASK-14 acceptance says "Clicking project/session navigates to detail view." Session click navigates to `/sessions`, but project click does not navigate away from dashboard. This is a behavior mismatch with the spec wording.

## Findings

- `frontend/src/pages/Dashboard.tsx:297` uses `setSearchParams({ project: ... })` for project card click.
- `frontend/src/pages/Dashboard.tsx:362` renders details inline instead of route transition.
- No `/projects/:id` route exists in `frontend/src/App.tsx`.

## Proposed Solutions

### Option 1: Implement project detail route

**Approach:** add `/projects/:projectId` page and navigate there from project cards.

**Pros:**
- Fully aligned with "navigate to detail view"
- Scales for richer project detail UX

**Cons:**
- Requires new page and route wiring

**Effort:** Medium

**Risk:** Medium

---

### Option 2: Keep inline detail but update spec copy/acceptance

**Approach:** treat inline expansion as detail view and change spec/acceptance wording accordingly.

**Pros:**
- No extra routing complexity
- Minimal engineering effort

**Cons:**
- Changes contract instead of implementation

**Effort:** Small

**Risk:** Medium

## Recommended Action

Implemented route-based project detail navigation at `/projects/:projectId` and switched dashboard cards to navigate there.

## Technical Details

**Affected files:**
- `frontend/src/pages/Dashboard.tsx:297`
- `frontend/src/pages/Dashboard.tsx:362`
- `frontend/src/App.tsx:31`

## Resources

- Spec: `docs/TASK-14-dashboard-ui.md`
- Commits reviewed: `d1c0404`, `2169644`

## Acceptance Criteria

- [ ] Project card click behavior is explicitly route-based OR spec text is updated to match inline detail behavior
- [ ] Session navigation behavior remains intact
- [ ] QA can verify deterministic "detail view" behavior for both entities

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Traced project click handler and route table
- Compared behavior against acceptance criteria wording

**Learnings:**
- Implementation provides details, but not route navigation for projects

## Notes

- Choose Option 1 if long-term IA includes project-level pages.
