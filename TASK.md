# Task: project-scaffold

## Context
Agenterm is a web-based terminal session manager written in Go. It bridges tmux sessions to a mobile-friendly chat UI via WebSocket. This is a greenfield project — no existing code beyond the design document.

**Tech stack:** Go 1.22+, no external framework, `go:embed` for frontend assets.

## Objective
Set up the Go project skeleton with proper module structure, build tooling, and a minimal "hello world" HTTP server that serves an embedded HTML page.

## Dependencies
- Depends on: none
- Branch: feature/01-project-scaffold
- Base: main

## Scope

### Files to Create
- `go.mod` — Go module definition (`github.com/user/agenterm`, go 1.22)
- `cmd/agenterm/main.go` — Entry point: parse flags, start server
- `internal/config/config.go` — Configuration struct + loading from flags/env/file
- `internal/server/server.go` — HTTP server setup, static file serving, WebSocket upgrade endpoint (stub)
- `web/index.html` — Minimal HTML placeholder (just "agenterm loading...")
- `web/embed.go` — `go:embed` directive to embed web/ directory
- `Makefile` — build, run, clean targets
- `.gitignore` — Go-specific ignores

### Files to Modify
- None (greenfield)

### Files NOT to Touch
- `architecture.md` — Reference only
- `docs/plans/*` — Reference only

## Implementation Spec

### Step 1: Initialize Go Module
Create `go.mod` with module path `github.com/user/agenterm` (placeholder, user can rename). Set Go version to 1.22.

### Step 2: Configuration
Create `internal/config/config.go`:
```go
type Config struct {
    Port        int    // --port, default 8765
    TmuxSession string // --session, default "ai-coding"
    Token       string // --token, auto-generated if empty
    ConfigPath  string // ~/.config/agenterm/config.toml
}
```
- Parse from command-line flags (using `flag` package, no external deps)
- If no token provided, generate a random 32-char hex string, save to config file, print to stdout
- Config file location: `~/.config/agenterm/config.toml` (create dir if not exists)
- Use a simple key=value format for config file (no TOML library needed for v1 — just `Port=8765\nToken=abc123`)

### Step 3: Embed Frontend
Create `web/embed.go`:
```go
package web

import "embed"

//go:embed index.html
var Assets embed.FS
```

Create `web/index.html` — minimal HTML that shows "agenterm" title and a connection status placeholder.

### Step 4: HTTP Server
Create `internal/server/server.go`:
- `func New(cfg *config.Config) *Server`
- Serve `web.Assets` at `/`
- Stub WebSocket endpoint at `/ws` (just accept and close for now)
- Bind to `0.0.0.0:<port>`
- Log startup message with URL and token

### Step 5: Entry Point
Create `cmd/agenterm/main.go`:
- Parse flags
- Load/create config
- Print banner with URL: `agenterm running at http://localhost:8765?token=<token>`
- Start HTTP server
- Handle SIGINT/SIGTERM for graceful shutdown

### Step 6: Build Tooling
Create `Makefile`:
- `build`: `go build -o bin/agenterm ./cmd/agenterm`
- `run`: `go run ./cmd/agenterm`
- `clean`: `rm -rf bin/`

Create `.gitignore`:
- `bin/`, `*.exe`, `.DS_Store`

## Testing Requirements
- `go build ./...` succeeds
- `go vet ./...` passes
- Running the binary starts an HTTP server on the configured port
- Visiting `http://localhost:8765` shows the placeholder HTML
- Token is auto-generated on first run and printed to stdout
- Ctrl+C gracefully shuts down the server

## Acceptance Criteria
- [ ] `go build ./cmd/agenterm` produces a working binary
- [ ] Binary serves embedded HTML at root
- [ ] Token auto-generation works
- [ ] Graceful shutdown on SIGINT
- [ ] No external dependencies (stdlib only for this task)

## Notes
- Keep the config file format dead simple — avoid pulling in TOML/YAML libraries. A simple `key=value` format is fine for v1.
- The WebSocket endpoint is a stub — feature/02-tmux-gateway and feature/04-websocket-hub will implement it.
- Use `log/slog` for structured logging (stdlib since Go 1.21).
