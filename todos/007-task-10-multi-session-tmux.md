# TASK-10 multi-session-tmux

- [x] Add `internal/tmux/manager.go` with multi-session lifecycle APIs
- [x] Refactor `internal/tmux/gateway.go` for per-instance `SessionName()` and idempotent `Close()`
- [x] Add `session_id` to hub protocol and route callbacks by session + window
- [x] Wire `cmd/agenterm/main.go` to use `tmux.Manager` with default-session auto-attach
- [x] Update `web/index.html` to send/receive session-scoped messages
- [x] Add tests for manager and session-aware hub protocol routing
- [x] Run targeted test suites in sandbox-compatible mode
