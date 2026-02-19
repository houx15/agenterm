# agenterm

> A self-hosted, browser-based control plane for running AI coding agents — orchestrate multiple LLM sessions through tmux, chat with a project manager AI, and watch your codebase build itself.

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)
![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?style=flat&logo=sqlite)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

agenterm is a single-binary Go server that bridges tmux terminal sessions to a modern React web UI. It gives you a project-level orchestrator powered by Claude, a playbook system for multi-agent workflows, speech-to-text input, and live streaming of every agent's output — all accessible from a phone over Tailscale.

---

## Table of Contents

- [Why agenterm](#why-agenterm)
- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Usage Guide](#usage-guide)
  - [PM Chat — Talk to the Orchestrator](#pm-chat--talk-to-the-orchestrator)
  - [Sessions — Live Agent Terminals](#sessions--live-agent-terminals)
  - [Settings — Agents, Playbooks & ASR](#settings--agents-playbooks--asr)
  - [Human Takeover](#human-takeover)
  - [Remote Access via Tailscale](#remote-access-via-tailscale)
- [Agent System](#agent-system)
- [Skills Protocol](#skills-protocol)
- [Playbook System](#playbook-system)
- [Review Cycles](#review-cycles)
- [REST API](#rest-api)
- [WebSocket Protocol](#websocket-protocol)
- [Development](#development)
- [Project Structure](#project-structure)
- [Security](#security)
- [Contributing](#contributing)
- [License](#license)

---

## Why agenterm

Modern LLM coding agents (Claude Code, Codex, Gemini CLI) run in terminal sessions. Managing a fleet of them — each working on an isolated git worktree, coordinating handoffs, reviewing each other's code — is painful with raw tmux.

agenterm wraps that workflow in a structured control plane:

- **Durable metadata**: projects, tasks, sessions, and review cycles stored in SQLite
- **Orchestrator**: an LLM that reads your backlog, spawns agents, and drives tasks to completion via tool calls against the REST API
- **Playbooks**: YAML-defined workflow stages (`plan` / `build` / `test`) matched to projects by language or path pattern
- **Live UI**: chat with the orchestrator, watch agent output stream in real time, take over a terminal with a tap

---

## Features

### Core Platform
- **Multi-session tmux management** — create, monitor, and destroy agent sessions via REST; each session is a tmux window in control mode
- **Live output streaming** — WebSocket hub pushes ANSI-stripped, classified output to every subscribed browser tab
- **Output classification** — parser segments output into prompts, errors, code blocks, and normal text; generates quick-action buttons for `[Y/n]`, `Ctrl+C`, etc.
- **SQLite persistence** — projects, tasks, worktrees, sessions, orchestrator history, and review cycles survive restarts
- **Single binary** — Go embeds the React SPA; deploy by copying one file

### Orchestration
- **PM Chat** — chat UI backed by a Claude tool-calling loop; the orchestrator can list/create/update projects, tasks, sessions, and worktrees through the local API
- **Scheduler guardrails** — global, per-project, per-workflow, per-role, and per-model parallelism limits prevent runaway agent spawning
- **Orchestrator history** — every tool call, result, and message is persisted and replayed on reconnect
- **Project knowledge** — store freeform notes that the orchestrator injects into its system prompt
- **Progressive skill disclosure** — orchestrator sees skill summaries first, then loads full skill docs only when it decides to apply one

### Agents & Playbooks
- **Agent registry** — YAML-defined agent profiles (command, model, capabilities, cost tier, speed tier); managed via Settings UI or REST API
- **Playbook system** — multi-phase workflow templates matched to projects by language or path; define the sequence of agents and their roles
- **Git worktree isolation** — each task gets its own `git worktree`, keeping branches independent

### Automation
- **Auto-commit** — periodic commit loop keeps work checkpointed
- **Review coordinator** — detects `[READY_FOR_REVIEW]` commits, prompts a reviewer agent, and feeds results back to the coder
- **Auto-merge** — attempts merge after review passes; escalates conflicts to the orchestrator

### Developer Experience
- **ASR speech input** — record audio in the browser, transcribe via Volcengine, populate the chat input
- **Human takeover** — click "Take Over" to pause automation and type directly into any agent's terminal
- **Review cycles** — structured API for tracking code review iterations and individual issues
- **Mobile-first UI** — collapsible sidebar, responsive layout, works on phones over Tailscale

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser (React SPA)                      │
│                                                                  │
│  ┌──────────┐  ┌─────────────────┐  ┌──────────────────────┐   │
│  │ Sidebar  │  │   PM Chat Page  │  │   Sessions Page      │   │
│  │ (nav)    │  │ orchestrator WS │  │ live output + input  │   │
│  └──────────┘  └────────┬────────┘  └──────────┬───────────┘   │
│                         │ /ws/orchestrator       │ /ws           │
└─────────────────────────┼────────────────────────┼──────────────┘
                          │                        │
┌─────────────────────────▼────────────────────────▼──────────────┐
│                       agenterm (Go binary)                        │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │  HTTP/WS     │  │  REST API    │  │  Orchestrator          │ │
│  │  Server      │  │  /api/*      │  │  (Claude tool-calling) │ │
│  └──────────────┘  └──────┬───────┘  └──────────┬─────────────┘ │
│                           │                      │               │
│  ┌──────────────┐  ┌──────▼───────┐  ┌──────────▼─────────────┐ │
│  │  Hub (WS)    │  │  Session     │  │  Automation            │ │
│  │  broadcast   │  │  Lifecycle   │  │  (commit/review/merge) │ │
│  └──────┬───────┘  └──────┬───────┘  └────────────────────────┘ │
│         │                 │                                      │
│  ┌──────▼───────┐  ┌──────▼───────┐  ┌────────────────────────┐ │
│  │ Output Parser│  │ Tmux Manager │  │  SQLite (modernc)      │ │
│  │ (classify)   │  │ (control mode│  │  projects/tasks/       │ │
│  └──────────────┘  └──────┬───────┘  │  sessions/reviews      │ │
│                           │          └────────────────────────┘ │
└───────────────────────────┼──────────────────────────────────────┘
                            │ tmux -C (control mode)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                           tmux                                   │
│  ┌───────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  Window @0    │  │  Window @1   │  │  Window @2   │  ...    │
│  │  claude code  │  │  codex       │  │  gemini cli  │         │
│  │  (implementer)│  │  (reviewer)  │  │  (researcher)│         │
│  └───────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

### Key Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/agenterm` | Process bootstrap, flag parsing, component wiring, graceful shutdown |
| `internal/config` | Flags → config file → env var loading, token generation |
| `internal/server` | HTTP mux, `go:embed` SPA serving, WebSocket endpoint registration |
| `internal/api` | REST handlers for all resources; auth/CORS/JSON middleware |
| `internal/db` | SQLite repositories, schema migrations, all entity models |
| `internal/tmux` | Control-mode protocol parser, gateway/manager for multi-session tmux |
| `internal/session` | Session lifecycle, command policy, output ring buffers, idle detection |
| `internal/parser` | ANSI stripping, output segmentation, message classification |
| `internal/hub` | WebSocket client hub, subscriptions, rate-limited output broadcasting |
| `internal/orchestrator` | Claude tool-calling loop, prompt building, scheduler enforcement |
| `internal/automation` | Auto-commit, review coordinator, auto-merge, conflict escalation |
| `internal/registry` | YAML-backed agent registry |
| `internal/playbook` | YAML-backed playbook registry, project matching |
| `internal/git` | Worktree operations, status/log helpers |
| `frontend` | React 18 + TypeScript + Vite SPA |

---

## Requirements

- **Go** 1.22 or later
- **Node.js** 18+ and **npm** (for frontend development only; pre-built assets are embedded)
- **tmux** 3.0+
- **git** 2.5+ (for worktree support)
- An **Anthropic API key** if using the orchestrator / PM Chat

---

## Installation

### From Source

```bash
git clone https://github.com/user/agenterm.git
cd agenterm
make build
```

The binary will be at `bin/agenterm`. It embeds the pre-built React frontend — no separate deployment step needed.

### Direct Run (no build step)

```bash
go run ./cmd/agenterm
```

---

## Quick Start

### 1. Create a tmux session

agenterm attaches to an existing tmux session (or creates one on first use).

```bash
tmux new-session -d -s ai-coding
```

### 2. Start agenterm

```bash
./bin/agenterm
```

On first run a token is generated and saved to `~/.config/agenterm/config`. The URL is printed to stdout:

```
agenterm listening on http://localhost:8765
Token: abc123...
```

Use `--print-token` to retrieve the token later:

```bash
./bin/agenterm --print-token
```

### 3. Open in browser

```
http://localhost:8765?token=<your-token>
```

Pass the token in the URL once — it is persisted in `localStorage` for future visits.

### 4. (Optional) Set your Anthropic API key for PM Chat

```bash
./bin/agenterm --llm-api-key sk-ant-...
```

Or export `ANTHROPIC_API_KEY` in your shell before starting agenterm.

---

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8765` | HTTP server port |
| `--session` | `ai-coding` | tmux session name to attach |
| `--token` | auto-generated | Bearer token for authentication |
| `--print-token` | `false` | Print the current token to stdout and exit |
| `--dir` | `~/08Coding` | Default working directory for new projects |
| `--db-path` | `~/.config/agenterm/agenterm.db` | SQLite database path |
| `--agents-dir` | `~/.config/agenterm/agents` | Directory for agent YAML definitions |
| `--playbooks-dir` | `~/.config/agenterm/playbooks` | Directory for playbook YAML definitions |
| `--llm-api-key` | `$ANTHROPIC_API_KEY` | Anthropic API key for orchestrator |
| `--llm-model` | `claude-sonnet-4-5` | LLM model for orchestrator |
| `--llm-base-url` | Anthropic default | Override LLM API base URL |
| `--orchestrator-global-max-parallel` | `32` | Global cap on concurrent agent sessions |

### Config File

Location: `~/.config/agenterm/config` (created automatically, `0600` permissions)

```ini
Port=8765
TmuxSession=ai-coding
Token=your-secret-token
LLMApiKey=sk-ant-...
AgentsDir=/home/user/.config/agenterm/agents
PlaybooksDir=/home/user/.config/agenterm/playbooks
```

**Precedence**: CLI flags > config file > built-in defaults

---

## Usage Guide

### PM Chat — Talk to the Orchestrator

The **PM Chat** page (`/pm-chat`) is a chat interface backed by a Claude tool-calling loop. The orchestrator has access to the full API — it can read your projects and tasks, create new sessions, check worktree status, and drive work forward.

**Example prompts:**
- *"What tasks are currently in progress for the auth project?"*
- *"Start a new implementation session for task-42 using the codex agent"*
- *"The review on task-17 passed — merge the worktree"*

The orchestrator streams responses token-by-token over WebSocket (`/ws/orchestrator`). Tool calls and their results are shown inline. The full conversation history is persisted and replayed when you reconnect.

**Voice input**: click the microphone button to record speech. Audio is transcribed via the ASR backend and pasted into the input field. Configure ASR credentials in **Settings → ASR**.

### Sessions — Live Agent Terminals

The **Sessions** page shows every active agent session. Click a session to open its live output view:

- Output streams in real time, classified into prompts, errors, code, and normal text
- **Quick actions** appear automatically for common prompts (`[Y/n]`, `Ctrl+C`, `Continue`)
- Type in the input bar and press `Enter` to send a command to the agent
- Use `Shift+Enter` for multi-line input

Sessions are scoped to tasks. Each task can have one or more sessions, each mapped to a separate tmux window.

### Settings — Agents, Playbooks & ASR

The **Settings** page has three tabs:

#### Agent Registry

Define the AI agents available to the orchestrator. Each agent is a YAML file in `--agents-dir` and editable via the UI.

Example agent definition:

```yaml
id: claude-code
name: Claude Code
command: claude --dangerously-skip-permissions
model: claude-sonnet-4-5
cost_tier: medium
speed_tier: medium
capabilities:
  - implement
  - debug
  - refactor
languages:
  - go
  - typescript
  - python
supports_session_resume: true
supports_headless: true
```

#### Playbooks

Playbooks define stage-based workflows (`plan`, `build`, `test`) matched to your projects. The orchestrator selects the matching playbook when it spawns agents for a task.

Example playbook definition:

```yaml
id: pairing-coding
name: Pairing Coding
description: Pair-style flow with planner + codebase reader, then builder + reviewer.
parallelism_strategy: Keep planning focused, then run builder/reviewer loops.
match:
  languages: []
  project_patterns: []
workflow:
  plan:
    enabled: true
    roles:
      - name: planner
        responsibilities: Clarify scope and milestones.
        allowed_agents: [claude-code, codex]
      - name: codebase-reader
        responsibilities: Explore repository and constraints.
        allowed_agents: [claude-code, codex]
  build:
    enabled: true
    roles:
      - name: builder
        responsibilities: Implement scoped changes.
        allowed_agents: [codex]
      - name: reviewer
        responsibilities: Review and request fixes.
        allowed_agents: [claude-code, codex]
  test:
    enabled: true
    roles:
      - name: tester
        responsibilities: Run tests and validate acceptance criteria.
        allowed_agents: [claude-code, codex]
```

#### ASR (Speech-to-Text)

Configure Volcengine ASR credentials for voice input in PM Chat. Credentials are stored in `localStorage` and sent directly to the backend transcription endpoint — they are never stored server-side.

### Human Takeover

Any session can be taken over manually. Click **Take Over** in the session view to:

1. Pause the automation loop for that session
2. Enable direct keyboard input
3. Receive full terminal output including ANSI sequences

Click **Release** to hand control back to the agent.

### Remote Access via Tailscale

agenterm has no built-in TLS. The recommended pattern for remote access is [Tailscale](https://tailscale.com/):

1. Install Tailscale on your dev machine and your phone/laptop
2. Start agenterm on the dev machine
3. Access from any device on your tailnet: `http://100.x.x.x:8765?token=...`

All traffic is encrypted by WireGuard — no port forwarding or reverse proxy needed.

---

## Agent System

Agents are defined as YAML files in `--agents-dir` (default `~/.config/agenterm/agents/`). The registry is hot-reloaded at startup.

### Agent Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier (used in playbooks and sessions) |
| `name` | string | Display name |
| `command` | string | Shell command to launch the agent (e.g. `claude`, `codex`) |
| `model` | string | LLM model identifier |
| `cost_tier` | `low`/`medium`/`high` | Used for scheduling decisions |
| `speed_tier` | `slow`/`medium`/`fast` | Used for scheduling decisions |
| `capabilities` | string[] | e.g. `implement`, `review`, `debug`, `research` |
| `languages` | string[] | Programming languages the agent handles well |
| `max_parallel_agents` | int | Max concurrent instances of this agent |
| `supports_session_resume` | bool | Whether the agent can resume interrupted sessions |
| `supports_headless` | bool | Whether the agent runs without user interaction |
| `notes` | string | Free-form notes for the orchestrator |

---

## Skills Protocol

agenterm now uses a filesystem-based skill protocol for orchestrator guidance, compatible with the Agent Skills style:

- Skill location: `skills/<skill-id>/SKILL.md`
- Format: YAML frontmatter + markdown body
- Frontmatter keys: `name`, `description`
- Progressive disclosure:
  - orchestrator prompt includes only skill summaries
  - orchestrator calls tools to fetch full details only when needed

Built-in disclosure tools:

- `list_skills` — returns discovered skill summaries
- `get_skill_details(skill_id)` — returns full skill body for one selected skill

Discovered roots (searched upward from current working directory):

- `skills/`
- `.agents/skills/`
- `.claude/skills/`

Reference:
- https://agentskills.io/
- https://github.com/anthropics/skills

---

## Playbook System

Playbooks are YAML files in `--playbooks-dir`. The orchestrator matches a playbook to a project using the `match` block.

### Matching

- **`match.languages`**: checked against the project's detected languages
- **`match.project_patterns`**: glob patterns matched against the project's `repo_path`

The first matching playbook is selected. If no playbook matches, the orchestrator falls back to first available playbook.

### Workflow Stages

Each playbook uses fixed stages:

- `plan`
- `build`
- `test`

Each stage has:

- `enabled`
- `roles[]`

Each role defines:

- `name`
- `responsibilities`
- `allowed_agents` (agent IDs)
- `suggested_prompt` (optional)

Current shipped templates:

- `pairing-coding`
- `tdd-coding`
- `compound-engineering-workflow`

---

## Review Cycles

agenterm has a structured review model for tracking code review iterations:

- A **review cycle** is created for a task when it reaches `ready_for_review` status
- Each cycle has a commit hash, status (`open`/`passed`/`failed`), and a collection of **review issues**
- Each issue has a severity (`critical`/`major`/`minor`/`suggestion`) and status (`open`/`resolved`/`wont_fix`)
- The orchestrator and automation layer gate merges on `open` critical/major issues

The automation review coordinator watches for `[READY_FOR_REVIEW]` in commit messages, creates the cycle, prompts a reviewer agent, and feeds results back to the implementing agent.

---

## REST API

All endpoints require a `Bearer` token in the `Authorization` header (or `?token=` query parameter).

### Projects
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects` | List all projects |
| `POST` | `/api/projects` | Create a project |
| `GET` | `/api/projects/{id}` | Get project |
| `PATCH` | `/api/projects/{id}` | Update project |
| `DELETE` | `/api/projects/{id}` | Delete project |

### Tasks
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/projects/{id}/tasks` | List tasks for project |
| `POST` | `/api/projects/{id}/tasks` | Create task |
| `GET` | `/api/tasks/{id}` | Get task |
| `PATCH` | `/api/tasks/{id}` | Update task |

### Sessions
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/tasks/{id}/sessions` | Create session for task |
| `GET` | `/api/sessions` | List sessions (filterable by status, task, project) |
| `GET` | `/api/sessions/{id}` | Get session |
| `POST` | `/api/sessions/{id}/send` | Send command to session |
| `GET` | `/api/sessions/{id}/output` | Get buffered output |
| `GET` | `/api/sessions/{id}/idle` | Check idle state |
| `PATCH` | `/api/sessions/{id}/takeover` | Toggle human takeover |
| `DELETE` | `/api/sessions/{id}` | Destroy session |

### Worktrees
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/projects/{id}/worktrees` | Create worktree for project |
| `GET` | `/api/worktrees/{id}/git-status` | Get git status |
| `GET` | `/api/worktrees/{id}/git-log` | Get git log |
| `DELETE` | `/api/worktrees/{id}` | Remove worktree |

### Agents & Playbooks
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/agents` | List agents |
| `POST` | `/api/agents` | Create agent |
| `PUT` | `/api/agents/{id}` | Update agent |
| `DELETE` | `/api/agents/{id}` | Delete agent |
| `GET` | `/api/playbooks` | List playbooks |
| `POST` | `/api/playbooks` | Create playbook |
| `PUT` | `/api/playbooks/{id}` | Update playbook |
| `DELETE` | `/api/playbooks/{id}` | Delete playbook |

### Orchestrator
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/orchestrator/chat` | Send a message to the orchestrator |
| `GET` | `/api/orchestrator/history` | Get conversation history |
| `GET` | `/api/orchestrator/report` | Get current orchestrator status report |
| `GET` | `/api/projects/{id}/orchestrator` | Get per-project orchestrator config |
| `PATCH` | `/api/projects/{id}/orchestrator` | Update per-project orchestrator config |

### Review
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/tasks/{id}/review-cycles` | List review cycles |
| `POST` | `/api/tasks/{id}/review-cycles` | Create review cycle |
| `PATCH` | `/api/review-cycles/{id}` | Update review cycle |
| `GET` | `/api/review-cycles/{id}/issues` | List issues |
| `POST` | `/api/review-cycles/{id}/issues` | Create issue |
| `PATCH` | `/api/review-issues/{id}` | Update issue |

### Other
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/asr/transcribe` | Transcribe audio (multipart/form-data) |
| `GET` | `/api/projects/{id}/knowledge` | List project knowledge entries |
| `POST` | `/api/projects/{id}/knowledge` | Add knowledge entry |
| `GET` | `/api/projects/{id}/role-bindings` | Get role→model bindings |
| `PUT` | `/api/projects/{id}/role-bindings` | Replace role bindings |
| `GET` | `/api/workflows` | List workflows |
| `POST` | `/api/workflows` | Create workflow |

---

## WebSocket Protocol

### `/ws` — Terminal Events

Subscribe to live session output and status events. Authenticate via `?token=` query parameter.

**Incoming message types** (server → browser):

```jsonc
{ "type": "output",        "sessionID": "...", "lines": ["..."] }
{ "type": "status",        "sessionID": "...", "status": "running" }
{ "type": "windows",       "windows": [...] }
{ "type": "project_event", "projectID": "...", "event": "task_updated", "data": {...} }
```

**Outgoing message types** (browser → server):

```jsonc
{ "type": "subscribe",   "sessionID": "..." }
{ "type": "unsubscribe", "sessionID": "..." }
{ "type": "send",        "sessionID": "...", "text": "ls -la\n" }
```

### `/ws/orchestrator` — PM Chat Streaming

Streams the orchestrator's response token-by-token.

```jsonc
{ "type": "token",       "content": "Sure, let me check..." }
{ "type": "tool_call",   "name": "list_tasks", "input": {...} }
{ "type": "tool_result", "name": "list_tasks", "content": "..." }
{ "type": "done" }
{ "type": "error",       "message": "..." }
```

---

## Development

### Prerequisites

```bash
# Install Go 1.22+
# Install Node.js 18+ and npm
# Install tmux 3.0+
```

### Build

```bash
make build          # Build frontend + Go binary → bin/agenterm
make frontend-build # Build React SPA only → web/frontend/dist/
make run            # go run ./cmd/agenterm
make clean          # Remove bin/
```

### Test

```bash
go test ./...       # All Go tests
go vet ./...        # Vet
```

### Frontend Development

The React app lives in `frontend/`. During development, run the Vite dev server alongside the Go server:

```bash
cd frontend
npm install
npm run dev         # Starts on http://localhost:5173
```

Point your browser to the Vite dev server. API requests proxy to the Go backend via `vite.config.ts`.

When ready to embed:

```bash
make frontend-build   # Outputs to web/frontend/dist/
make build            # Go embeds dist/ into the binary
```

---

## Project Structure

```
agenterm/
├── cmd/agenterm/
│   └── main.go                  # Entry point; wires all components
├── internal/
│   ├── api/                     # REST handlers + middleware
│   │   ├── router.go            # Route registration
│   │   ├── orchestrator.go      # /api/orchestrator/* handlers
│   │   ├── asr.go               # /api/asr/transcribe handler
│   │   └── asr_volc.go          # Volcengine ASR client
│   ├── automation/              # Auto-commit, review loops, merge
│   ├── config/                  # Flag + config-file loading
│   ├── db/                      # SQLite repositories + migrations
│   │   ├── models.go            # All entity types
│   │   └── migrations/          # SQL migration files
│   ├── git/                     # Worktree and git helpers
│   ├── hub/                     # WebSocket hub + protocol
│   ├── orchestrator/            # Claude tool-calling orchestrator
│   ├── parser/                  # Output classifier + ANSI stripper
│   ├── playbook/                # Playbook registry + project matching
│   ├── registry/                # Agent registry (YAML)
│   ├── server/                  # HTTP server + WS endpoint wiring
│   ├── session/                 # Session lifecycle + command policy
│   └── tmux/                    # tmux control-mode gateway/manager
├── frontend/
│   ├── src/
│   │   ├── api/                 # API client + TypeScript types
│   │   ├── components/          # Shared React components
│   │   ├── hooks/               # Custom hooks (WS, STT, etc.)
│   │   ├── pages/               # Route-level pages
│   │   │   ├── PMChat.tsx       # PM Chat page
│   │   │   ├── Sessions.tsx     # Session list + terminal view
│   │   │   └── Settings.tsx     # Agents / Playbooks / ASR
│   │   └── settings/            # ASR settings helpers
│   ├── vite.config.ts
│   └── package.json
├── web/
│   ├── embed.go                 # go:embed directive
│   └── frontend/dist/           # Built React app (embedded in binary)
├── docs/                        # Task specs, plans, architecture docs
├── Makefile
├── go.mod
└── SPEC.md                      # Product specification (Chinese)
```

---

## Security

- **Token authentication** — every HTTP request and WebSocket connection requires a valid bearer token
- **Token generated on first run** — stored in `~/.config/agenterm/config` with `0600` permissions; never printed to stdout unless `--print-token` is used
- **Command policy** — the session manager blocks dangerous shell patterns (eval, subshell substitution, path traversal, out-of-scope paths) on commands sent via the API
- **CORS** — default policy allows all origins; restrict in production if exposing beyond localhost
- **ASR credentials** — stored in browser `localStorage` only; transmitted over the existing authenticated HTTP connection and never persisted server-side

### Recommendations

- Run behind [Tailscale](https://tailscale.com/) for remote access — no open ports needed
- Do not expose agenterm directly to the public internet without a TLS reverse proxy
- Rotate the token by deleting `~/.config/agenterm/config` and restarting

---

## Contributing

Contributions are welcome. Please:

1. Fork the repository and create a feature branch
2. Follow existing code conventions (Go: `gofmt`, `go vet`; TypeScript: Prettier/ESLint config in `frontend/`)
3. Add or update tests for any changed behaviour in `internal/`
4. Open a pull request with a clear description

For significant features, open an issue first to discuss the approach.

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Acknowledgements

Built with:
- [nhooyr.io/websocket](https://nhooyr.io/websocket) — zero-dependency Go WebSocket
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — pure-Go SQLite (no CGO)
- [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) — YAML parsing for agent/playbook registries
- [React](https://react.dev/) + [Vite](https://vitejs.dev/) — frontend toolchain
- [@xterm/xterm](https://xtermjs.org/) — terminal emulator in the browser
- [tmux](https://github.com/tmux/tmux) — the runtime that makes it all possible
