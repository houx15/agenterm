# Agenterm Design Document

## Overview

Agenterm is a web-based terminal session manager that turns tmux windows into chat-like conversations, enabling cross-device management of AI coding CLI sessions (Claude Code, Kimi, Codex, etc.) from a mobile browser.

**Core value proposition:**
1. **Multi-tool orchestration** — manage multiple AI CLIs side-by-side like IM threads
2. **Semantic intelligence** — auto-detect Y/n prompts, errors, code blocks; surface quick-action buttons
3. **Universal & lightweight** — zero-install browser client, single binary server, works with any tmux session

## Tech Stack

- **Backend:** Go (single binary via `go build`, embedded frontend via `go:embed`)
- **Frontend:** Vanilla JS (or Preact via CDN, no build step), single `index.html`
- **Protocol:** WebSocket (JSON) for real-time bidirectional communication
- **Terminal integration:** tmux Control Mode (`tmux -C`)
- **Network:** Tailscale for secure remote access (optional, any network works)

## Architecture

```
┌─────────────────────────────────────────────┐
│  Mobile/Desktop Browser                     │
│  Vanilla JS + optional Preact (CDN)         │
│  Chat UI with session sidebar               │
└──────────────┬──────────────────────────────┘
               │ WebSocket (JSON)
┌──────────────▼──────────────────────────────┐
│  agenterm (single Go binary)                │
│  ┌──────────┐ ┌───────────┐ ┌────────────┐ │
│  │ HTTP/WS  │ │ Session   │ │ Output     │ │
│  │ Server   │ │ Manager   │ │ Parser     │ │
│  └──────────┘ └───────────┘ └────────────┘ │
│  ┌──────────┐ ┌───────────┐                │
│  │ Auth     │ │ Tmux      │                │
│  │ (Token)  │ │ Gateway   │                │
│  └──────────┘ └───────────┘                │
└──────────────┬──────────────────────────────┘
               │ stdin/stdout (Control Mode)
┌──────────────▼──────────────────────────────┐
│  tmux server                                │
│  ┌────────┐ ┌────────┐ ┌────────┐          │
│  │Window 0│ │Window 1│ │Window 2│          │
│  │Claude  │ │Kimi    │ │Shell   │          │
│  └────────┘ └────────┘ └────────┘          │
└─────────────────────────────────────────────┘
```

## Component Design

### Tmux Gateway

Spawns `tmux -C attach -t <session>` as a subprocess with two goroutines:

**Reader goroutine** — parses stdout line by line:
- `%output %<pane_id> <data>` — terminal output (octal-escaped, must decode)
- `%window-add @<id>` — new window created
- `%window-close @<id>` — window destroyed
- `%window-renamed @<id> <name>` — name changed
- `%begin`/`%end`/`%error` — command response framing

**Writer goroutine** — sends commands to stdin:
- `send-keys -t @<id> <keys>` — inject keystrokes
- `list-windows` — enumerate windows on startup

Maintains `map[string]*Window` of active windows with metadata.

### Output Parser (Semantic Layer)

Transforms raw terminal byte streams into chat-friendly messages.

**Accumulator** — buffers output per window, flushes on:
- Timeout: 1.5s of silence
- Prompt detection: regex `[$>%❯]\s*$`
- Confirmation detection: `[Y/n]`, `[y/N]`, `Do you want`, `Are you sure`

**Classifier** — tags each flushed message:
- `prompt` — contains Y/n or similar → triggers quick-action buttons
- `error` — contains "Error", "error:", "FAIL" → red styling
- `code` — contains code fences or indented blocks → syntax highlighting
- `normal` — everything else

**ANSI handling:**
- v1: Strip ANSI codes to plain text
- v2: Convert to colored `<span>` elements

### WebSocket Hub

Fan-out pattern: one tmux connection, N browser clients.

**Server → Client messages:**
```json
{"type": "output", "window": "@0", "text": "...", "class": "prompt", "ts": 1234567890}
{"type": "windows", "list": [{"id": "@0", "name": "claude-app-refactor", "status": "working"}]}
{"type": "status", "window": "@0", "status": "waiting"}
```

**Client → Server messages:**
```json
{"type": "input", "window": "@0", "keys": "y\n"}
{"type": "resize", "window": "@0", "cols": 80, "rows": 24}
```

### Auth

- On first run, generate a random token and print to console
- Clients connect via `ws://host:8765/ws?token=<token>`
- Token stored in `~/.config/agenterm/config.toml`
- Sufficient for Tailscale-secured personal use; open source users can add reverse proxy auth

## Session Naming & Status

### Naming Convention
`<model>-<project>-<task>` — derived from tmux window name.
- Examples: `claude-agenterm-refactor`, `kimi-webapp-auth`
- Displayed in UI as structured: **Claude** / agenterm / refactor

### Status Signals
- **Working** (spinning) — output received within last 3 seconds
- **Waiting for response** (pulsing) — confirmation prompt detected, awaiting user input
- **Idle** (static dot) — no output for >30 seconds, no pending prompt
- **Disconnected** (gray) — window closed or tmux session lost

Statuses derived heuristically from the output stream, no configuration needed.

## Frontend Design

### Mobile Layout (< 768px)

**Session list view:**
```
┌─────────────────────────┐
│ ≡ agenterm    ● Online  │
├─────────────────────────┤
│ 🔵 Claude / app / auth  │
│ "Fixing auth bug..."    │  2m ago  ● Working
├─────────────────────────┤
│ 🟡 Kimi / web / tests   │
│ "Do you want to..."     │  5m ago  ◉ Waiting
├─────────────────────────┤
│ ⚫ Shell / ops / deploy  │
│ "$ _"                   │ 12m ago  ○ Idle
└─────────────────────────┘
```

**Chat view (tap a session):**
```
┌─────────────────────────┐
│ ← Claude / app / auth   │
├─────────────────────────┤
│ ┌─────────────────────┐ │
│ │ Searching files...  │ │
│ └─────────────────────┘ │
│ ┌─────────────────────┐ │
│ │ Proceed? [Y/n]      │ │
│ │ [Yes] [No] [Ctrl+C] │ │
│ └─────────────────────┘ │
├─────────────────────────┤
│ [Type a command...]  ➤  │
└─────────────────────────┘
```

**Desktop (>= 768px):** Side-by-side (session list left, chat right).

### Quick Actions System
When Output Parser detects a confirmation prompt:
- Parse options (Y/n, yes/no, numbered choices)
- Generate buttons dynamically
- Presets: `[Y/n]` → Yes/No/Ctrl+C, Error → Retry/Ctrl+C

### Frontend Tech
- No build step: single `index.html` embedded via `go:embed`
- Reactive rendering via vanilla JS or Preact (CDN)
- Auto-scroll with "scroll to bottom" button
- WebSocket reconnect with exponential backoff (1s → 30s max)

## Error Handling

- **tmux session not found:** Show setup instructions in UI
- **tmux process crashes:** Auto-reconnect with 5s backoff, notify clients
- **WebSocket disconnect:** Client exponential backoff (1s, 2s, 4s, 8s, max 30s)
- **Large output bursts:** Batch WebSocket sends (max 1 msg per 100ms per window)
- **Binary output:** Detect non-UTF8, show "[binary data skipped]"
- **Message history:** Last 500 messages in memory per window (configurable, no disk persistence in v1)

## Deployment

```bash
# Install
go install github.com/<user>/agenterm@latest

# Or download binary
curl -L https://github.com/<user>/agenterm/releases/latest/download/agenterm-$(uname -s)-$(uname -m) -o agenterm

# Run
agenterm --session ai-coding --port 8765

# Access
# Local: http://localhost:8765
# Remote (via Tailscale): http://<tailscale-ip>:8765?token=<token>
```

## Future Extensions (Not in v1)

1. AI summarization of terminal output
2. File system browsing via the web UI
3. Voice input (Web Speech API)
4. Multi-user read-only observation mode
5. Persistent message history (SQLite)
6. Notification system (browser push notifications for prompts)
