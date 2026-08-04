#!/usr/bin/env bash
# HY-06: docs.yaml concurrency must not share a bare `pages` group across
# PR and main — a PR docs build must not cancel an in-flight main Pages deploy.
#
# Asserts:
#   (a) concurrency.group is scoped with an event/ref expression (not bare `pages`)
#   (b) cancel-in-progress is conditional on pull_request (not unconditional true)
# Pure bash/grep — no yq (Docs CI does not install yq).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/docs.yaml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

[[ -f "${WORKFLOW}" ]] || fail "workflow file not found: ${WORKFLOW}"

# Extract the concurrency block (from 'concurrency:' through next top-level key).
block="$(awk '
  /^concurrency:/ {inblock=1; next}
  inblock && /^[^[:space:]#]/ {exit}
  inblock {print}
' "${WORKFLOW}")"
[[ -n "${block}" ]] || fail "docs.yaml has no concurrency block"

group_line="$(printf '%s\n' "${block}" | grep -E '^[[:space:]]*group:' | head -1 || true)"
[[ -n "${group_line}" ]] || fail "docs.yaml has no concurrency.group"

# Bare group: pages — reject exact token after group:
if printf '%s\n' "${group_line}" | grep -Eq 'group:[[:space:]]*pages[[:space:]]*$'; then
  fail "concurrency.group must not be bare 'pages' (PR builds cancel main Pages deploys); got: ${group_line}"
fi
if ! printf '%s\n' "${group_line}" | grep -Eq 'github\.(event\.pull_request\.number|ref)'; then
  fail "concurrency.group must scope by PR number or github.ref; got: ${group_line}"
fi
pass "concurrency.group is ref-scoped (not bare pages): ${group_line}"

cancel_line="$(printf '%s\n' "${block}" | grep -E '^[[:space:]]*cancel-in-progress:' | head -1 || true)"
[[ -n "${cancel_line}" ]] || fail "docs.yaml has no concurrency.cancel-in-progress"

if printf '%s\n' "${cancel_line}" | grep -Eq 'cancel-in-progress:[[:space:]]*true[[:space:]]*$'; then
  fail "cancel-in-progress must be conditional (PR only), not unconditional true; got: ${cancel_line}"
fi
if ! printf '%s\n' "${cancel_line}" | grep -Fq 'pull_request'; then
  fail "cancel-in-progress must gate on pull_request event; got: ${cancel_line}"
fi
pass "cancel-in-progress is PR-conditional: ${cancel_line}"

echo "All docs.yaml pages-concurrency tests passed."
