# Task: frontend-react

## Context
The current frontend is a single HTML file (~1000 lines) with inline CSS/JS. The SPEC calls for a multi-page app with Dashboard, PM Chat, and Session Terminal views. This requires migrating to React for component-based development. The frontend should be buildable to static files that Go can embed.

## Objective
Scaffold a React (Vite) frontend with routing, shared layout (sidebar), and migrate the existing session terminal functionality to React components.

## Dependencies
- Depends on: TASK-08 (rest-api) — needs API endpoints to consume
- Branch: feature/frontend-react
- Base: main (after TASK-08 merge)

## Scope

### Files to Create
```
frontend/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
├── src/
│   ├── main.tsx
│   ├── App.tsx                    — Router + layout
│   ├── api/
│   │   ├── client.ts             — Fetch wrapper with auth
│   │   └── types.ts              — TypeScript types matching Go models
│   ├── hooks/
│   │   ├── useWebSocket.ts       — WebSocket connection + reconnect
│   │   └── useApi.ts             — API fetch hooks
│   ├── components/
│   │   ├── Layout.tsx            — Sidebar + main content shell
│   │   ├── Sidebar.tsx           — Navigation + session list
│   │   ├── Terminal.tsx          — xterm.js terminal component
│   │   ├── ChatMessage.tsx       — Parsed output message bubble
│   │   ├── ActionButtons.tsx     — Quick action buttons (Y/n, etc.)
│   │   └── StatusDot.tsx         — Session status indicator
│   ├── pages/
│   │   ├── Dashboard.tsx         — Overview page (placeholder)
│   │   ├── Sessions.tsx          — Session list + terminal view
│   │   └── Settings.tsx          — Agent registry config (placeholder)
│   └── styles/
│       └── globals.css           — Dark theme, matching current design
```

### Files to Modify
- `web/embed.go` — Change embed path to `frontend/dist` (Vite output)
- `internal/server/server.go` — Serve from new embed path, add SPA fallback (serve index.html for all non-API routes)
- `Makefile` — Add `frontend-build` target, update `build` to include frontend

### Files NOT to Touch
- `internal/` (Go backend) — No changes except server.go embed path
- Existing `web/index.html` — Keep as fallback, will be replaced

## Implementation Spec

### Step 1: Scaffold Vite + React + TypeScript
```bash
npm create vite@latest frontend -- --template react-ts
cd frontend
npm install react-router-dom @xterm/xterm @xterm/addon-fit
npm install -D @types/react @types/react-dom
```

### Step 2: Configure Vite for Go embedding
```typescript
// vite.config.ts
export default defineConfig({
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8765',
      '/ws': { target: 'ws://localhost:8765', ws: true },
    },
  },
})
```

### Step 3: Layout component
- Sidebar (280px, collapsible on mobile) with:
  - App logo/name
  - Navigation links (Dashboard, Sessions, Settings)
  - Session list grouped by project (later) or flat list (initially)
  - Each session shows: name, status dot, agent type icon
- Main content area renders routed page

### Step 4: WebSocket hook
Port the existing `Connection` class to a React hook:
```typescript
function useWebSocket(token: string) {
  // Returns: { connected, messages, send, lastMessage }
  // Handles: auto-reconnect with exponential backoff
  // Message types: windows, output, terminal_data, status
}
```

### Step 5: Terminal component
Port xterm.js integration:
```typescript
function Terminal({ sessionId, wsConnection }: Props) {
  // Creates xterm.js instance
  // Subscribes to terminal_data messages for this session
  // Sends terminal_input messages on keypress
  // Handles resize with FitAddon
}
```

### Step 6: Sessions page
- Left panel: list of active sessions (from WebSocket windows message)
- Right panel: selected session's terminal OR chat view
- Toggle between raw terminal and chat mode
- Port existing chat message rendering (ClassNormal, ClassPrompt, ClassError, ClassCode)

### Step 7: Build integration
```makefile
frontend-build:
	cd frontend && npm install && npm run build

build: frontend-build
	go build -o bin/agenterm ./cmd/agenterm
```

Update `web/embed.go`:
```go
//go:embed frontend/dist
var Assets embed.FS
```

## Testing Requirements
- Vite dev server proxies correctly to Go backend
- WebSocket connection establishes and reconnects
- Terminal renders and accepts input
- Chat messages display with correct styling
- Built output embeds correctly in Go binary
- SPA routing works (refresh on /sessions doesn't 404)

## Acceptance Criteria
- [x] React app builds to static files embeddable in Go
- [x] Sidebar navigation between Dashboard/Sessions/Settings
- [x] WebSocket connection with auto-reconnect
- [x] Terminal component renders xterm.js with full I/O
- [x] Chat view shows classified messages with action buttons
- [x] Dark theme matching current design
- [x] Mobile responsive (sidebar collapses)
- [x] Dev mode: `npm run dev` with proxy to Go backend

## Notes
- Keep the frontend lightweight — no heavy state management (React context is enough)
- Use React Router v6 with `createBrowserRouter`
- The Go server needs SPA fallback: return index.html for any path that doesn't match /api/* or /ws
- Token passed via URL query param or localStorage
- Port the existing CSS variables (--bg-main, --accent, etc.) to globals.css
- xterm.js v5 is already in the project's web/vendor — can reuse or npm install fresh
