# Orchestrator Required Skills

## Goal
Define the minimum skill set the orchestrator must have to reliably run multi-agent, multi-worktree software delivery.

## Core Execution Skills

### 1) Model Selection And Allocation
- Select models/agents by role requirements, allowed agents in playbook stage, available pool capacity, and target worktree count.
- Produce a clear selection report before execution:
  - role -> selected agent/model
  - count per role
  - blocked roles (if any) and reason

Inputs:
- playbook stage config (`roles`, `allowed_agents`)
- live agent registry / capacity
- requested parallelism and worktree plan

Outputs:
- allocation plan
- blocking constraints

### 2) Session Bootstrap In Correct Context
- Create tmux session/window for selected worker.
- Enter correct working directory (project root or specific worktree).
- Start the correct TUI command for selected agent/model.

Inputs:
- task/worktree context
- agent command templates

Outputs:
- `session_id`, tmux metadata, startup status

### 3) Prompt/Command Dispatch
- Send structured prompts/commands to a specific session via tmux.
- Enforce explicit target session to avoid command chaos.

Inputs:
- `session_id`
- prompt/command text

Outputs:
- command ledger entry (`issued`, `running`, `done`, `failed`)

### 4) Session Output Read
- Read latest output from a specific session.
- Return raw output and summarized status for orchestration decisions.

Inputs:
- `session_id`, line/window limits

Outputs:
- output chunk
- summary hints (idle/running/waiting/error)

### 5) Session Close/Teardown
- Safely close a session when role work ends or is aborted.
- Ensure resources/slots are released.

Inputs:
- `session_id`

Outputs:
- closed status
- released capacity info

### 6) Worktree Provisioning
- Create worktrees according to planning results and phase/task decomposition.
- Map worktree -> task -> assigned role(s).

Inputs:
- project path
- branch/worktree plan

Outputs:
- worktree records and paths

### 7) Commit Execution
- Create commits in assigned worker branch with policy-compliant messages.
- Attach commit metadata for review workflow.

Inputs:
- session/worktree context
- commit policy

Outputs:
- commit hash, changed files summary

### 8) Branch Merge
- Merge worktree branches into target branch with policy checks.
- Handle clean merges and conflict flows.

Inputs:
- source worktree/branch
- target branch

Outputs:
- merge result, conflict status

## Additional Required Reliability Skills

### 9) Stage Gate And Approval
- Before mutating actions or phase transitions, propose execution and wait for explicit user confirmation.
- Mandatory for `plan -> build -> test` transitions.

### 10) Capacity Scheduler
- Enforce limits by global/project/agent/role.
- Reject over-allocation with structured reasons.

### 11) Per-Session Command Queue + Ledger
- Serialize commands per session.
- Track command lifecycle for audit/debug/retry safety.

### 12) Session Health And Recovery
- Detect stuck/crashed sessions, perform controlled restart/resume, avoid duplicate effects.

### 13) Test And Quality Gates
- Require test/review checks before merge/close.
- Block progression on unresolved issues.

### 14) Conflict Resolution Loop
- Detect merge conflicts, assign resolver, verify merge clean-up completion.

### 15) Project Memory Management
- Read/write durable memory artifacts:
  - plans
  - decisions
  - review findings
  - final summaries

### 16) Human Takeover And Resume
- Support pause/manual intervention/resume without losing orchestration state.

### 17) Branch/Policy Guardrails
- Enforce branch naming, commit format, protected-branch and merge policies.

### 18) Cost/Time/Utilization Reporting
- Provide per-stage resource usage:
  - model usage
  - worker utilization
  - elapsed time
  - bottlenecks

## Suggested Tool Contract Groups

### A. Planning / Allocation
- `plan_stage_allocation(project_id, stage, requested_parallelism)`
- `propose_stage_execution(project_id, stage)`

### B. Session Lifecycle
- `create_session(task_id, agent_type, role)`
- `send_command(session_id, text)`
- `read_session_output(session_id, lines)`
- `close_session(session_id)`

### C. Workspace / Git
- `create_worktree(project_id, task_id, branch_name, path?)`
- `commit_changes(session_id | worktree_id, message)`
- `merge_worktree(worktree_id, target_branch?)`
- `resolve_merge_conflict(worktree_id, session_id?, message?)`

### D. Governance / Quality
- `request_stage_approval(project_id, stage, proposal_payload)`
- `run_quality_gate(project_id | task_id)`
- `write_project_memory(...)`
- `read_project_memory(...)`

## Acceptance Criteria
- Orchestrator can execute end-to-end flow with explicit approvals and no ambiguous session targeting.
- Parallel workers never exceed configured capacity.
- Every mutating action is auditable through command/workflow history.
- Stage transitions are evidence-based (tests/review status), not plain-text guessing.
