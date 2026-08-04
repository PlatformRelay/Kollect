#!/usr/bin/env bash
# HY-06: docs.yaml concurrency must not share a bare `pages` group across
# PR and main — a PR docs build must not cancel an in-flight main Pages deploy.
#
# Asserts:
#   (a) concurrency.group is scoped with an event/ref expression (not bare `pages`)
#   (b) cancel-in-progress is conditional on pull_request (not unconditional true)
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

if ! command -v yq >/dev/null 2>&1; then
  echo "yq not found; install yq (mikefarah/yq v4) to run this test" >&2
  exit 1
fi

[[ -f "${WORKFLOW}" ]] || fail "workflow file not found: ${WORKFLOW}"

GROUP="$(yq eval '.concurrency.group' "${WORKFLOW}")"
[[ "${GROUP}" != "null" ]] || fail "docs.yaml has no concurrency.group"

if [[ "${GROUP}" == "pages" ]]; then
  fail "concurrency.group must not be bare 'pages' (PR builds cancel main Pages deploys); got: ${GROUP}"
fi
# Must include a per-ref / per-PR discriminator (PR number or github.ref).
if [[ "${GROUP}" != *'github.event.pull_request.number'* && "${GROUP}" != *'github.ref'* ]]; then
  fail "concurrency.group must scope by PR number or github.ref; got: ${GROUP}"
fi
pass "concurrency.group is ref-scoped (not bare pages): ${GROUP}"

CANCEL="$(yq eval '.concurrency.cancel-in-progress' "${WORKFLOW}")"
[[ "${CANCEL}" != "null" ]] || fail "docs.yaml has no concurrency.cancel-in-progress"

# Unconditional true lets PR workflows cancel main deploys in a shared group;
# after scoping, cancel must still be PR-only so main force-pushes stay safe.
if [[ "${CANCEL}" == "true" ]]; then
  fail "cancel-in-progress must be conditional (PR only), not unconditional true; got: ${CANCEL}"
fi
if [[ "${CANCEL}" != *'pull_request'* ]]; then
  fail "cancel-in-progress must gate on pull_request event; got: ${CANCEL}"
fi
pass "cancel-in-progress is PR-conditional: ${CANCEL}"

echo "All docs.yaml pages-concurrency tests passed."
