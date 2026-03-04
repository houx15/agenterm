# Task: pm-chat-ui

## Context
The PM Chat page is where users interact with the Orchestrator (AI Project Manager). It combines a task DAG visualization with a streaming chat interface. This is the primary control surface for the entire system.

## Objective
Build the PM Chat page with: project selector, task dependency DAG, streaming chat with the orchestrator, and interactive elements (clickable tasks, confirmation buttons).

## Dependencies
- Depends on: TASK-13 (frontend-react), TASK-15 (orchestrator)
- Branch: feature/pm-chat-ui
- Base: main (after dependencies merge)

## Scope

### Files to Create
- `frontend/src/pages/PMChat.tsx` — PM Chat page layout
- `frontend/src/components/TaskDAG.tsx` — Task dependency graph visualization
- `frontend/src/components/ChatPanel.tsx` — Streaming chat with orchestrator
- `frontend/src/components/ChatMessage.tsx` — (enhance) Message bubble with tool call visibility
- `frontend/src/components/ProjectSelector.tsx` — Project dropdown/tabs
- `frontend/src/hooks/useOrchestratorWS.ts` — WebSocket hook for /ws/orchestrator

### Files to Modify
- `frontend/src/App.tsx` — Add PM Chat route
- `frontend/src/components/Sidebar.tsx` — Add PM Chat navigation link

## Implementation Spec

### Step 1: Page layout
```
┌──────────────────────────────────────────────┐
│  PM Chat          [Project: myapp ▾]         │
├────────────────────┬─────────────────────────┤
│  Task Graph        │  Chat                   │
│                    │                         │
│  ┌──────┐          │  PM: I'll break this    │
│  │ auth │──┐       │  into 3 tasks...        │
│  └──────┘  │       │                         │
│  ┌──────┐  ├──┐    │  [Creating worktree...] │
│  │ api  │──┘  │    │  [Starting agent...]    │
│  └──────┘     │    │                         │
│  ┌──────┐     │    │  ✅ All tasks created   │
│  │ test │─────┘    │                         │
│  └──────┘          │  You: How's progress?   │
│                    │                         │
│                    ├─────────────────────────┤
│                    │  [Message input...]  📎  │
└────────────────────┴─────────────────────────┘
```

### Step 2: Task DAG visualization
- Render tasks as nodes in a dependency graph
- Edges show depends_on relationships
- Color-coded by status: pending(gray), running(blue), reviewing(yellow), done(green), failed(red)
- Click task → navigate to its session terminal
- Use SVG or canvas rendering (no heavy graph library needed for simple DAGs)

### Step 3: Chat panel
- Connect to `ws://host/ws/orchestrator?project_id=<id>`
- Stream tokens as they arrive
- Show tool calls inline: "[Creating worktree feature/auth...]"
- Show tool results: "[✅ Worktree created]"
- User messages in right-aligned bubbles
- PM responses in left-aligned bubbles
- Auto-scroll to bottom on new messages

### Step 4: Interactive elements
- Task mentions in PM responses are clickable links
- Confirmation prompts rendered as button groups
- "Create 3 worktrees?" → [Confirm] [Modify] [Cancel]
- Progress indicators for long operations

### Step 5: Orchestrator WebSocket hook
```typescript
function useOrchestratorWS(projectId: string) {
  // Returns: { messages, send, isStreaming }
  // Handles: token streaming, tool_call display, done signals
  // Reconnects on disconnect
}
```

## Acceptance Criteria
- [x] Task DAG renders with correct dependencies
- [x] Chat streams orchestrator responses in real-time
- [x] Tool calls visible inline in chat
- [x] Tasks in DAG update status in real-time
- [x] Clicking task navigates to session terminal
- [x] Project selector switches context
- [x] Mobile responsive (DAG collapses to list on small screens)

## Notes
- The DAG doesn't need to be complex — even a simple vertical list with arrows for dependencies works as v1
- Use `requestAnimationFrame` for smooth token streaming
- Conversation history loaded from API on page mount
- Consider Markdown rendering for PM responses (code blocks, lists)
