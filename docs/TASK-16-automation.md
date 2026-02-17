# Task: automation

## Context
The SPEC defines several automation mechanisms that make the system autonomous: auto-commit hooks, idle/completion detection, and the coordinator flow (coder → reviewer → feedback loop). These automations reduce the need for human intervention in the standard development workflow.

## Objective
Implement auto-commit hooks, the coordinator coder↔reviewer flow, and the human takeover mechanism as described in SPEC Sections 5.2-5.4.

## Dependencies
- Depends on: TASK-12 (session-lifecycle), TASK-11 (worktree-management), TASK-15 (orchestrator)
- Branch: feature/automation
- Base: main (after dependencies merge)

## Scope

### Files to Create
- `internal/automation/autocommit.go` — Auto-commit: periodic check + commit for worktrees
- `internal/automation/coordinator.go` — Coordinator: monitors coder output, sends diffs to reviewer, relays feedback
- `internal/automation/hooks.go` — Generate Claude Code hook configs for worktrees
- `scripts/auto-commit.sh` — Shell script for auto-commit (injected into worktrees)
- `scripts/on-agent-stop.sh` — Shell script for agent completion (injected into worktrees)

### Files to Modify
- `internal/session/monitor.go` — Enhanced idle detection with marker file and git commit checks
- `cmd/agenterm/main.go` — Start automation goroutines

### Files NOT to Touch
- `internal/tmux/` — Use via session manager
- `web/` — No frontend changes for this task

## Implementation Spec

### Step 1: Auto-commit service
```go
type AutoCommitter struct {
    interval time.Duration  // Default: 30s
    worktrees map[string]string  // worktree_id → path
}

func (ac *AutoCommitter) Run(ctx context.Context) {
    // Every interval:
    // 1. For each active worktree:
    //    git -C <path> status --porcelain
    // 2. If changes exist:
    //    git -C <path> add -A
    //    git -C <path> commit -m "[auto] <summary>"
    // 3. If commit has changes related to TASK.md completion:
    //    Notify orchestrator
}
```

### Step 2: Claude Code hooks generation
When creating a session with agent_type=claude-code, auto-inject:
```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "Write|Edit",
      "hooks": [{"type": "command", "command": ".orchestra/hooks/auto-commit.sh"}]
    }],
    "Stop": [{
      "hooks": [{"type": "command", "command": ".orchestra/hooks/on-agent-stop.sh"}]
    }]
  }
}
```
Write this to `<worktree>/.claude/settings.json`.

### Step 3: Coordinator flow
```go
type Coordinator struct {
    sessionMgr  *session.Manager
    orchestrator *orchestrator.Orchestrator
}

// MonitorCoderSession watches a coder session for review-ready commits
func (c *Coordinator) MonitorCoderSession(ctx context.Context, coderSessionID string, reviewerSessionID string) {
    // 1. Watch git log for commits with [READY_FOR_REVIEW]
    // 2. When detected:
    //    a. Get git diff HEAD~1
    //    b. Read TASK.md from worktree
    //    c. Compose review prompt: TASK.md + diff
    //    d. Send to reviewer session
    // 3. Watch reviewer session output for completion
    // 4. Parse review result:
    //    - If "APPROVED" / "LGTM" → mark task done
    //    - If suggestions → send feedback to coder session
    // 5. Loop until approved or max iterations
}
```

### Step 4: Human takeover integration
Enhance WebSocket attach/detach detection:
```go
// In hub/client.go - when a client subscribes to a session's terminal:
func (c *Client) onTerminalAttach(sessionID string) {
    // 1. Set session.human_attached = true in DB
    // 2. Set session.status = "human_takeover"
    // 3. Pause coordinator for this session
    // 4. Pause auto-commit for this worktree
    // 5. Broadcast status change
}

func (c *Client) onTerminalDetach(sessionID string) {
    // 1. Show confirmation dialog: "Return control to PM?"
    // 2. If yes: reset human_attached, resume automation
    // 3. Broadcast status change
}
```

### Step 5: Completion detection enhancement
Add to session monitor:
- Check for `.orchestra/done` marker file every 5s
- Check git log for `[READY_FOR_REVIEW]` commits
- Check if agent process has exited (tmux window closed)

## Acceptance Criteria
- [x] Auto-commit runs periodically for active worktrees
- [x] Claude Code hooks injected on session creation
- [x] Coordinator detects review-ready commits and sends to reviewer
- [x] Reviewer feedback relayed back to coder
- [x] Human takeover pauses all automation for the session
- [x] Returning control resumes automation
- [x] Completion detection works with marker files and git commits

## Notes
- Auto-commit should NOT commit if there's a merge conflict
- The coordinator is essentially the "glue" between coder and reviewer sessions
- Max review iterations should be configurable (default: 3)
- Human takeover should show a clear visual indicator in the UI
- Auto-commit messages should be distinguishable from agent commits
