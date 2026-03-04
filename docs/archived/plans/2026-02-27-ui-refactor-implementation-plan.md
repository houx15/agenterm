# UI Refactor: Superset-Style IDE Workspace — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the page-based web-app UI with a single-workspace IDE layout: project sidebar + terminal panes + toggleable orchestrator panel.

**Architecture:** Three-column workspace rendered as a single React component tree. Left sidebar shows project tree with nested agents. Center holds auto-laid-out terminal panes. Right panel is a toggleable orchestrator chat + project info. No react-router page navigation — everything in one view. Settings/Home/Connect as modal overlays.

**Tech Stack:** React 18, TypeScript, xterm.js, CSS custom properties (no new deps), lucide-react icons.

---

## File Map — What Changes

### Keep (reuse as-is):
- `api/client.ts`, `api/types.ts`, `api/runtime.ts` — API layer unchanged
- `hooks/useWebSocket.ts` — WebSocket unchanged
- `hooks/useOrchestratorWS.ts` — Orchestrator WS unchanged
- `hooks/useSpeechToText.ts` — Voice input unchanged
- `hooks/useApi.ts` — Generic fetcher unchanged
- `orchestrator/bus.ts`, `orchestrator/replay.ts`, `orchestrator/schema.ts` — Orchestrator utils unchanged
- `settings/asr.ts` — ASR settings unchanged
- `components/Terminal.tsx` — xterm component unchanged
- `components/ChatMessage.tsx` — Chat message renderer unchanged
- `components/Modal.tsx` — Modal component unchanged
- `components/StatusDot.tsx` — Status dot unchanged
- `components/TaskDAG.tsx` — Task DAG unchanged
- `main.tsx` — Entry point unchanged

### Create (new files):
- `components/Workspace.tsx` — Main workspace shell (replaces Layout)
- `components/ProjectSidebar.tsx` — Left sidebar with project tree + utilities
- `components/TerminalGrid.tsx` — Center pane auto-layout for terminal panes
- `components/OrchestratorPanel.tsx` — Right panel (chat + roadmap + demand + exceptions)
- `components/StagePipeline.tsx` — Roadmap pipeline component
- `components/SettingsModal.tsx` — Simplified agent registry modal
- `components/HomeView.tsx` — Compact dashboard/home view
- `components/ConnectModal.tsx` — QR code mobile pairing modal
- `styles/workspace.css` — New workspace-focused stylesheet (replaces globals.css)

### Delete (after migration):
- `components/Layout.tsx` — Replaced by Workspace
- `components/Sidebar.tsx` — Replaced by ProjectSidebar
- `components/ProjectCard.tsx` — No longer needed (project info in sidebar)
- `components/ProjectSelector.tsx` — No longer needed (sidebar has project tree)
- `components/SessionGrid.tsx` — No longer needed (TerminalGrid replaces)
- `components/ActivityFeed.tsx` — Folded into HomeView
- `components/ActionButtons.tsx` — Terminal input handled differently
- `components/CreateProjectModal.tsx` — Simplified into HomeView or sidebar action
- `pages/PMChat.tsx` — Logic split into OrchestratorPanel + Workspace
- `pages/Sessions.tsx` — Logic split into TerminalGrid + Workspace
- `pages/Dashboard.tsx` — Replaced by HomeView
- `pages/DemandPool.tsx` — Embedded in OrchestratorPanel
- `pages/ConnectMobile.tsx` — Replaced by ConnectModal
- `pages/Settings.tsx` — Replaced by SettingsModal
- `pages/ProjectDetail.tsx` — No longer a separate page
- `pages/MobileCompanion.tsx` — Keep separately (different route)

### Modify:
- `App.tsx` — Remove router, render Workspace directly (keep MobileCompanion route)
- `components/Lucide.tsx` — Add new icons needed by new components
- `components/ChatPanel.tsx` — Minor: remove panel header (OrchestratorPanel provides it)

---

## Task 1: CSS Theme Foundation

**Files:**
- Create: `frontend/src/styles/workspace.css`
- Modify: `frontend/src/styles/globals.css` (will be replaced)
- Modify: `frontend/src/main.tsx` (switch CSS import)

**Step 1: Create the new workspace CSS file**

Create `frontend/src/styles/workspace.css` with:
- CSS custom properties for dark theme (default) and light theme
- Base resets (box-sizing, html/body/root 100% height)
- Typography (system font stack)
- Workspace shell grid layout (3-column: sidebar | center | right-panel)
- Sidebar styles (project tree, utility nav, collapse states)
- Terminal pane grid styles (auto-layout, pane headers, dividers)
- Orchestrator panel styles (chat area, input row, pipeline)
- Modal overlay styles
- Utility classes (scrollable, empty-state, badges, status dots)
- Transitions and hover states

Key CSS variables for dark theme:
```css
:root {
  --bg-app: #0a0a0a;
  --bg-surface: #141414;
  --bg-surface-hover: #1a1a1a;
  --bg-surface-active: #1f1f1f;
  --bg-elevated: #1c1c1c;
  --border-subtle: #1e1e1e;
  --border-default: #2a2a2a;
  --text-primary: #e8e8e8;
  --text-secondary: #888888;
  --text-tertiary: #555555;
  --accent: #8b5cf6;
  --accent-hover: #a78bfa;
  --accent-muted: rgba(139, 92, 246, 0.15);
  --status-green: #22c55e;
  --status-yellow: #eab308;
  --status-red: #ef4444;
  --status-gray: #6b7280;
  --font-sans: 'SF Pro Text', -apple-system, 'Segoe UI', sans-serif;
  --font-mono: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
}
```

Light theme override:
```css
:root[data-theme='light'] {
  --bg-app: #fafafa;
  --bg-surface: #ffffff;
  --bg-surface-hover: #f5f5f5;
  --bg-surface-active: #efefef;
  --bg-elevated: #ffffff;
  --border-subtle: #e5e5e5;
  --border-default: #d4d4d4;
  --text-primary: #171717;
  --text-secondary: #737373;
  --text-tertiary: #a3a3a3;
  --accent: #7c3aed;
  --accent-hover: #6d28d9;
  --accent-muted: rgba(124, 58, 237, 0.1);
}
```

Workspace shell:
```css
.workspace {
  display: grid;
  grid-template-columns: var(--sidebar-width, 220px) 1fr var(--panel-width, 0px);
  height: 100vh;
  background: var(--bg-app);
  color: var(--text-primary);
  font-family: var(--font-sans);
  overflow: hidden;
}

.workspace.panel-open {
  --panel-width: 340px;
}

.workspace.sidebar-collapsed {
  --sidebar-width: 0px;
}
```

**Step 2: Update main.tsx to import new CSS**

Change `import './styles/globals.css'` to `import './styles/workspace.css'`

**Step 3: Verify build compiles**

Run: `cd frontend && npm run build`
Expected: Build succeeds (CSS is imported but no components reference it yet)

**Step 4: Commit**

```bash
git add frontend/src/styles/workspace.css frontend/src/main.tsx
git commit -m "feat(ui): add workspace CSS foundation with dark/light theme"
```

---

## Task 2: Lucide Icon Additions

**Files:**
- Modify: `frontend/src/components/Lucide.tsx`

**Step 1: Add new icons needed by the workspace**

Add these icon components to Lucide.tsx (following the existing SVG pattern):
- `PanelRight` — toggle orchestrator panel
- `PanelLeft` — toggle sidebar
- `Bot` — orchestrator agent icon
- `Terminal` — terminal pane icon
- `Layers` — stage pipeline icon
- `QrCode` — connect mobile
- `Inbox` — demand pool / exceptions
- `ArrowRight` — pipeline arrows
- `X` — close buttons
- `Maximize2` — expand pane
- `Minimize2` — collapse pane
- `Circle` — status dot (unfilled)
- `CircleDot` — status dot (needs attention)

Use the lucide icon SVG paths from the lucide-react library (already a dependency).

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No type errors

**Step 3: Commit**

```bash
git add frontend/src/components/Lucide.tsx
git commit -m "feat(ui): add workspace icon set to Lucide component"
```

---

## Task 3: Workspace Shell Component

**Files:**
- Create: `frontend/src/components/Workspace.tsx`
- Modify: `frontend/src/App.tsx`

**Step 1: Create Workspace.tsx**

This is the top-level layout component that replaces `Layout.tsx`. It manages:
- Sidebar open/close state
- Right panel open/close state
- Active project selection
- Theme toggle
- Modal visibility (settings, home, connect)

Structure:
```tsx
export default function Workspace() {
  // State: sidebar collapsed, panel open, active project, theme, modals
  // Renders: <div className="workspace ...">
  //   <ProjectSidebar />        ← left
  //   <TerminalGrid />          ← center
  //   <OrchestratorPanel />     ← right (conditional)
  // </div>
  // Plus modal overlays for Settings, Home, Connect
}
```

For this step, create the shell with placeholder divs for the three columns. Wire up:
- `sidebarCollapsed` state (toggle with Cmd+B / button)
- `panelOpen` state (toggle with button)
- `activeProjectID` state (managed here, passed down)
- `theme` state (dark default, toggle)
- CSS class composition for grid layout

**Step 2: Update App.tsx**

Remove the `createBrowserRouter` with all page routes. Replace with:
```tsx
export default function App() {
  // Keep: token, WebSocket, windows, activeWindow, unread state
  // Remove: router
  // Render: <AppContext.Provider> wrapping either <Workspace /> or <MobileCompanion />
  // Use simple URL check: window.location.pathname === '/mobile' ? <MobileCompanion /> : <Workspace />
}
```

Keep the MobileCompanion route working (check `window.location.pathname`).

**Step 3: Verify the app renders the workspace shell**

Run: `cd frontend && npm run dev`
Open browser: should see a 3-column grid with placeholder content.

**Step 4: Commit**

```bash
git add frontend/src/components/Workspace.tsx frontend/src/App.tsx
git commit -m "feat(ui): add Workspace shell replacing router-based Layout"
```

---

## Task 4: Project Sidebar Component

**Files:**
- Create: `frontend/src/components/ProjectSidebar.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)

**Step 1: Create ProjectSidebar.tsx**

Props:
```tsx
interface ProjectSidebarProps {
  projects: Project[]
  activeProjectID: string
  onSelectProject: (id: string) => void
  sessions: Session[]
  unreadByWindow: Record<string, number>
  onSelectAgent: (session: Session) => void
  onNewProject: () => void
  onOpenHome: () => void
  onOpenConnect: () => void
  onOpenSettings: () => void
  collapsed: boolean
}
```

Renders:
- Brand header: "agenTerm" text
- Project tree: each project is a collapsible group
  - Active project highlighted with accent background
  - Under each project: list of agent sessions
  - Orchestrator sessions listed first (icon: Bot)
  - Other agents listed with Terminal icon
  - Status dot next to each agent (green/yellow/red/gray based on session.status)
  - Unread badge (number) on agents with output
- "+ New Project" button
- Bottom utility nav: Home, Connect, Agents (settings)

Use data from `listSessions` and `listProjects` APIs (already fetched in PMChat, will be lifted to Workspace).

The sidebar should group sessions by project using the `session.project_id` or `session.task_id` → project mapping (existing logic from Sessions.tsx).

**Step 2: Wire into Workspace.tsx**

Replace the left placeholder div with `<ProjectSidebar>` passing the required props.

**Step 3: Verify sidebar renders with project tree**

Run dev server, check that projects and agents appear in the sidebar.

**Step 4: Commit**

```bash
git add frontend/src/components/ProjectSidebar.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add ProjectSidebar with project tree and agent list"
```

---

## Task 5: Terminal Grid Component

**Files:**
- Create: `frontend/src/components/TerminalGrid.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)

**Step 1: Create TerminalGrid.tsx**

This component renders one Terminal pane per non-orchestrator agent session for the active project.

Props:
```tsx
interface TerminalGridProps {
  sessions: Session[]           // Non-orchestrator sessions for active project
  rawBuffers: Record<string, string>  // Terminal history buffers by window ID
  activeWindowID: string | null
  onTerminalInput: (windowID: string, sessionID: string, keys: string) => void
  onTerminalResize: (windowID: string, sessionID: string, cols: number, rows: number) => void
  onClosePane: (windowID: string) => void
  onFocusPane: (windowID: string) => void
}
```

Renders:
- Auto-layout grid:
  - 1 session: single full-width pane
  - 2 sessions: 2 columns, equal width
  - 3+ sessions: 2 columns, rows wrap
- Each pane has:
  - Header bar: agent name (role or agent_type) + status + close [X] button
  - Terminal component (existing `<Terminal />`)
  - Active pane has accent border highlight

Auto-layout CSS:
```css
.terminal-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 1px;
  background: var(--border-subtle);
  flex: 1;
  overflow: hidden;
}

.terminal-pane {
  display: flex;
  flex-direction: column;
  background: var(--bg-app);
  min-height: 0;
}

.terminal-pane-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 12px;
  color: var(--text-secondary);
}

.terminal-pane.active .terminal-pane-header {
  border-bottom-color: var(--accent);
}
```

Migrate terminal buffer management logic from `Sessions.tsx` into `Workspace.tsx` (the parent). TerminalGrid is a pure rendering component.

**Step 2: Wire into Workspace.tsx**

- Move rawBuffers state and terminal_data message handling from Sessions.tsx to Workspace.
- Move `sendInput` / `sendControlKey` / `refreshWindowSnapshot` logic to Workspace.
- Replace center placeholder with `<TerminalGrid>`.
- Filter sessions to non-orchestrator ones for the active project.

**Step 3: Verify terminals render in a grid**

Run dev server. Active project's agent terminals should appear in the center area.

**Step 4: Commit**

```bash
git add frontend/src/components/TerminalGrid.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add TerminalGrid with auto-layout agent terminal panes"
```

---

## Task 6: Stage Pipeline Component

**Files:**
- Create: `frontend/src/components/StagePipeline.tsx`

**Step 1: Create StagePipeline.tsx**

A compact horizontal pipeline showing: `brainstorm → plan → build → test → summarize`

Props:
```tsx
interface StagePipelineProps {
  currentPhase: string          // e.g. "build"
  sessions?: Session[]          // For build stage: show worktree sub-structure
  collapsed?: boolean
  onToggle?: () => void
}
```

Renders:
```
▾ Roadmap
[brainstorm] → [plan] → [BUILD ▾] → [test] → [summarize]
                          ├─ worktree-1: coder (claude) ● working
                          ├─ worktree-1: reviewer (codex) ○ idle
                          └─ worktree-2: coder (claude-2) ● working
```

Each stage is a small pill/chip:
- Completed stages: filled accent background
- Active stage: outlined with accent border, bold text
- Pending stages: dimmed text, no border
- Build stage: expandable to show worktree details when active

CSS:
```css
.stage-pipeline {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 12px;
  flex-wrap: wrap;
}

.stage-chip {
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 500;
  text-transform: capitalize;
}

.stage-chip.done { background: var(--accent); color: white; }
.stage-chip.active { border: 1px solid var(--accent); color: var(--accent); }
.stage-chip.pending { color: var(--text-tertiary); }

.stage-arrow { color: var(--text-tertiary); font-size: 10px; }

.stage-build-detail {
  width: 100%;
  padding: 4px 12px 4px 24px;
  font-size: 11px;
  color: var(--text-secondary);
}
```

For the build stage detail: derive worktree info from sessions that have `role` containing "coder" or "reviewer". Group by worktree (if session metadata includes worktree info, or group by task_id).

**Step 2: Verify component renders correctly**

Can test standalone with mock data before integrating.

**Step 3: Commit**

```bash
git add frontend/src/components/StagePipeline.tsx
git commit -m "feat(ui): add StagePipeline component for project phase visualization"
```

---

## Task 7: Orchestrator Panel Component

**Files:**
- Create: `frontend/src/components/OrchestratorPanel.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)
- Modify: `frontend/src/components/ChatPanel.tsx` (minor: remove panel header)

**Step 1: Modify ChatPanel.tsx**

Remove the `<div className="pm-panel-header">` section at the top (lines 98-101). The OrchestratorPanel will provide its own header. Make the panel header optional via a prop `showHeader?: boolean` (default true for backwards compat during migration).

**Step 2: Create OrchestratorPanel.tsx**

This consolidates the right side from PMChat.tsx — orchestrator chat + project info.

Props:
```tsx
interface OrchestratorPanelProps {
  project: Project | null
  projectID: string
  tasks: Task[]
  sessions: Session[]
  open: boolean
  onClose: () => void
  onOpenTaskSession: (taskID: string) => void
  onOpenDemandPool: () => void
}
```

Structure:
```
<aside className="orchestrator-panel">
  <div className="orchestrator-panel-header">
    <h3>Orchestrator</h3>
    <button onClick={onClose}><X /></button>
  </div>

  <StagePipeline currentPhase={...} sessions={sessions} />

  <ChatPanel ... />   ← Existing ChatPanel, reused

  <div className="orchestrator-panel-sections">
    <details>
      <summary>Demand Pool ({count})</summary>
      <DemandPoolInline projectID={projectID} />  ← Simplified inline version
    </details>
    <details>
      <summary>Exceptions ({openCount})</summary>
      <ExceptionList ... />
    </details>
  </div>
</aside>
```

Migrate from PMChat.tsx:
- `useOrchestratorWS` hook usage
- Progress report fetching
- Exception fetching and resolving
- Chat message assembly
- Roadmap stage computation

The Demand Pool section: create a simplified inline version that uses the existing API functions from `client.ts` (listDemandPoolItems, etc.) without the full DemandPool page complexity. Show a list of items with status badges. "View All" button opens the full DemandPool in a modal.

**Step 3: Wire into Workspace.tsx**

Replace right placeholder with `<OrchestratorPanel>` (conditional on `panelOpen` state).

**Step 4: Verify orchestrator chat works**

Run dev server. Toggle the panel. Send a message to orchestrator. Verify streaming works.

**Step 5: Commit**

```bash
git add frontend/src/components/OrchestratorPanel.tsx frontend/src/components/ChatPanel.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add OrchestratorPanel with chat, pipeline, demand pool, exceptions"
```

---

## Task 8: Settings Modal Component

**Files:**
- Create: `frontend/src/components/SettingsModal.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)

**Step 1: Create SettingsModal.tsx**

Simplified agent registry. Uses existing Modal component.

Props:
```tsx
interface SettingsModalProps {
  open: boolean
  onClose: () => void
}
```

Renders a table:
```
| Name     | Command  | Description          | Actions    |
|----------|----------|----------------------|------------|
| claude   | claude   | Claude Code agent    | [Edit][Del]|
| codex    | codex    | OpenAI Codex agent   | [Edit][Del]|
|          |          |                      |            |
| [+ Add Agent]                                          |
```

Migrate agent CRUD logic from `Settings.tsx`:
- `listAgents`, `createAgent`, `updateAgent`, `deleteAgent`
- Remove playbook management entirely
- Remove ASR settings (keep in asr.ts but no UI for now)

Each row is editable inline (click Edit → fields become inputs).

**Step 2: Wire into Workspace.tsx**

Add `settingsOpen` state. Render `<SettingsModal open={settingsOpen} onClose={...} />`.
ProjectSidebar's "Agents" utility button triggers `onOpenSettings`.

**Step 3: Verify settings modal opens and CRUD works**

**Step 4: Commit**

```bash
git add frontend/src/components/SettingsModal.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add SettingsModal with simplified agent registry"
```

---

## Task 9: Home View Component

**Files:**
- Create: `frontend/src/components/HomeView.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)

**Step 1: Create HomeView.tsx**

A compact dashboard that renders in the center area when no project is selected, or as a modal overlay.

Props:
```tsx
interface HomeViewProps {
  projects: Project[]
  onSelectProject: (id: string) => void
  onCreateProject: () => void
}
```

Shows:
- "agenTerm" heading
- Recent projects list (clickable cards to jump to workspace)
- Quick stats: total projects, active sessions, agents available
- "+ Create Project" button

Migrate minimal logic from Dashboard.tsx:
- `listProjects`, `listSessions` for counts
- Project cards (simplified from ProjectCard.tsx)

**Step 2: Wire into Workspace.tsx**

Show HomeView in the center area when no `activeProjectID` is set.
Also accessible via the "Home" utility button in sidebar.

**Step 3: Commit**

```bash
git add frontend/src/components/HomeView.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add HomeView with recent projects and quick stats"
```

---

## Task 10: Connect Mobile Modal

**Files:**
- Create: `frontend/src/components/ConnectModal.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)

**Step 1: Create ConnectModal.tsx**

Simple modal showing QR code for mobile pairing.

Props:
```tsx
interface ConnectModalProps {
  open: boolean
  onClose: () => void
}
```

Migrate from ConnectMobile.tsx:
- Token-based URL generation
- QR code rendering (the existing page uses a canvas-based QR generator or simple text display)

Check what ConnectMobile.tsx currently does:
- It shows the token and a URL to visit on mobile
- Render this in a centered modal with the QR code

**Step 2: Wire into Workspace.tsx**

Add `connectOpen` state. Render `<ConnectModal>`.
ProjectSidebar's "Connect" button triggers it.

**Step 3: Commit**

```bash
git add frontend/src/components/ConnectModal.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add ConnectModal for mobile QR pairing"
```

---

## Task 11: Create Project Flow

**Files:**
- Create: `frontend/src/components/CreateProjectFlow.tsx`
- Modify: `frontend/src/components/Workspace.tsx` (wire in)

**Step 1: Create CreateProjectFlow.tsx**

Reuse/simplify CreateProjectModal.tsx logic. Modal with:
- Project name input
- Repository path picker (existing directory browser)
- Agent selection (from registry)
- Create button

Migrate from CreateProjectModal.tsx — the API call is `createProject`.

**Step 2: Wire into Workspace.tsx**

ProjectSidebar's "+ New Project" button and HomeView's "Create Project" button both open this modal.

**Step 3: Commit**

```bash
git add frontend/src/components/CreateProjectFlow.tsx frontend/src/components/Workspace.tsx
git commit -m "feat(ui): add CreateProjectFlow modal"
```

---

## Task 12: Workspace State Integration

**Files:**
- Modify: `frontend/src/components/Workspace.tsx` (major)

**Step 1: Migrate data fetching to Workspace**

Move the following data-fetching logic from PMChat.tsx and Sessions.tsx into Workspace.tsx:

From PMChat.tsx:
- `refreshAll` (project list + session stats)
- `loadProjectData` (tasks + sessions for active project)
- `refreshExceptions`
- Project session window mapping
- Project unread counts

From Sessions.tsx:
- `rawBuffers` state + `handleServerMessage` for terminal_data
- `refreshWindowSnapshot`
- `sendInput` / `sendControlKey`
- `createSession`

This makes Workspace the single source of truth for all workspace data.

**Step 2: Pass data down to children**

- ProjectSidebar gets: projects, sessions, activeProjectID, unread counts
- TerminalGrid gets: non-orchestrator sessions, rawBuffers, terminal callbacks
- OrchestratorPanel gets: project, tasks, sessions, orchestrator state

**Step 3: Verify full workspace works end-to-end**

- Select a project → sidebar highlights, terminals appear, orchestrator panel shows chat
- Switch projects → terminals update, chat switches
- Send terminal input → appears in xterm
- Send orchestrator message → streaming response appears
- Toggle panel → slides in/out
- Toggle sidebar → collapses/expands

**Step 4: Commit**

```bash
git add frontend/src/components/Workspace.tsx
git commit -m "feat(ui): integrate full workspace state management"
```

---

## Task 13: App.tsx Final Rewrite

**Files:**
- Modify: `frontend/src/App.tsx`

**Step 1: Simplify App.tsx**

Remove:
- All page imports (Dashboard, PMChat, Sessions, Settings, etc.)
- `createBrowserRouter` and all route definitions
- `RouterProvider`

Keep:
- `AppContext` with token, WebSocket, windows, activeWindow, unread
- `useWebSocket` hook
- MobileCompanion support

New render:
```tsx
export default function App() {
  const [token] = useState(() => getToken())
  const ws = useWebSocket(token)
  // ... existing window/unread state ...

  const isMobile = window.location.pathname === '/mobile'

  return (
    <AppContext.Provider value={value}>
      {isMobile ? <MobileCompanion /> : <Workspace />}
    </AppContext.Provider>
  )
}
```

**Step 2: Remove react-router-dom usage**

Check if any remaining components use `useNavigate`, `useParams`, `useSearchParams`, `NavLink`, etc. Remove those imports. The workspace doesn't use URL routing.

Note: keep `react-router-dom` in package.json for now (MobileCompanion may still use it). If MobileCompanion doesn't need it, remove the dependency entirely.

**Step 3: Verify app loads correctly**

**Step 4: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(ui): simplify App.tsx to single workspace view"
```

---

## Task 14: Delete Old Files

**Files:**
- Delete: `frontend/src/components/Layout.tsx`
- Delete: `frontend/src/components/Sidebar.tsx`
- Delete: `frontend/src/components/ProjectCard.tsx`
- Delete: `frontend/src/components/ProjectSelector.tsx`
- Delete: `frontend/src/components/SessionGrid.tsx`
- Delete: `frontend/src/components/ActivityFeed.tsx`
- Delete: `frontend/src/components/ActionButtons.tsx`
- Delete: `frontend/src/components/CreateProjectModal.tsx`
- Delete: `frontend/src/pages/PMChat.tsx`
- Delete: `frontend/src/pages/Sessions.tsx`
- Delete: `frontend/src/pages/Dashboard.tsx`
- Delete: `frontend/src/pages/DemandPool.tsx`
- Delete: `frontend/src/pages/ConnectMobile.tsx`
- Delete: `frontend/src/pages/Settings.tsx`
- Delete: `frontend/src/pages/ProjectDetail.tsx`
- Delete: `frontend/src/styles/globals.css`

**Step 1: Delete all old files**

**Step 2: Verify no broken imports**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors (all references should point to new components)

**Step 3: Verify build succeeds**

Run: `cd frontend && npm run build`
Expected: Clean build

**Step 4: Commit**

```bash
git add -A frontend/src/
git commit -m "refactor(ui): remove old page-based components and styles"
```

---

## Task 15: Visual Polish Pass

**Files:**
- Modify: `frontend/src/styles/workspace.css`
- Modify: various components for CSS class tweaks

**Step 1: Apply Superset-inspired visual polish**

Using the reference screenshots, refine:
- Sidebar: ensure project items match Superset's "Tab Group" styling
- Terminal pane headers: match Superset's pane header with name + close button
- Orchestrator panel: clean chat layout, compact pipeline
- Status dots: correct colors and sizes
- Notification badges: red circle with white text
- Transitions: smooth 150ms for all hover/active states
- Focus ring: subtle accent outline for keyboard navigation
- Scrollbars: thin, dark-themed

**Step 2: Test both dark and light themes**

Toggle theme and verify all elements look correct in both.

**Step 3: Commit**

```bash
git add frontend/src/
git commit -m "style(ui): apply Superset-inspired visual polish to workspace"
```

---

## Task 16: Build Verification

**Step 1: Full build**

Run: `cd frontend && npm run build`
Expected: Clean build with no warnings

**Step 2: TypeScript check**

Run: `cd frontend && npx tsc --noEmit`
Expected: No type errors

**Step 3: Manual smoke test**

- Load app in browser
- Verify dark theme by default
- Toggle to light theme
- Select a project → terminals appear
- Toggle orchestrator panel → chat works
- Send a message → streaming response
- Open Settings modal → agent registry CRUD works
- Open Connect modal → QR/token displayed
- Check sidebar project switching
- Resize sidebar and panel

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat(ui): complete Superset-style IDE workspace refactor"
```

---

## Execution Order Summary

| Task | Description | Depends On |
|------|-------------|------------|
| 1 | CSS Theme Foundation | — |
| 2 | Lucide Icon Additions | — |
| 3 | Workspace Shell | 1, 2 |
| 4 | Project Sidebar | 3 |
| 5 | Terminal Grid | 3 |
| 6 | Stage Pipeline | — |
| 7 | Orchestrator Panel | 3, 6 |
| 8 | Settings Modal | 3 |
| 9 | Home View | 3 |
| 10 | Connect Modal | 3 |
| 11 | Create Project Flow | 3 |
| 12 | Workspace State Integration | 4, 5, 7, 8, 9, 10, 11 |
| 13 | App.tsx Rewrite | 12 |
| 14 | Delete Old Files | 13 |
| 15 | Visual Polish | 14 |
| 16 | Build Verification | 15 |

Tasks 1, 2, 6 can be done in parallel.
Tasks 4, 5, 7, 8, 9, 10, 11 can be done in parallel after Task 3.
Tasks 12-16 are sequential.
