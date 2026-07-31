#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT_DIR
readonly WORKFLOW="${ROOT_DIR}/.github/workflows/ci.yaml"
readonly PULL_SCRIPT="${ROOT_DIR}/hack/ci/pull-image-with-retry.sh"
readonly FORGEJO_IMAGE="codeberg.org/forgejo/forgejo:11"

fail() {
  echo "forgejo_prepull_test: $*" >&2
  exit 1
}

test_workflow_wiring() {
  [[ -x "${PULL_SCRIPT}" ]] || fail "retry helper is missing or not executable"

  local regression_line prepull_line integration_line
  regression_line="$(grep -nF "run: bash test/ci/forgejo_prepull_test.sh" "${WORKFLOW}" | cut -d: -f1 || true)"
  prepull_line="$(grep -nF "bash hack/ci/pull-image-with-retry.sh \"${FORGEJO_IMAGE}\"" "${WORKFLOW}" | cut -d: -f1 || true)"
  integration_line="$(grep -nF "run: task test-integration" "${WORKFLOW}" | cut -d: -f1 || true)"

  [[ -n "${regression_line}" ]] || fail "required integration job does not run this regression test"
  [[ -n "${prepull_line}" ]] || fail "workflow does not pre-pull the pinned Forgejo image"
  [[ -n "${integration_line}" ]] || fail "integration test step is missing"
  ((regression_line < prepull_line)) || fail "regression test must run before the Forgejo pre-pull"
  ((prepull_line < integration_line)) || fail "Forgejo pre-pull must run before integration tests"
}

test_retry_then_success() {
  local temp_dir calls_file
  temp_dir="$(mktemp -d)"
  calls_file="${temp_dir}/calls"
  trap 'rm -rf "${temp_dir}"' RETURN

  cat >"${temp_dir}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
: "${FAKE_DOCKER_CALLS:?}"
calls=0
if [[ -f "${FAKE_DOCKER_CALLS}" ]]; then
  calls="$(cat "${FAKE_DOCKER_CALLS}")"
fi
calls=$((calls + 1))
printf '%s\n' "${calls}" >"${FAKE_DOCKER_CALLS}"
[[ "${1:-}" == "pull" ]] || exit 64
[[ "${2:-}" == "codeberg.org/forgejo/forgejo:11" ]] || exit 65
((calls >= 3))
EOF
  chmod +x "${temp_dir}/docker"

  PATH="${temp_dir}:${PATH}" FAKE_DOCKER_CALLS="${calls_file}" \
    PULL_RETRY_ATTEMPTS=3 PULL_RETRY_DELAY_SECONDS=0 "${PULL_SCRIPT}" "${FORGEJO_IMAGE}"

  [[ "$(cat "${calls_file}")" == "3" ]] || fail "helper did not stop immediately after success"
}

test_bounded_failure() {
  local temp_dir calls_file
  temp_dir="$(mktemp -d)"
  calls_file="${temp_dir}/calls"
  trap 'rm -rf "${temp_dir}"' RETURN

  cat >"${temp_dir}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
: "${FAKE_DOCKER_CALLS:?}"
calls=0
if [[ -f "${FAKE_DOCKER_CALLS}" ]]; then
  calls="$(cat "${FAKE_DOCKER_CALLS}")"
fi
printf '%s\n' "$((calls + 1))" >"${FAKE_DOCKER_CALLS}"
exit 1
EOF
  chmod +x "${temp_dir}/docker"

  if PATH="${temp_dir}:${PATH}" FAKE_DOCKER_CALLS="${calls_file}" \
    PULL_RETRY_ATTEMPTS=2 PULL_RETRY_DELAY_SECONDS=0 "${PULL_SCRIPT}" "${FORGEJO_IMAGE}"; then
    fail "helper unexpectedly succeeded when every pull failed"
  fi

  [[ "$(cat "${calls_file}")" == "2" ]] || fail "helper exceeded its configured attempt bound"
}

test_workflow_wiring
test_retry_then_success
test_bounded_failure
echo "forgejo_prepull_test: PASS"
