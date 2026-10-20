#!/usr/bin/env bash
# OFS-1..OFS-5 (openspec change operatorhub-fork-sync): hack/operatorhub-pr.sh brings each
# community-operators fork's default branch up to date with upstream before it branches or pushes,
# fails closed on a fork with commits of its own, and makes no remote write under DRY_RUN=1.
#
# The script runs for real against stubs of gh, git, curl and make placed first on PATH. Every
# call is logged; a remote write (merge-upstream POST, any other non-GET gh api call, gh pr create,
# git push) is logged with a WRITE prefix, so "no writes" is a count, not a grep for a message.
# The gh stub answers with responses shaped like the real API's (fixtures below were taken from
# `gh api repos/PlatformRelay/community-operators --jq '{default_branch,parent:{full_name:.parent.full_name}}'`
# and `gh api repos/k8s-operatorhub/community-operators/compare/main...PlatformRelay:community-operators:main
# --jq '{status,ahead_by,behind_by}'` on 2026-10-20) and applies the script's own --jq expression
# to them with jq, so a wrong jq path in the script fails here.
#
# Usage:
#   hack/test/operatorhub_fork_sync_test.sh              # run every case against the real script
#   hack/test/operatorhub_fork_sync_test.sh --self-test  # mutate copies, assert each mutant reds
#
# OFS_SCRIPT overrides the script under test (used to record the red against an older script).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
REAL_SCRIPT="${OFS_SCRIPT:-${ROOT}/hack/operatorhub-pr.sh}"

fail() {
  printf 'operatorhub fork sync: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

command -v jq >/dev/null 2>&1 || fail "jq is required"
[[ -f "${REAL_SCRIPT}" ]] || fail "${REAL_SCRIPT} is missing"

WORK="$(mktemp -d)"
# BUNDLE_DIR must be relative to the checkout (the script prefixes it with its working directory),
# so the fixture bundle lives under the gitignored dist/.
BUNDLE_REL="dist/test-operatorhub-fork-sync.$$"
trap 'rm -rf "${WORK:?}" "${ROOT:?}/${BUNDLE_REL:?}"' EXIT

make_bundle() {
  local b="${ROOT}/${BUNDLE_REL}" crd
  mkdir -p "${b}/manifests" "${b}/metadata"
  printf 'kind: ClusterServiceVersion\n' >"${b}/manifests/kollect.clusterserviceversion.yaml"
  printf 'annotations:\n  operators.operatorframework.io.bundle.package.v1: kollect\n' >"${b}/metadata/annotations.yaml"
  while IFS= read -r crd; do
    cp "${crd}" "${b}/manifests/"
  done < <(find config/crd/bases -name 'kollect.dev_*.yaml' | sort)
}

make_stubs() {
  local bin="$1"
  mkdir -p "${bin}"

  cat >"${bin}/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'gh %s\n' "$*" >>"${STUB_LOG}"
upstream_for() {
  case "$1" in
  community-operators) echo "k8s-operatorhub/community-operators" ;;
  community-operators-prod) echo "redhat-openshift-ecosystem/community-operators-prod" ;;
  *) echo "unknown/$1" ;;
  esac
}
state_file() { printf '%s/state-%s' "${STUB_DIR}" "$1"; }
emit() {
  if [[ -n "${jq_expr}" ]]; then printf '%s' "$1" | jq -r "${jq_expr}"; else printf '%s\n' "$1"; fi
}
if [[ "${1:-}" == pr ]]; then
  case "${2:-}" in
  list) exit 0 ;;
  create)
    printf 'WRITE gh %s\n' "$*" >>"${STUB_LOG}"
    echo "https://github.com/stub/pull/1"
    exit 0
    ;;
  esac
  exit 0
fi
[[ "${1:-}" == api ]] || exit 0
shift
method=GET endpoint="" jq_expr="" fields=()
while (($#)); do
  case "$1" in
  --method | -X) method="$2"; shift 2 ;;
  --jq | -q) jq_expr="$2"; shift 2 ;;
  -f | -F | --field | --raw-field) fields+=("$2"); shift 2 ;;
  -H) shift 2 ;;
  -*) shift ;;
  *) [[ -z "${endpoint}" ]] && endpoint="$1"; shift ;;
  esac
done
if [[ "${method}" != GET ]]; then
  printf 'WRITE gh api %s %s %s\n' "${method}" "${endpoint}" "${fields[*]:-}" >>"${STUB_LOG}"
fi
case "${endpoint}" in
repos/*/compare/*)
  repo="${endpoint#*...}"
  repo="${repo#*:}"
  repo="${repo%%:*}"
  read -r status ahead behind <"$(state_file "${repo}")"
  emit "{\"status\":\"${status}\",\"ahead_by\":${ahead},\"behind_by\":${behind},\"total_commits\":${ahead}}"
  ;;
repos/*/merge-upstream)
  repo="${endpoint#repos/*/}"
  repo="${repo%/merge-upstream}"
  read -r status ahead behind <"$(state_file "${repo}")"
  if [[ "${status}" == behind && "${STUB_STUCK:-0}" != 1 ]]; then
    echo "identical 0 0" >"$(state_file "${repo}")"
    emit '{"message":"Successfully fetched and fast-forwarded from upstream","merge_type":"fast-forward","base_branch":"upstream:main"}'
  else
    emit '{"message":"This branch is not behind the upstream","merge_type":"none","base_branch":"upstream:main"}'
  fi
  ;;
repos/*/*)
  repo="${endpoint##*/}"
  parent="${STUB_PARENT:-$(upstream_for "${repo}")}"
  emit "{\"full_name\":\"PlatformRelay/${repo}\",\"default_branch\":\"main\",\"fork\":true,\"parent\":{\"full_name\":\"${parent}\",\"default_branch\":\"main\"}}"
  ;;
*) exit 0 ;;
esac
STUB

  cat >"${bin}/git" <<'STUB'
#!/usr/bin/env bash
printf 'git %s\n' "$*" >>"${STUB_LOG}"
case "${1:-}" in
clone) mkdir -p "${!#}" ;;
push) printf 'WRITE git %s\n' "$*" >>"${STUB_LOG}" ;;
esac
exit 0
STUB

  cat >"${bin}/curl" <<'STUB'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >>"${STUB_LOG}"
case "${!#}" in
*/token*) echo '{"token":"stub"}' ;;
*) echo '{"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":"sha256:00"}]}' ;;
esac
STUB

  cat >"${bin}/make" <<'STUB'
#!/usr/bin/env bash
printf 'make %s\n' "$*" >>"${STUB_LOG}"
STUB

  chmod +x "${bin}/gh" "${bin}/git" "${bin}/curl" "${bin}/make"
}

# run_script <script> <case-dir> <state-of-community-operators> <state-of-community-operators-prod> [VAR=val ...]
# Leaves the exit code in ${case_dir}/rc, the combined output in ${case_dir}/out and the call log
# in ${case_dir}/log.
run_script() {
  local script="$1" case_dir="$2" s1="$3" s2="$4" rc=0
  shift 4
  mkdir -p "${case_dir}"
  : >"${case_dir}/log"
  echo "${s1}" >"${case_dir}/state-community-operators"
  echo "${s2}" >"${case_dir}/state-community-operators-prod"
  env -u GH_TOKEN -u DRY_RUN -u BUNDLE_DIR -u STUB_STUCK -u STUB_PARENT \
    PATH="${WORK}/bin:${PATH}" STUB_LOG="${case_dir}/log" STUB_DIR="${case_dir}" \
    VERSION=9.9.9-test FORK_OWNER=platformrelay BUNDLE_DIR="${BUNDLE_REL}" \
    IMAGE_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
    "$@" bash "${script}" >"${case_dir}/out" 2>&1 || rc=$?
  echo "${rc}" >"${case_dir}/rc"
}

writes() { grep -c '^WRITE ' "$1/log" || true; }
first_line() { grep -n -m1 -- "$2" "$1/log" | cut -d: -f1; }

# Each case prints nothing and returns 0 on success, or prints its own failure line and returns 1.
case_behind_synced() {
  local s="$1" d="${WORK}/c-behind" sync push
  run_script "${s}" "${d}" "behind 0 15" "behind 0 18" GH_TOKEN=stub
  [[ "$(cat "${d}/rc")" == 0 ]] || { echo "behind->synced: exit $(cat "${d}/rc"): $(tail -3 "${d}/out")"; return 1; }
  for fork in community-operators community-operators-prod; do
    grep -Fqx "WRITE gh api POST repos/platformrelay/${fork}/merge-upstream branch=main" "${d}/log" ||
      { echo "behind->synced: no merge-upstream call for platformrelay/${fork}"; return 1; }
    [[ "$(grep -c "^gh api repos/.*/compare/main\.\.\.platformrelay:${fork}:main" "${d}/log")" -ge 2 ]] ||
      { echo "behind->synced: platformrelay/${fork} was not re-compared after the sync"; return 1; }
  done
  sync="$(first_line "${d}" '^WRITE gh api POST .*/merge-upstream')"
  push="$(first_line "${d}" '^WRITE git push')"
  [[ -n "${push}" && "${sync}" -lt "$(first_line "${d}" '^git clone')" ]] ||
    { echo "behind->synced: the forks were not synced before the first clone/push (sync line ${sync:-none}, push line ${push:-none})"; return 1; }
  if [[ "$(grep -c '^WRITE git push' "${d}/log")" != 2 ]] || grep '^WRITE git push' "${d}/log" | grep -v ' kollect-v9.9.9-test$' >/dev/null; then
    echo "behind->synced: a push targets something other than the kollect-v9.9.9-test branch"
    return 1
  fi
}

case_identical_no_sync() {
  local s="$1" d="${WORK}/c-identical"
  run_script "${s}" "${d}" "identical 0 0" "identical 0 0" GH_TOKEN=stub
  [[ "$(cat "${d}/rc")" == 0 ]] || { echo "identical: exit $(cat "${d}/rc"): $(tail -3 "${d}/out")"; return 1; }
  ! grep -F 'merge-upstream' "${d}/log" >/dev/null || { echo "identical: synced a fork that was already identical"; return 1; }
  [[ "$(grep -c '^WRITE git push' "${d}/log")" == 2 ]] || { echo "identical: expected two submission pushes"; return 1; }
}

assert_failed_closed() {
  local label="$1" d="$2" message="$3"
  [[ "$(writes "${d}")" == 0 ]] || { echo "${label}: made $(writes "${d}") remote write(s): $(grep '^WRITE' "${d}/log" | head -3)"; return 1; }
  [[ "$(cat "${d}/rc")" != 0 ]] || { echo "${label}: exited 0"; return 1; }
  grep -Fq -- "${message}" "${d}/out" || { echo "${label}: no '${message}' message: $(tail -3 "${d}/out")"; return 1; }
  ! grep '^git ' "${d}/log" >/dev/null || { echo "${label}: ran git before failing"; return 1; }
}

case_diverged_fails() {
  local s="$1" d="${WORK}/c-diverged"
  run_script "${s}" "${d}" "behind 0 3" "diverged 2 5" GH_TOKEN=stub
  assert_failed_closed "diverged->fail" "${d}" "community-operators-prod:main is diverged relative to"
}

case_ahead_fails() {
  local s="$1" d="${WORK}/c-ahead"
  run_script "${s}" "${d}" "ahead 1 0" "identical 0 0" GH_TOKEN=stub
  assert_failed_closed "ahead->fail" "${d}" "platformrelay/community-operators:main is ahead relative to"
}

case_wrong_parent_fails() {
  local s="$1" d="${WORK}/c-parent"
  run_script "${s}" "${d}" "behind 0 1" "behind 0 1" GH_TOKEN=stub STUB_PARENT=someone/else
  assert_failed_closed "wrong parent" "${d}" "is not a fork of k8s-operatorhub/community-operators"
}

case_stuck_sync_fails() {
  local s="$1" d="${WORK}/c-stuck"
  run_script "${s}" "${d}" "behind 0 4" "identical 0 0" GH_TOKEN=stub STUB_STUCK=1
  [[ "$(cat "${d}/rc")" != 0 ]] || { echo "stuck sync: exited 0"; return 1; }
  grep -Fq 'is still behind' "${d}/out" || { echo "stuck sync: no 'is still behind' message: $(tail -3 "${d}/out")"; return 1; }
  ! grep '^WRITE git push' "${d}/log" >/dev/null || { echo "stuck sync: pushed although the fork is still behind"; return 1; }
}

case_dry_run_plan() {
  local s="$1" d="${WORK}/c-dry"
  run_script "${s}" "${d}" "behind 0 15" "identical 0 0" GH_TOKEN=stub DRY_RUN=1
  [[ "$(cat "${d}/rc")" == 0 ]] || { echo "DRY_RUN: exit $(cat "${d}/rc"): $(tail -3 "${d}/out")"; return 1; }
  [[ "$(writes "${d}")" == 0 ]] || { echo "DRY_RUN: made $(writes "${d}") remote write(s): $(grep '^WRITE' "${d}/log" | head -3)"; return 1; }
  ! grep -E '^(git|curl) ' "${d}/log" >/dev/null || { echo "DRY_RUN: ran git or curl"; return 1; }
  grep -Fq 'would sync platformrelay/community-operators:main from k8s-operatorhub/community-operators:main (behind by 15)' "${d}/out" ||
    { echo "DRY_RUN: does not print the sync it would make"; return 1; }
  grep -Fq 'platformrelay/community-operators-prod:main is identical to redhat-openshift-ecosystem/community-operators-prod:main; would not sync' "${d}/out" ||
    { echo "DRY_RUN: does not report the up-to-date fork"; return 1; }
  [[ "$(grep -c 'would create branch kollect-v9.9.9-test' "${d}/out")" == 2 && "$(grep -c 'git push --force origin kollect-v9.9.9-test' "${d}/out")" == 2 ]] ||
    { echo "DRY_RUN: does not print the branch and push for both forks"; return 1; }
}

case_dry_run_diverged() {
  local s="$1" d="${WORK}/c-dry-diverged"
  run_script "${s}" "${d}" "diverged 1 1" "behind 0 2" GH_TOKEN=stub DRY_RUN=1
  [[ "$(cat "${d}/rc")" != 0 ]] || { echo "DRY_RUN diverged: exited 0"; return 1; }
  grep -Fq 'is diverged relative to' "${d}/out" || { echo "DRY_RUN diverged: no divergence message"; return 1; }
  [[ "$(writes "${d}")" == 0 ]] || { echo "DRY_RUN diverged: made a remote write"; return 1; }
}

case_dry_run_offline() {
  local s="$1" d="${WORK}/c-dry-offline"
  run_script "${s}" "${d}" "diverged 1 1" "diverged 1 1" DRY_RUN=1
  ! grep -E '^(gh|git|curl) ' "${d}/log" >/dev/null || { echo "DRY_RUN offline: called gh, git or curl without GH_TOKEN"; return 1; }
  [[ "$(cat "${d}/rc")" == 0 ]] || { echo "DRY_RUN offline: exit $(cat "${d}/rc"): $(tail -3 "${d}/out")"; return 1; }
  grep -Fq 'fork state not checked' "${d}/out" || { echo "DRY_RUN offline: does not say the fork state was not checked"; return 1; }
}

CASES=(case_behind_synced case_identical_no_sync case_diverged_fails case_ahead_fails
  case_wrong_parent_fails case_stuck_sync_fails case_dry_run_plan case_dry_run_diverged case_dry_run_offline)

# check_cases <script>: runs every case; prints one line per failed case; returns non-zero if any failed.
check_cases() {
  local script="$1" c out failed=0
  for c in "${CASES[@]}"; do
    if ! out="$("${c}" "${script}")"; then
      printf '%s\n' "${out}"
      failed=1
    fi
  done
  return "${failed}"
}

make_bundle
make_stubs "${WORK}/bin"

if [[ "${1:-}" != --self-test ]]; then
  if ! check_cases "${REAL_SCRIPT}" >"${WORK}/failures"; then
    cat "${WORK}/failures" >&2
    fail "hack/operatorhub-pr.sh does not sync forks safely (failures above)"
  fi
  for c in "${CASES[@]}"; do pass "${c#case_}"; done
  echo "All operatorhub fork sync tests passed."
  exit 0
fi

# --self-test: one mutant per clause, each a copy of the script with hack/lib beside it. A mutant
# must turn its own case red with its own message; the no-op control must stay green.
mutant() {
  local label="$1" case_fn="$2" expect="$3" sed_expr="$4" dir out
  dir="${WORK}/m-${label}"
  mkdir -p "${dir}/hack"
  cp -R "${ROOT}/hack/lib" "${dir}/hack/lib"
  sed -e "${sed_expr}" "${REAL_SCRIPT}" >"${dir}/hack/operatorhub-pr.sh"
  if cmp -s "${dir}/hack/operatorhub-pr.sh" "${REAL_SCRIPT}"; then
    fail "self-test: mutant '${label}' is identical to the script -- the sed anchor no longer matches"
  fi
  if [[ "${case_fn}" == all ]]; then
    check_cases "${dir}/hack/operatorhub-pr.sh" >"${WORK}/noop" ||
      fail "self-test: the no-op control reds: $(cat "${WORK}/noop")"
    pass "self-test: no-op control stays green"
  else
    if out="$("${case_fn}" "${dir}/hack/operatorhub-pr.sh")"; then
      fail "self-test: mutant '${label}' survived ${case_fn}"
    fi
    [[ "${out}" == *"${expect}"* ]] ||
      fail "self-test: mutant '${label}' red ${case_fn} for another reason: ${out}"
    pass "self-test: mutant '${label}' reds ${case_fn}"
  fi
  rm -rf "${dir:?}"
}

# The sed expressions are single-quoted on purpose: they match the script's literal ${...} text.
# shellcheck disable=SC2016
{
mutant no-sync case_behind_synced 'no merge-upstream call' '/^sync_forks || exit 1$/d'
mutant diverged-accepted case_diverged_fails 'diverged->fail: made' 's/^    identical | behind) ;;$/    identical | behind | diverged) ;;/'
mutant ahead-accepted case_ahead_fails 'ahead->fail: made' 's/^    identical | behind) ;;$/    identical | behind | ahead) ;;/'
mutant parent-unchecked case_wrong_parent_fails 'wrong parent: made' 's/^  if \[\[ "\$(lowercase "\${parent}")" != .*; then$/  if false; then/'
mutant no-recompare case_stuck_sync_fails 'stuck sync: exited 0' 's/^    if \[\[ "\${status}" != "identical" \]\]; then$/    if false; then/'
mutant dry-run-syncs case_dry_run_plan 'DRY_RUN: made' 's/^    preflight_forks || exit 1$/    preflight_forks \&\& sync_forks || exit 1/'
mutant dry-run-offline-calls case_dry_run_offline 'DRY_RUN offline: called gh' 's/^  if \[\[ -n "\${GH_TOKEN:-}" \]\]; then$/  if true; then/'
mutant sync-after-clone case_behind_synced 'were not synced before the first clone' '/^sync_forks || exit 1$/d; s/^submit_bundle "k8s-operatorhub\/community-operators"$/submit_bundle "k8s-operatorhub\/community-operators"; sync_forks/'
mutant noop all '' '$a\
# no-op control'
}

echo "All operatorhub fork sync self-tests passed."
