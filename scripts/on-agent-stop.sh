#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "${repo_root}" ]]; then
  exit 0
fi

mkdir -p "${repo_root}/.orchestra"
: > "${repo_root}/.orchestra/done"

if [[ -n "$(git -C "${repo_root}" status --porcelain)" ]]; then
  git -C "${repo_root}" add -A
  if ! git -C "${repo_root}" diff --cached --quiet; then
    git -C "${repo_root}" commit -m "[READY_FOR_REVIEW] agent completed run" >/dev/null 2>&1 || true
  fi
fi
