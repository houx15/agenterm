#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${HOME}/.config/agenterm/agenterm.db"
ASSUME_YES=0

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--db PATH] [--yes]

Clear project-related data in the agenterm SQLite DB for testing:
- projects
- tasks
- worktrees
- sessions
- orchestrator_messages
- project_orchestrators
- role_bindings
- project_knowledge_entries
- review_cycles
- review_issues
- demand_pool_items
- role_loop_attempts

Notes:
- This keeps global settings like workflows and agent configs.
- This is destructive and intended for local testing only.

Options:
  --db PATH   Path to SQLite DB (default: ${HOME}/.config/agenterm/agenterm.db)
  --yes       Skip interactive confirmation
  -h, --help  Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --db)
      DB_PATH="${2:-}"
      shift 2
      ;;
    --yes)
      ASSUME_YES=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${DB_PATH}" ]]; then
  echo "Error: db path is empty" >&2
  exit 1
fi

if [[ ! -f "${DB_PATH}" ]]; then
  echo "Error: DB file not found: ${DB_PATH}" >&2
  exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "Error: sqlite3 is required but not found in PATH" >&2
  exit 1
fi

if [[ ${ASSUME_YES} -ne 1 ]]; then
  echo "This will DELETE project-related data from: ${DB_PATH}"
  read -r -p "Type 'yes' to continue: " answer
  if [[ "${answer}" != "yes" ]]; then
    echo "Aborted."
    exit 0
  fi
fi

sqlite3 "${DB_PATH}" <<'SQL'
PRAGMA foreign_keys = ON;
BEGIN;
DELETE FROM review_issues;
DELETE FROM review_cycles;
DELETE FROM role_loop_attempts;
DELETE FROM sessions;
DELETE FROM worktrees;
DELETE FROM tasks;
DELETE FROM orchestrator_messages;
DELETE FROM project_knowledge_entries;
DELETE FROM role_bindings;
DELETE FROM project_orchestrators;
DELETE FROM demand_pool_items;
DELETE FROM projects;
COMMIT;
SQL

echo "Done. Project-related data has been cleared from ${DB_PATH}."
