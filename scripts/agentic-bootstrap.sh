#!/usr/bin/env bash
set -euo pipefail

API_BASE="http://localhost:8765"
TOKEN=""
REPO_PATH=""
PROJECT_NAME=""
PLAYBOOK_ID="compound-engineering"
AGENTS_DIR="configs/agentic/agents"
PLAYBOOKS_DIR="configs/agentic/playbooks"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/agentic-bootstrap.sh \
    --token <TOKEN> \
    --repo-path <ABS_PATH> \
    --project-name <NAME> \
    [--playbook <PLAYBOOK_ID>] \
    [--api-base <URL>] \
    [--agents-dir <DIR>] \
    [--playbooks-dir <DIR>]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --token) TOKEN="${2:-}"; shift 2 ;;
    --repo-path) REPO_PATH="${2:-}"; shift 2 ;;
    --project-name) PROJECT_NAME="${2:-}"; shift 2 ;;
    --playbook) PLAYBOOK_ID="${2:-}"; shift 2 ;;
    --api-base) API_BASE="${2:-}"; shift 2 ;;
    --agents-dir) AGENTS_DIR="${2:-}"; shift 2 ;;
    --playbooks-dir) PLAYBOOKS_DIR="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "$TOKEN" || -z "$REPO_PATH" || -z "$PROJECT_NAME" ]]; then
  echo "Error: --token, --repo-path, and --project-name are required." >&2
  usage
  exit 1
fi

if [[ ! -d "$REPO_PATH" ]]; then
  echo "Error: repo path does not exist: $REPO_PATH" >&2
  exit 1
fi

auth_header=("Authorization: Bearer ${TOKEN}")
json_header=("Content-Type: application/json")

api_get_status() {
  local path="$1"
  curl -sS -o /dev/null -w "%{http_code}" -H "${auth_header[0]}" "${API_BASE}${path}"
}

api_upsert_json_file() {
  local kind="$1"   # agent|playbook
  local id="$2"
  local file="$3"

  local get_path="/api/${kind}s/${id}"
  local base_path="/api/${kind}s"
  local code
  code="$(api_get_status "$get_path")"

  if [[ "$code" == "200" ]]; then
    curl -sS -X PUT \
      -H "${auth_header[0]}" -H "${json_header[0]}" \
      --data-binary @"$file" \
      "${API_BASE}${get_path}" >/dev/null
    echo "Updated ${kind}: ${id}"
  else
    curl -sS -X POST \
      -H "${auth_header[0]}" -H "${json_header[0]}" \
      --data-binary @"$file" \
      "${API_BASE}${base_path}" >/dev/null
    echo "Created ${kind}: ${id}"
  fi
}

extract_json_id() {
  local file="$1"
  sed -n 's/^[[:space:]]*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$file" | head -n 1
}

echo "Checking server at ${API_BASE} ..."
health_code="$(api_get_status "/api/projects")"
if [[ "$health_code" != "200" ]]; then
  echo "Error: API is not reachable or token is invalid (GET /api/projects -> ${health_code})." >&2
  exit 1
fi

if [[ -d "$AGENTS_DIR" ]]; then
  while IFS= read -r -d '' file; do
    id="$(extract_json_id "$file")"
    if [[ -z "$id" ]]; then
      echo "Skip invalid agent JSON (missing id): $file" >&2
      continue
    fi
    api_upsert_json_file "agent" "$id" "$file"
  done < <(find "$AGENTS_DIR" -type f -name '*.json' -print0 | sort -z)
fi

if [[ -d "$PLAYBOOKS_DIR" ]]; then
  while IFS= read -r -d '' file; do
    id="$(extract_json_id "$file")"
    if [[ -z "$id" ]]; then
      echo "Skip invalid playbook JSON (missing id): $file" >&2
      continue
    fi
    api_upsert_json_file "playbook" "$id" "$file"
  done < <(find "$PLAYBOOKS_DIR" -type f -name '*.json' -print0 | sort -z)
fi

echo "Creating project ..."
create_payload="$(cat <<EOF
{
  "name": "${PROJECT_NAME}",
  "repo_path": "${REPO_PATH}",
  "playbook": "${PLAYBOOK_ID}",
  "status": "active"
}
EOF
)"

project_resp="$(curl -sS -X POST \
  -H "${auth_header[0]}" -H "${json_header[0]}" \
  -d "$create_payload" \
  "${API_BASE}/api/projects")"

project_id="$(printf '%s' "$project_resp" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -n 1)"
if [[ -z "$project_id" ]]; then
  echo "Project creation response: $project_resp"
  echo "Warning: could not parse project id from response."
else
  echo "Created project id: $project_id"
  echo "Next: open PM Chat and select project '$PROJECT_NAME'."
fi
