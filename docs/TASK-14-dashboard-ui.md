# Task: dashboard-ui

## Context
The Dashboard is the landing page of AgenTerm — a global overview of all projects, active sessions, and recent activity. It gives users a bird's-eye view before diving into specific sessions or projects.

## Objective
Build the Dashboard page showing: project overview cards, active session status grid, recent task completions, and resource usage summary.

## Dependencies
- Depends on: TASK-13 (frontend-react), TASK-08 (rest-api)
- Branch: feature/dashboard-ui
- Base: main (after TASK-13 merge)

## Scope

### Files to Create
- `frontend/src/pages/Dashboard.tsx` — Main dashboard page (replace placeholder)
- `frontend/src/components/ProjectCard.tsx` — Project summary card
- `frontend/src/components/SessionGrid.tsx` — Active sessions status grid
- `frontend/src/components/ActivityFeed.tsx` — Recent activity list

### Files to Modify
- `frontend/src/api/client.ts` — Add API calls for projects, sessions, tasks

### Files NOT to Touch
- Go backend — Dashboard is purely frontend, consuming existing APIs

## Implementation Spec

### Step 1: Dashboard layout
```
┌──────────────────────────────────────────┐
│  Dashboard                               │
├──────────────────────────────────────────┤
│  Projects (3 active)                     │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │ Project1 │ │ Project2 │ │   + New  │   │
│  │ 3/5 done │ │ 1/3 done │ │  Project │   │
│  │ 2 agents │ │ 1 agent  │ │         │   │
│  └─────────┘ └─────────┘ └─────────┘   │
├──────────────────────────────────────────┤
│  Active Sessions                         │
│  ┌──┐ ┌──┐ ┌──┐ ┌──┐ ┌──┐              │
│  │🟢│ │🟢│ │🟡│ │🔴│ │⚪│              │
│  │CC│ │CX│ │CC│ │GM│ │CC│              │
│  └──┘ └──┘ └──┘ └──┘ └──┘              │
├──────────────────────────────────────────┤
│  Recent Activity                         │
│  • task-auth completed (2m ago)         │
│  • agent codex started on task-api      │
│  • review passed for task-models        │
└──────────────────────────────────────────┘
```

### Step 2: Project cards
- Show project name, status, task progress (X/Y done)
- Active agent count
- Click to navigate to project detail
- "+ New Project" card to create project

### Step 3: Session status grid
- Compact grid of session status dots
- Each dot shows: agent type abbreviation, status color
- Click to jump to session terminal
- Grouped by project

### Step 4: Activity feed
- Recent events: task status changes, session starts/stops, commits
- Timestamp + description
- Fetched from API or received via WebSocket events

### Step 5: Data fetching
- Fetch on mount: GET /api/projects, GET /api/sessions
- Subscribe to WebSocket events for real-time updates
- Auto-refresh every 30s as fallback

## Acceptance Criteria
- [ ] Dashboard shows all active projects with progress
- [ ] Session grid shows all active sessions with status
- [ ] Activity feed shows recent events
- [ ] Clicking project/session navigates to detail view
- [ ] Real-time updates via WebSocket

## Notes
- Keep it simple initially — can be enhanced later
- Empty state: show helpful onboarding message when no projects exist
- Consider using CSS Grid for responsive card layout
