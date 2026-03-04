# UI Refactor: Superset-Style IDE Workspace

**Date:** 2026-02-27
**Status:** Approved
**Approach:** Full single-workspace consolidation (Approach A)

## Problem

The current UI uses traditional web-app page routing (sidebar nav to separate pages: PMChat, Sessions, Dashboard, Settings). This feels awkward for a desktop-style agent orchestration tool. Users must context-switch between pages to see terminals, chat with the orchestrator, and manage projects.

## Reference

Superset (`reference/superset/`) — an Electron-based terminal for coding agents with:
- Tab bar + sidebar + split terminal panes
- Dark theme with purple accents
- Agent status indicators and notifications
- shadcn/ui component library

## Design

### Overall Layout: Three-Column Workspace

```
+----------------+-----------------------------------------+---------------------+
| Left Sidebar   | Center: Terminal Panes                  | Right: Orchestrator  |
| (~200px)       | (flex)                                  | (~320px, toggleable) |
+----------------+-----------------------------------------+---------------------+
```

Everything lives in a single view. No page-based routing (except `/settings` as modal overlay).

### 1. Left Sidebar

Two sections: project tree (top) and utilities (bottom).

```
+------------------+
| agenTerm         |  <- Brand, no badge
|------------------|
| > my-app       * |  <- Active project (* = notifications)
|   @ orchestr.    |  <- Orchestrator (always first, special icon)
|   o claude       |  <- Agent: status dot (green/yellow/red/gray)
|   o codex        |
|   o server       |
|                  |
| > proj-2         |  <- Collapsed project
| > proj-3         |
|                  |
| [+ New Project]  |
|                  |
|------------------|
| [home] Home      |  <- Dashboard: recent projects + stats
| [qr]   Connect   |  <- QR code modal for mobile pairing
| [gear] Agents    |  <- Agent registry (simplified)
+------------------+
```

**Behaviors:**
- Projects are expandable tree nodes showing their agents.
- Only one project active at a time (highlighted).
- Clicking a project switches the center pane to its agents.
- Agent status dots: green (working), yellow (needs input), gray (idle), red (error).
- Unread badge (number) on agents with new output.
- Collapsible with Cmd+B.

### 2. Center: Terminal Panes

Each non-orchestrator agent gets an xterm terminal pane.

```
+------------------------+-------------------+
| claude                 | codex        [x]  |
| ~/proj > claude        | ~/proj > codex    |
|                        |                   |
| Claude Code v2.0.55   | OpenAI Codex      |
| > refactoring UI...   | > fixing bug...   |
|                   [x]  |                   |
+------------------------+-------------------+
| server                                [x]  |
| ~/proj > bun dev                           |
| ready on localhost:3000                    |
+--------------------------------------------+
```

**Behaviors:**
- Auto-layout: 1 agent = full width, 2 = side-by-side, 3+ = 2-col grid.
- Pane headers with agent name + close button.
- Resizable dividers between panes.
- Clicking agent in sidebar focuses that pane.
- Empty state when no agents running.

### 3. Right Panel: Orchestrator + Project Info

Toggleable panel containing orchestrator chat and project context.

```
+---------------------+
| Orchestrator   [x]  |
|---------------------|
| > Roadmap           |  <- Foldable pipeline
| [brainstorm] [plan] |
| [build]      [test] |
| [summarize]         |
|---------------------|
|                     |
| Chat messages...    |
|                     |
| > Demand Pool (3)   |  <- Expandable section
| > Exceptions (1)    |  <- Expandable with count
|---------------------|
| [Type message...]   |
| [mic]        [Send] |
+---------------------+
```

**Behaviors:**
- Toggle with button or keyboard shortcut.
- Chat: real-time streaming from orchestrator WebSocket.
- Demand Pool: expandable section, not a separate page.
- Exceptions: expandable list with resolve buttons.
- Voice input preserved.
- Panel width resizable.

### 4. Stage Pipeline (Roadmap) Design

The pipeline represents the project workflow: `brainstorm -> plan -> build -> test -> summarize`.

During **build stage**, complexity increases:
- Multiple worktrees may be active simultaneously
- Agents have roles: `coder` and `reviewer`
- The pipeline should show this sub-structure

```
Pipeline (compact, in orchestrator panel):

  [brainstorm] → [plan] → [BUILD ▾] → [test] → [summarize]
                            |
                            +-- worktree-1: coder (claude) ● working
                            +-- worktree-1: reviewer (codex) ○ idle
                            +-- worktree-2: coder (claude-2) ● working
```

When the build stage is active and expanded, show:
- Sub-rows for each worktree
- Agent assignment (which agent is coder, which is reviewer)
- Status per agent within the build stage

For config: the playbook YAML already defines stages. The UI reads the playbook's stage definitions and agent role assignments. No special UI config needed beyond what the agent registry and playbook provide.

### 5. Visual Theme

**Dark theme (default):**
- Background: #0a0a0a (near-black)
- Surface: #141414 (sidebar, panels)
- Border: #1e1e1e (subtle)
- Text primary: #e8e8e8
- Text secondary: #888888
- Accent: #8b5cf6 (purple, matching Superset)
- Status: green #22c55e, yellow #eab308, red #ef4444, gray #6b7280

**Light theme (available via toggle):**
- Background: #fafafa
- Surface: #ffffff
- Border: #e5e5e5
- Text primary: #171717
- Text secondary: #737373
- Accent: #7c3aed (slightly darker purple for contrast)

Default is dark. Toggle in sidebar utilities or keyboard shortcut.

**Typography:**
- UI: system sans-serif stack (SF Pro, Segoe UI, etc.)
- Terminals: xterm monospace (unchanged)

**Effects:**
- Minimal shadows, flat design
- 150ms ease transitions for hover/focus
- No gradients (current body gradient removed)

### 6. Pages Removed/Simplified

| Current | New Location |
|---------|-------------|
| PMChat page | Right panel (orchestrator) |
| Sessions page | Center pane (terminals) |
| Dashboard page | "Home" utility view (compact) |
| DemandPool page | Section in orchestrator panel |
| ConnectMobile page | QR modal from sidebar |
| Settings page | Modal overlay, agent registry only |
| Topbar header | Removed; brand in sidebar |
| Playbook config | Removed from settings UI |

### 7. Agent Registry (Settings)

Simplified to match Superset's approach:

```
+--------------------------------------+
| Agent Registry                  [x]  |
|--------------------------------------|
| Name        | Command     | Spec     |
|-------------|-------------|----------|
| claude      | claude      | Claude   |
| codex       | codex       | OpenAI   |
| aider       | aider       | Aider    |
|                                      |
| [+ Add Agent]                        |
+--------------------------------------+
```

Each agent entry: name, launch command, one-line description.
No playbook designer in the UI.

## Non-Goals

- No Electron/Tauri changes (this is a web UI refactor only)
- No backend API changes (reuse existing endpoints)
- No mobile companion changes (separate route stays)
- No new dependencies (stay with React + xterm + lucide + CSS variables)
