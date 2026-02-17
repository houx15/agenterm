#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "${repo_root}" ]]; then
  exit 0
fi

if git -C "${repo_root}" diff --name-only --diff-filter=U | grep -q .; then
  exit 0
fi

if [[ -z "$(git -C "${repo_root}" status --porcelain)" ]]; then
  exit 0
fi

git -C "${repo_root}" add -A
if git -C "${repo_root}" diff --cached --quiet; then
  exit 0
fi

git -C "${repo_root}" commit -m "[auto] tool-write checkpoint" >/dev/null 2>&1 || true
