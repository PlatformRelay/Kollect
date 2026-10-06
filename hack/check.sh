#!/usr/bin/env bash
# task check -- the full local gate (openspec change task-check-entrypoint, TCE-1..TCE-3).
#
# Runs every required CI gate that can run on a developer machine, runs every hack/test guard
# in every mode its code declares (bare, plus --self-test for the scripts that parse it), and
# prints the required gates it does NOT run, each with its reason. A failed gate is reported
# and the final exit is non-zero; check never reports success because the last gate passed.
#
# The Docker/kind gates stay separate, named tasks (test-integration, kind-smoke via the
# e2e-smoke workflow, docker:build, pipeline-cli-smoke) -- they are the exclusion table below,
# not silent omissions. Locked by hack/test/task_check_test.sh (plain and --self-test), which
# runs in the required lint job.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}" || exit 1

failures=()

run_gate() {
  local name="$1"
  shift
  printf '\n--- task check: %s ---\n' "${name}"
  if "$@"; then
    printf 'check: %-22s ok\n' "${name}"
  else
    failures+=("${name}")
    printf 'check: %-22s FAILED\n' "${name}" >&2
  fi
}

# The required CI gates (hack/release/verify-eligibility.sh required_checks) that this script
# runs locally, one command per required context. Keep this list aligned with
# verify-eligibility.sh: hack/test/task_check_test.sh fails when a required gate is neither
# here nor in the exclusion table below, or when an exclusion loses its reason.
run_gate verify task verify
run_gate format:check task format:check
run_gate lint:shell task lint:shell
run_gate lint:markdown task lint:markdown
run_gate lint task lint
run_gate vulncheck task vulncheck
run_gate test task test
run_gate scrub task scrub
run_gate spec:validate task spec:validate
run_gate audit-rbac task audit:rbac
run_gate build task build
run_gate helm task helm-test
run_gate gitleaks bash hack/install-gitleaks.sh ./bin
# The same invocation shape as CI's gitleaks job (the checksum-pinned installer first).
run_gate gitleaks-detect ./bin/gitleaks detect --source . --config .github/gitleaks.toml --redact --no-git
run_gate workflow-security bash hack/install-zizmor.sh ./bin
# The pinned offline audit, same invocation as ci.yaml's workflow-security job.
run_gate workflow-security-audit ./bin/zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/

# TCE-3 mapping: required contexts whose local equivalent is a combination of the gates above.
# hack/test/task_check_test.sh reads these lines; the guard sweep is the entry for guards.
#   coverage: preflight=lint:markdown,verify,guard-sweep

# The guard sweep: every hack/test guard, bare, plus --self-test for the scripts that parse
# the flag (a new guard is picked up by the glob with no edit to this script or the Taskfile).
printf '\n--- task check: hack/test guards (every mode) ---\n'
for guard in hack/test/*_test.sh hack/test/lab_harness_meta_suite.sh; do
  printf 'check: guard %s\n' "${guard}"
  if ! bash "${guard}"; then
    failures+=("guard ${guard}")
    printf 'check: guard %s FAILED\n' "${guard}" >&2
  fi
  if grep -v '^[[:space:]]*#' "${guard}" 2>/dev/null | grep -q -- '--self-test'; then
    printf 'check: guard %s (--self-test)\n' "${guard}"
    if bash "${guard}" --self-test; then
      printf 'check: %-22s ok\n' "guard ${guard} --self-test"
    else
      failures+=("guard ${guard} --self-test")
      printf 'check: guard %s --self-test FAILED\n' "${guard}" >&2
    fi
  fi
done

# The required gates check does NOT run, each with the reason it cannot run here. These stay
# separate, named tasks -- printed, never silently skipped, and
# hack/test/task_check_test.sh fails when one loses its reason.
exclusion() {
  printf 'check: excluded %-22s %s\n' "$1" "$2"
}
printf '\n--- task check: required gates not run locally ---\n'
exclusion test-integration "needs a Docker daemon (testcontainers); run task test-integration separately"
exclusion kind-smoke "needs a kind cluster; runs in the required kind-smoke context of the e2e-smoke workflow"
exclusion docker-build "needs a Docker daemon; run task docker:build"
exclusion pipeline-cli-smoke "needs a kind cluster and the CLI image; runs in the e2e-smoke workflow"
exclusion dependency-review "needs the GitHub pull-request context (dependency graph, PR base SHA); runs on pull_request only"

printf '\n--- task check: summary ---\n'
if [[ "${#failures[@]}" -gt 0 ]]; then
  printf 'task check: FAILED -- %d gate(s) failed: %s\n' "${#failures[@]}" "${failures[*]}"
  exit 1
fi
printf 'task check: all runnable gates passed\n'
exit 0
