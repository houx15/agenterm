# Agentic Setup Guide

This guide defines how an AI assistant should set up agenterm for a user.

## Intent

Agentic onboarding is requirement-driven, not template-driven:

1. assistant asks focused setup questions
2. assistant explains why each config is chosen
3. assistant applies config via API
4. assistant creates project and provides kickoff steps

## Minimal Required Questions

Before changing anything, assistant should ask:

1. What repo should this project use? (absolute local path)
2. Which agent tools/models are available on this machine?
3. Preferred workflow style?
   - pairing
   - tdd
   - compound parallel
   - custom
4. Parallelism preference and risk tolerance?
5. Should orchestrator run with anthropic or openai profile?

## Why These Questions Matter

1. Repo path defines where worktrees/specs/sessions operate.
2. Available tools/models constrain `allowed_agents` and role mapping.
3. Workflow style determines stage/role shape.
4. Parallelism controls throughput vs coordination complexity.
5. Orchestrator provider/profile controls orchestration runtime behavior.

## Setup Flow (Assistant)

1. Read `AGENTS.md`.
2. Verify API access (`GET /api/projects`).
3. Propose agent registry config with rationale.
4. Propose playbook config with rationale.
5. Ask for user approval.
6. Apply:
   - `POST/PUT /api/agents`
   - `POST/PUT /api/playbooks`
   - `POST /api/projects`
7. Return:
   - created/updated IDs
   - suggested first PM Chat message

## Optional Helper Script

You can use:

```bash
bash scripts/agentic-bootstrap.sh \
  --token "<TOKEN>" \
  --repo-path "/absolute/path/to/repo" \
  --project-name "My Project" \
  --playbook "compound-engineering"
```

Notes:

1. Script reads JSON files in:
   - `configs/agentic/agents/`
   - `configs/agentic/playbooks/`
2. These files are intentionally user-defined.
3. Assistant should generate files based on user requirements when needed.

## Config Principles

1. Keep playbook stages fixed: `plan`, `build`, `test`.
2. Encode role contracts explicitly:
   - `mode`
   - `inputs_required`
   - `actions_allowed`
   - `handoff_to`
   - `retry_policy`
   - `gates`
3. Use smallest viable parallelism first, then increase.
4. Keep `actions_allowed` least-privilege per role.

## Kickoff Output (Assistant should provide)

After setup, assistant should provide:

1. Project ID and selected playbook ID.
2. Why this playbook/agent mapping was chosen.
3. A copy-paste PM Chat kickoff prompt for plan stage.
