#!/usr/bin/env bash
# DEMO-02: canonical demo-up / demo-down must exist and delegate to the hero harness.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TASKFILE="${ROOT}/Taskfile.yml"

fail() {
  printf 'demo task aliases: %s\n' "$*" >&2
  exit 1
}

[[ -f "${TASKFILE}" ]] || fail "Taskfile.yml not found"

grep -Eq '^[[:space:]]*demo-up:' "${TASKFILE}" ||
  fail "Taskfile.yml missing demo-up task"
grep -Eq '^[[:space:]]*demo-down:' "${TASKFILE}" ||
  fail "Taskfile.yml missing demo-down task"

# Extract a task body (from its key until the next sibling task key at the same
# indent) and assert it references the hero target via `task: <name>`.
assert_task_delegates() {
  local task_key="$1"
  local hero_task="$2"
  local body
  body="$(
    awk -v key="${task_key}" '
      $0 ~ "^  " key ":" { grab=1; next }
      grab && /^  [a-zA-Z0-9_-]+:/ { exit }
      grab { print }
    ' "${TASKFILE}"
  )"
  [[ -n "${body}" ]] || fail "${task_key} task body empty"
  printf '%s\n' "${body}" | grep -Eq "task:[[:space:]]*${hero_task}([[:space:]]|$)" ||
    fail "${task_key} must delegate to ${hero_task} (task: ${hero_task})"
}

assert_task_delegates demo-up demo-hero-up
assert_task_delegates demo-down demo-hero-down

printf 'demo task aliases: ok\n'
