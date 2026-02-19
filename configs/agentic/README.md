# Agentic Config Workspace

This folder is intentionally empty of fixed templates.

## Purpose

It is a workspace for AI assistants (or users) to generate setup configs based on project-specific requirements.

## Expected Structure

1. Agent files:
   - `configs/agentic/agents/*.json`
2. Playbook files:
   - `configs/agentic/playbooks/*.json`

## Why No Defaults

Different users have different:

1. available local tools/models
2. orchestration preferences
3. risk/parallelism policies
4. workflow requirements

So assistants should ask questions first, then generate configs.

## Apply Generated Configs

Use:

```bash
bash scripts/agentic-bootstrap.sh \
  --token "<TOKEN>" \
  --repo-path "/absolute/path/to/repo" \
  --project-name "My Project" \
  --playbook "<PLAYBOOK_ID>"
```
