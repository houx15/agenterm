# agenterm

A web-based terminal session manager that bridges tmux sessions to a mobile-friendly chat UI via WebSocket.

**Transform terminal windows into conversational AI agent sessions with cross-device, asynchronous programming collaboration.**

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

## Features

- **Multi-Session Management** — Monitor and manage multiple tmux windows simultaneously
- **Chat-Style Interface** — Terminal output transformed into chat messages with smart segmentation
- **Smart Message Classification** — Auto-detect prompts, errors, code blocks, and generate quick action buttons
- **Mobile-First Design** — Responsive UI optimized for touch interaction on phones and tablets
- **Real-Time Bidirectional** — Instant terminal output push and command delivery via WebSocket
- **Secure by Default** — Token-based authentication, secrets hidden by default
- **Zero Frontend Build** — Single HTML file with embedded CSS/JS, served via `go:embed`
- **Cross-Platform** — Works on macOS, Linux, anywhere tmux runs

## Requirements

- Go 1.22 or later
- [tmux](https://github.com/tmux/tmux) 3.0+

## Installation

### From Source

```bash
git clone https://github.com/user/agenterm.git
cd agenterm
make build
```

The binary will be at `bin/agenterm`.

### Direct Run

```bash
go run ./cmd/agenterm
```

## Quick Start

### 1. Create a tmux session

```bash
tmux new-session -s ai-coding
```

### 2. Start agenterm

```bash
./bin/agenterm
```

Or with custom options:

```bash
./bin/agenterm --port 9000 --session my-session
```

### 3. Open in browser

```
http://localhost:8765?token=<your-token>
```

On first run, agenterm generates a token and saves it to `~/.config/agenterm/config`. Use `--print-token` to reveal it:

```bash
./bin/agenterm --print-token
```

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 8765 | Server port (1-65535) |
| `--session` | ai-coding | tmux session name to attach |
| `--token` | auto-generated | Authentication token |
| `--print-token` | false | Print token to stdout |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AGENTERM_PORT` | Server port |
| `AGENTERM_SESSION` | tmux session name |
| `AGENTERM_TOKEN` | Authentication token |

**Precedence:** CLI flags > environment variables > config file > defaults

### Config File

Location: `~/.config/agenterm/config`

Format (simple key=value):

```
Port=8765
TmuxSession=ai-coding
Token=your-secret-token-here
```

## Usage

### Remote Access with Tailscale

agenterm works great with [Tailscale](https://tailscale.com/) for secure remote access:

1. Install Tailscale on both machines
2. Start agenterm on your dev machine
3. Access from your phone via Tailscale IP: `http://100.x.x.x:8765?token=...`

All traffic is encrypted through WireGuard — no port forwarding needed.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send command |
| `Shift+Enter` | New line in input |
| `←` (mobile) | Back to session list |

### Quick Actions

When agenterm detects prompts like `[Y/n]` or `Do you want to continue?`, it automatically shows action buttons:

- **Yes/No** — For confirmation dialogs
- **Continue** — For generic prompts
- **Ctrl+C** — To cancel/interrupt

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Browser                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Sidebar   │  │  Chat View  │  │     Input Bar       │  │
│  │ (Sessions)  │  │ (Messages)  │  │   (Commands)        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└───────────────────────────┬─────────────────────────────────┘
                            │ WebSocket
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      agenterm (Go)                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ HTTP Server │  │   Hub       │  │   Output Parser     │  │
│  │ (go:embed)  │  │ (WebSocket) │  │ (Classification)    │  │
│  └─────────────┘  └──────┬──────┘  └─────────────────────┘  │
│                          │                                   │
│                   ┌──────▼──────┐                           │
│                   │Tmux Gateway │                           │
│                   │(Control Mode)│                          │
│                   └──────┬──────┘                           │
└──────────────────────────┼──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                       tmux                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  Window @0  │  │  Window @1  │  │  Window @2  │  ...    │
│  │  Claude     │  │  Codex      │  │  Kimi       │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

| Package | Description |
|---------|-------------|
| `cmd/agenterm` | Entry point, startup, shutdown |
| `internal/config` | Configuration loading (flags/env/file) |
| `internal/server` | HTTP server, static file serving, WebSocket endpoint |
| `internal/hub` | WebSocket hub, client management, message broadcasting |
| `internal/tmux` | tmux Control Mode protocol, gateway lifecycle |
| `internal/parser` | Output parsing, message classification, ANSI stripping |
| `web` | Frontend (single HTML file with embedded CSS/JS) |

## Development

### Build

```bash
make build   # Build binary to bin/agenterm
make run     # Run directly with go run
make clean   # Remove bin/
```

### Test

```bash
go test ./...
go vet ./...
```

### Project Structure

```
agenterm/
├── cmd/agenterm/main.go      # Entry point
├── internal/
│   ├── config/config.go      # Configuration
│   ├── server/server.go      # HTTP/WebSocket server
│   ├── hub/
│   │   ├── hub.go            # WebSocket hub
│   │   └── protocol.go       # Message types
│   ├── tmux/
│   │   ├── gateway.go        # tmux gateway
│   │   ├── protocol.go       # Control Mode parser
│   │   └── types.go          # Event types
│   └── parser/
│       ├── parser.go         # Output parser
│       ├── patterns.go       # Regex patterns
│       ├── ansi.go           # ANSI stripping
│       └── types.go          # Message types
├── web/
│   ├── embed.go              # go:embed directive
│   └── index.html            # Frontend (single file)
├── go.mod
├── Makefile
└── README.md
```

## Security

- **Token Authentication** — All WebSocket connections require a valid token
- **Token Hidden by Default** — Tokens are not printed to logs/stdout unless `--print-token` is used
- **Config File Permissions** — Config file is saved with `0600` permissions
- **No External Dependencies** — Minimal attack surface with only one dependency (websocket)

### Recommendations

- Use with [Tailscale](https://tailscale.com/) or similar VPN for remote access
- Don't expose directly to the public internet
- Regenerate tokens if compromised (delete config file and restart)

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Built with:
- [nhooyr.io/websocket](https://nhooyr.io/websocket) — WebSocket implementation
- [tmux](https://github.com/tmux/tmux) — Terminal multiplexer
