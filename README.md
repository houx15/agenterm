# agenterm

> A desktop control plane for human-orchestrated AI coding agents — you make the decisions, agents execute in scoped terminal sessions.

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)
![Tauri](https://img.shields.io/badge/Tauri-1.x-FFC131?style=flat&logo=tauri)
![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?style=flat&logo=sqlite)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

agenterm puts you in the driver's seat. Instead of an AI orchestrator deciding what to do next, **you** drive the workflow — planning, building, reviewing, merging, and testing — while AI agents execute autonomously within scoped PTY sessions. Think of it as a human-orchestrator's IDE for managing a fleet of coding agents.

---

## Table of Contents

- [Why agenterm](#why-agenterm)
- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Getting Started](#getting-started)
  - [Desktop App (Tauri)](#desktop-app-tauri)
  - [Server Only (no Tauri)](#server-only-no-tauri)
- [Configuration](#configuration)
- [Usage](#usage)
- [REST API](#rest-api)
- [WebSocket Protocol](#websocket-protocol)
- [Development](#development)
- [Project Structure](#project-structure)
- [License](#license)

---

## Why agenterm

Modern LLM coding agents (Claude Code, Codex, Gemini CLI, Kimi CLI, OpenCode) run in terminal sessions. Managing a fleet of them — each working on an isolated git worktree — is painful with raw terminal tabs.

agenterm gives you a structured control plane:

- **Requirement-driven workflow**: describe what you want to build, plan it with an AI planner, then launch agents to execute
- **Human decisions, AI execution**: you pick the stage transitions (plan → build → review → merge → test); agents just code
- **Live terminal views**: watch every agent's PTY output in real time, switch between TUI/Markdown/Split views
- **Agent capacity dashboard**: see which agents are busy and how many slots are free at a glance

---

## Features

### Core
- **PTY-based sessions** — each agent runs in a native PTY (no tmux dependency); output streams to the browser in real time
- **Output classification** — parser segments output into prompts, errors, code blocks, tool calls, and signals like `[READY_FOR_REVIEW]` or `[BLOCKED]`
- **SQLite persistence** — projects, requirements, sessions, worktrees, review cycles survive restarts
- **Single Go binary** — embeds the React SPA; deploy by copying one file
- **Tauri desktop shell** — native macOS/Linux/Windows app that auto-launches the Go backend as a sidecar

### Workflow
- **Demand pool** — queue up what you want to build; promote demands into requirements
- **Planning sessions** — open a planner agent TUI to brainstorm and produce a blueprint (task breakdown with worktree assignments)
- **One-click execution** — assign agents to blueprint tasks and launch all sessions at once
- **Stage pipeline** — Plan → Build → Review → Merge → Test; you trigger each transition
- **Scaffold system** — auto-generates CLAUDE.md/AGENTS.md and permission configs per worktree based on agent type

### UI
- **Foldable sidebar** — projects list + agent capacity with status dots
- **Three-mode workspace** — Workspace (active sessions), Demands (requirement queue), Settings (project config)
- **Per-agent view modes** — TUI only, Markdown only, or TUI/Markdown split
- **Markdown file tree** — browse and edit `.md` files in any worktree
- **Onboarding wizard** — first-run setup for agents and permission templates

### Agents
- **Agent registry** — define agents with command, capacity, capabilities; managed via REST API or Settings UI
- **Permission templates** — per-agent-type permission configs (`.claude/settings.json`, `.codex/rules/`, `opencode.json`, etc.)
- **Capacity tracking** — real-time view of busy/idle slots per agent

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Tauri Desktop Shell (Rust)                  │
│  Spawns Go backend as sidecar, renders React webview          │
└───────────────────────────┬──────────────────────────────────┘
                            │ http://localhost:8765
┌───────────────────────────▼──────────────────────────────────┐
│                     agenterm (Go binary)                       │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │  HTTP/WS     │  │  REST API    │  │  Scaffold            │ │
│  │  Server      │  │  /api/*      │  │  (CLAUDE.md, perms)  │ │
│  └──────────────┘  └──────┬───────┘  └─────────────────────┘ │
│                           │                                   │
│  ┌──────────────┐  ┌──────▼───────┐  ┌─────────────────────┐ │
│  │  Hub (WS)    │  │  Session     │  │  SQLite (modernc)   │ │
│  │  broadcast   │  │  Lifecycle   │  │  projects/reqs/     │ │
│  └──────┬───────┘  └──────┬───────┘  │  sessions/reviews   │ │
│         │                 │          └─────────────────────┘ │
│  ┌──────▼───────┐  ┌──────▼───────┐                          │
│  │ Output Parser│  │ PTY Backend  │                          │
│  │ (classify)   │  │ (spawn/read) │                          │
│  └──────────────┘  └──────┬───────┘                          │
│                           │                                   │
└───────────────────────────┼──────────────────────────────────┘
                            │ PTY (pseudo-terminal)
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                  ▼
   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
   │ Claude Code │  │ Codex CLI   │  │ Gemini CLI  │  ...
   │ (worktree A)│  │ (worktree B)│  │ (worktree C)│
   └─────────────┘  └─────────────┘  └─────────────┘
```

### Key Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/agenterm` | Process bootstrap, flag parsing, component wiring, graceful shutdown |
| `internal/api` | REST handlers for all resources; auth/CORS/JSON middleware |
| `internal/db` | SQLite repositories, schema migrations, all entity models |
| `internal/hub` | WebSocket client hub, subscriptions, output broadcasting |
| `internal/parser` | ANSI stripping, output segmentation, signal detection |
| `internal/pty` | PTY spawn/read backend for agent sessions |
| `internal/registry` | YAML-backed agent registry |
| `internal/scaffold` | Blueprint parsing, CLAUDE.md generation, permission config writing |
| `internal/session` | Session lifecycle, command policy, idle detection |
| `internal/config` | Flags → config file → env var loading |
| `internal/server` | HTTP mux, `go:embed` SPA serving, WebSocket endpoints |
| `internal/git` | Worktree operations, status/log helpers |
| `src-tauri` | Tauri desktop shell (Rust): sidecar management, window config |
| `frontend` | React 18 + TypeScript + Vite + Tailwind CSS v4 + xterm.js |

---

## Requirements

- **Go** 1.22+
- **Node.js** 18+ and **npm**
- **git** 2.5+ (for worktree support)
- **Rust** + **Cargo** (only if running the Tauri desktop app)
- At least one AI coding agent installed (e.g. `claude`, `codex`, `gemini`)

---

## Getting Started

### Desktop App (Tauri)

The recommended way to run agenterm. Tauri provides a native window and auto-manages the Go backend.

```bash
git clone https://github.com/houx15/agenterm.git
cd agenterm

# Install frontend dependencies
npm --prefix frontend install

# Run in development mode (hot-reload frontend + Go backend)
cargo tauri dev
```

The app opens automatically. The Go backend starts as a sidecar on `localhost:8765` with a built-in token.

To build a release binary:

```bash
cargo tauri build
```

### Server Only (no Tauri)

Run just the Go backend + embedded SPA, accessible in any browser:

```bash
# Build frontend + Go binary
make build

# Start the server
./bin/agenterm
```

Or without building:

```bash
# Build frontend first
make frontend-build

# Run directly
go run ./cmd/agenterm
```

On first run a token is generated and printed:

```
agenterm listening on http://localhost:8765
Token: abc123...
```

Open `http://localhost:8765?token=<your-token>` in a browser. The token is saved in `localStorage` after the first visit.

Use `--print-token` to retrieve the token later:

```bash
./bin/agenterm --print-token
```

---

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8765` | HTTP server port |
| `--token` | auto-generated | Bearer token for authentication |
| `--print-token` | `false` | Print current token and exit |
| `--dir` | `~/08Coding` | Default working directory |
| `--db-path` | `~/.config/agenterm/agenterm.db` | SQLite database path |
| `--agents-dir` | `~/.config/agenterm/agents` | Agent YAML definitions directory |

### Config File

Location: `~/.config/agenterm/config` (auto-created, `0600` permissions)

**Precedence**: CLI flags > config file > built-in defaults

---

## Usage

### 1. First Run — Onboarding

On first launch (no agents configured), an onboarding wizard guides you through:
1. **Agent Setup** — enable/configure AI agents (Claude Code, Codex, Gemini CLI, etc.)
2. **Permission Templates** — set permission levels per agent type
3. **Done** — workspace is ready

### 2. Create a Project

Click **New Project** in the sidebar. Provide a folder path and project name.

### 3. Add Requirements

Switch to the **Demands** tab. Type what you want to build and click **Add**. Requirements queue up with priority ordering.

### 4. Plan

Select a requirement and open a planning session. A planner agent (in a TUI session) helps you brainstorm and produce a blueprint — a breakdown of tasks with worktree and agent assignments.

### 5. Execute

Review the blueprint, assign agents to tasks, and click **Launch All**. Each agent gets its own PTY session in an isolated git worktree with auto-generated context files.

### 6. Monitor & Transition

Watch agents work in the **Workspace** tab. Switch between TUI, Markdown, and Split views per agent. When a stage is complete, click the next stage button (Build → Review → Merge → Test) to advance the pipeline.

---

## REST API

All endpoints require `Authorization: Bearer <token>` or `?token=<token>`.

### Projects
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/projects` | Create project |
| `GET` | `/api/projects` | List projects |
| `GET` | `/api/projects/{id}` | Get project |
| `PATCH` | `/api/projects/{id}` | Update project |
| `DELETE` | `/api/projects/{id}` | Delete project |

### Requirements
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/projects/{id}/requirements` | Create requirement |
| `GET` | `/api/projects/{id}/requirements` | List requirements |
| `POST` | `/api/projects/{id}/requirements/reorder` | Reorder requirements |
| `GET` | `/api/requirements/{id}` | Get requirement |
| `PATCH` | `/api/requirements/{id}` | Update requirement |
| `DELETE` | `/api/requirements/{id}` | Delete requirement |

### Planning & Execution
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/requirements/{id}/planning` | Create planning session |
| `GET` | `/api/requirements/{id}/planning` | Get planning session |
| `PATCH` | `/api/planning-sessions/{id}` | Update planning session |
| `POST` | `/api/planning-sessions/{id}/blueprint` | Save blueprint |
| `POST` | `/api/requirements/{id}/launch` | Launch execution |
| `POST` | `/api/requirements/{id}/transition` | Transition stage |

### Sessions
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/tasks/{id}/sessions` | Create session |
| `GET` | `/api/sessions` | List sessions |
| `GET` | `/api/sessions/{id}` | Get session |
| `POST` | `/api/sessions/{id}/send` | Send command |
| `GET` | `/api/sessions/{id}/output` | Get buffered output |
| `DELETE` | `/api/sessions/{id}` | Destroy session |

### Agents
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/agents` | List agents |
| `GET` | `/api/agents/status` | Agent capacity status |
| `POST` | `/api/agents` | Create agent |
| `PUT` | `/api/agents/{id}` | Update agent |
| `DELETE` | `/api/agents/{id}` | Delete agent |

### Permission Templates
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/permission-templates` | List all templates |
| `GET` | `/api/permission-templates/{agent_type}` | List by agent type |
| `POST` | `/api/permission-templates` | Create template |
| `PUT` | `/api/permission-templates/{id}` | Update template |
| `DELETE` | `/api/permission-templates/{id}` | Delete template |

---

## WebSocket Protocol

### `/ws` — Terminal Events

Subscribe to live session output and status events. Authenticate via `?token=`.

**Server → Client:**
```jsonc
{ "type": "output",        "sessionID": "...", "lines": ["..."] }
{ "type": "status",        "sessionID": "...", "status": "running" }
{ "type": "project_event", "projectID": "...", "event": "...", "data": {...} }
```

**Client → Server:**
```jsonc
{ "type": "subscribe",   "sessionID": "..." }
{ "type": "unsubscribe", "sessionID": "..." }
{ "type": "send",        "sessionID": "...", "text": "ls -la\n" }
```

---

## Development

### Build

```bash
make build            # Frontend + Go binary → bin/agenterm
make frontend-build   # React SPA only → web/frontend/dist/
make run              # go run ./cmd/agenterm
make clean            # Remove bin/
```

### Test

```bash
go test ./...         # All Go tests
go vet ./...          # Vet
```

### Frontend Development

```bash
cd frontend
npm install
npm run dev           # Vite dev server on http://localhost:5173
```

The Vite dev server proxies API requests to the Go backend.

### Desktop Development

```bash
cargo tauri dev       # Runs Vite + Go backend + Tauri window with hot-reload
```

---

## Project Structure

```
agenterm/
├── cmd/agenterm/              # Go entry point
│   └── main.go
├── internal/
│   ├── api/                   # REST handlers + middleware
│   ├── config/                # Flag + config loading
│   ├── db/                    # SQLite repos + migrations
│   ├── git/                   # Worktree and git helpers
│   ├── hub/                   # WebSocket hub
│   ├── parser/                # Output classifier + signal detection
│   ├── pty/                   # PTY backend
│   ├── registry/              # Agent registry (YAML)
│   ├── scaffold/              # Blueprint, CLAUDE.md, permissions
│   ├── server/                # HTTP server + SPA embedding
│   └── session/               # Session lifecycle + idle detection
├── frontend/                  # React 18 + TypeScript + Tailwind v4
│   └── src/
│       ├── api/               # API client
│       ├── components/        # UI components
│       └── styles/            # Tailwind theme
├── src-tauri/                 # Tauri desktop shell (Rust)
│   ├── src/lib.rs             # Sidecar management
│   └── tauri.conf.json        # Window + build config
├── web/
│   ├── embed.go               # go:embed directive
│   └── frontend/dist/         # Built SPA (embedded in binary)
├── configs/agents/            # Agent YAML definitions
├── docs/                      # Design docs and plans
├── Makefile
├── go.mod
└── Cargo.toml
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

Built with [Go](https://go.dev/), [React](https://react.dev/), [Tauri](https://tauri.app/), [xterm.js](https://xtermjs.org/), and [modernc.org/sqlite](https://gitlab.com/cznic/sqlite).
