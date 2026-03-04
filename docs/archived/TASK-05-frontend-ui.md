# Task: frontend-ui

## Context
Agenterm presents terminal sessions as chat conversations. The frontend is a single HTML file with embedded CSS and JS, served by the Go backend via `go:embed`. It must be mobile-first, responsive, and work without any build tools.

**Tech stack:** Vanilla HTML5 + CSS3 + JavaScript (ES modules). No build step. Optionally Preact via CDN for reactivity.

## Objective
Build the complete chat UI with session sidebar, message rendering, quick action buttons, input bar, and WebSocket connection management — all in a single `index.html` file.

## Dependencies
- Depends on: feature/04-websocket-hub (WebSocket protocol spec)
- Branch: feature/05-frontend-ui
- Base: main

## Scope

### Files to Create
- `web/index.html` — Complete single-file frontend (HTML + CSS + JS)

### Files to Modify
- None (replaces the placeholder from feature/01)

### Files NOT to Touch
- All Go files — backend is handled separately

## Implementation Spec

### Step 1: HTML Structure

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=no">
    <title>agenterm</title>
    <style>/* inline CSS */</style>
</head>
<body>
    <div id="app">
        <div id="sidebar"><!-- session list --></div>
        <div id="main">
            <div id="header"><!-- current session info --></div>
            <div id="messages"><!-- message list --></div>
            <div id="input-bar"><!-- text input + send button --></div>
        </div>
    </div>
    <script type="module">/* inline JS */</script>
</body>
</html>
```

### Step 2: CSS Design (Mobile-First)

**Color scheme:** Dark theme (terminal-native feel)
- Background: `#1a1a2e`
- Sidebar: `#16213e`
- Message bubbles: `#0f3460` (terminal output), `#533483` (user input)
- Text: `#e0e0e0`
- Accent: `#00d2ff` (status indicators, buttons)
- Error: `#ff6b6b`
- Prompt/waiting: `#ffd93d`

**Mobile layout (< 768px):**
- Full-screen session list OR chat view (slide transition)
- Header: back button (←) + session name + status indicator
- Messages: full-width bubbles, scrollable
- Input bar: fixed to bottom, 48px height, send button

**Desktop layout (>= 768px):**
- Sidebar: 280px fixed left
- Main area: flex-grow right
- No slide transition, both visible

**Status indicators:**
- Working: spinning animation (CSS `@keyframes spin`)
- Waiting: pulsing animation (CSS `@keyframes pulse`)
- Idle: static circle
- Disconnected: gray circle

**Session naming display:**
Parse `model-project-task` from window name:
- Bold first segment (model name)
- Dimmed separator `/`
- Normal remaining segments
- Example: **Claude** / agenterm / refactor

### Step 3: JavaScript — WebSocket Connection

```javascript
class Connection {
    constructor(token) { ... }
    connect()        // establish WebSocket, handle auth
    reconnect()      // exponential backoff: 1s, 2s, 4s, 8s, max 30s
    send(message)    // send JSON to server
    onMessage(handler) // register message handler
    get status()     // "connected" | "connecting" | "disconnected"
}
```

- Extract token from URL query parameter `?token=`
- Connect to `ws://${location.host}/ws?token=${token}`
- On open: update connection indicator to green
- On close: update to red, start reconnect timer
- On message: parse JSON, dispatch by type

### Step 4: JavaScript — State Management

```javascript
const state = {
    windows: [],           // [{id, name, status}]
    activeWindow: null,    // currently selected window ID
    messages: {},          // {windowID: [Message, ...]}
    unread: {},            // {windowID: count}
    connected: false,
};
```

- On `type: "windows"`: update `state.windows`, re-render sidebar
- On `type: "output"`: append to `state.messages[windowID]`, re-render if active, increment unread if not active
- On `type: "status"`: update window status, re-render sidebar item

### Step 5: JavaScript — Session List (Sidebar)

Render each window as a card:
```
┌─────────────────────────────┐
│ ● Claude / app / auth       │
│ "Do you want to proceed..." │
│                    2m ago ◉ │
└─────────────────────────────┘
```

- Show last message preview (truncated to 50 chars)
- Show relative timestamp ("just now", "2m ago", "1h ago")
- Show status indicator (working/waiting/idle)
- Show unread badge (number) if unread > 0
- Click → set as active window, clear unread, show chat view

### Step 6: JavaScript — Chat View (Messages)

Render messages in a scrollable container:
- **Terminal output (ClassNormal):** Left-aligned, dark bubble, monospace font
- **User input:** Right-aligned, purple bubble (shown when user sends input)
- **Error (ClassError):** Left-aligned, red-tinted bubble, with error icon
- **Prompt (ClassPrompt):** Left-aligned, yellow-tinted bubble, with action buttons below
- **Code (ClassCode):** Left-aligned, code block with syntax highlighting background
- **System (ClassSystem):** Centered, small gray text

**Quick Action Buttons:**
When message has `actions` array:
```html
<div class="actions">
    <button onclick="sendAction('y\n')">Yes</button>
    <button onclick="sendAction('n\n')">No</button>
    <button onclick="sendAction('\x03')" class="danger">Ctrl+C</button>
</div>
```
- Style as pill buttons, large enough for touch (min 44px)
- Danger actions (Ctrl+C) in red

**Auto-scroll:**
- Scroll to bottom on new message IF user is already near bottom (within 100px)
- Show floating "↓ New messages" button if user has scrolled up

### Step 7: JavaScript — Input Bar

```html
<div id="input-bar">
    <textarea id="input" placeholder="Type a command..." rows="1"></textarea>
    <button id="send-btn">➤</button>
</div>
```

- Enter → send (add `\n` to keys)
- Shift+Enter → newline in textarea
- Auto-resize textarea height (max 4 lines)
- On send: `{type: "input", window: activeWindow, keys: text + "\n"}`
- Also render user input as a right-aligned message bubble locally (optimistic)
- Clear textarea after send
- Common shortcuts bar above input (optional, v2):
  - `Ctrl+C`, `Ctrl+D`, `↑` (previous command), `Tab` (autocomplete)

### Step 8: JavaScript — Connection Status Bar

Show a banner at the top when disconnected:
```
⚠ Disconnected — reconnecting in 4s...
```
- Red background, white text
- Show countdown to next reconnect attempt
- Auto-hide when reconnected

## Testing Requirements
- Manual testing on mobile Safari and Chrome (responsive layout)
- WebSocket connection/reconnection works
- Messages render correctly by class (normal, prompt, error, code)
- Quick action buttons send correct keys
- Session switching works (messages preserved per window)
- Unread badges increment/clear correctly
- Auto-scroll behavior works (scroll to bottom, "new messages" button)
- Input: Enter sends, Shift+Enter adds newline

## Acceptance Criteria
- [ ] Mobile layout works on 375px+ screens (iPhone SE and larger)
- [ ] Desktop layout shows sidebar + chat side by side
- [ ] Session list shows all windows with name, status, preview, unread count
- [ ] Chat view renders messages with correct styling per class
- [ ] Quick action buttons appear for prompt messages and send correct keys
- [ ] WebSocket reconnects automatically with exponential backoff
- [ ] Disconnection banner appears/disappears correctly
- [ ] Input bar sends commands and shows user input as chat bubble
- [ ] Auto-scroll works correctly

## Notes
- All CSS and JS must be inline in the single HTML file (for `go:embed`).
- Use CSS custom properties (variables) for the color scheme to make theming easy later.
- Use `monospace` font for message text (terminal output should feel like terminal).
- The `actions` array in messages may be empty — only render buttons when actions exist.
- Timestamps should use relative format ("2m ago") and update periodically (every 30s).
- Test on actual mobile device — touch targets must be at least 44x44px.
- Handle the case where no windows exist — show an empty state with instructions.
