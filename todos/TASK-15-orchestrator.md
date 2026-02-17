---
status: complete
priority: p1
issue_id: "TASK-15"
tags: [orchestrator, api, websocket, llm]
dependencies: [TASK-08, TASK-09, TASK-11, TASK-12]
---

# Implement TASK-15 Orchestrator

## Tasks

- [x] Add orchestrator core package (`internal/orchestrator/*`) with tool schemas, prompt builder, tool loop, event triggers.
- [x] Add conversation persistence in DB for per-project bounded history.
- [x] Add orchestrator API endpoints (`POST /api/orchestrator/chat`, `GET /api/orchestrator/report`) and router wiring.
- [x] Add orchestrator WebSocket channel (`/ws/orchestrator`) with token/tool-call/tool-result/done events.
- [x] Add config for LLM API key/model/base URL.
- [x] Wire orchestrator and event triggers in `cmd/agenterm/main.go`.
- [x] Add tests for prompt/tool schema/tool loop/events/API/WS/history persistence.
- [x] Update acceptance checklist in `docs/TASK-15-orchestrator.md`.
