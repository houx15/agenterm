---
status: complete
priority: p1
issue_id: "TASK-16"
tags: [automation, session, hooks, coordinator, review]
dependencies: [TASK-11, TASK-12, TASK-15]
---

# Implement TASK-16 Automation

## Tasks

- [x] Add automation package with auto-commit service, coordinator flow, and Claude hook generation.
- [x] Inject Claude hooks during session creation for `agent_type=claude-code` worktrees.
- [x] Wire automation services in `cmd/agenterm/main.go` startup flow.
- [x] Integrate human terminal attach/detach callbacks to pause/resume automation.
- [x] Enhance session monitor completion checks cadence for marker/git/tmux signals.
- [x] Add tests for hooks generation, auto-commit/coordinator core behavior, and monitor/takeover integration.
- [x] Update acceptance checklist in `docs/TASK-16-automation.md`.

## Notes

- Keep auto-commit safe: skip when merge conflicts/unmerged paths exist.
- Review loop default max iterations: 3 (configurable in coordinator constructor).
- Automation must respect `session.status = human_takeover`.
