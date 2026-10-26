#!/usr/bin/env bash
# renovate_policy_test.sh -- the self-hosted Renovate policy guard (openspec change
# dependency-update-automation, DUA-1 to DUA-5, DUA-7 to DUA-10).
#
# Workflow clauses (task 1.1):
#   T1  token        the Renovate step's token is exactly secrets.RENOVATE_TOKEN, and no parsed
#                    value of renovate.yaml names the repository token (DUA-1)
#   T2  fail closed  a step before the Renovate step gets RENOVATE_TOKEN from the secret, has no
#                    if:/continue-on-error:, and its run body (run here under the runner's bash:
#                    `bash -e`, or `-eo pipefail` for an explicit shell: bash) exits non-zero
#                    naming RENOVATE_TOKEN when it is empty and zero when it is set (DUA-1)
#   T3  permissions  every job's effective permissions are a mapping granting contents: read
#                    only; no permissions value is a string such as write-all (DUA-1)
#   T4  schedule     triggers are exactly schedule and workflow_dispatch, and one cron entry
#                    fires every day (DUA-2)
#   T5  concurrency  concurrency.group is the literal renovate, cancel-in-progress is boolean
#                    false (DUA-2)
#   T6  pin          renovatebot/github-action@<40 hex> # vX.Y.Z (>= v46.3.10), a digest-pinned
#                    renovate-image, and no with: key besides configurationFile, token and
#                    renovate-image (DUA-3)
#   T7  environment  the only RENOVATE_* keys are RENOVATE_REPOSITORIES (exactly
#                    github.repository) and the token check step's RENOVATE_TOKEN, and nothing
#                    names GITHUB_ENV (DUA-3, DUA-7, DUA-8)
#   T8  validator    the lint job of ci.yaml runs exactly `bash hack/ci/renovate-config-validate.sh`
#                    with no if:/continue-on-error: (DUA-5)
#   T9  no merging   no run body or github-script of a workflow or composite action merges pull
#                    requests, and no workflow triggers on pull_request_target (DUA-8)
#
# Config, validator and docs clauses (task 1.2), over renovate.json and .github/renovate-config.json:
#   T10 presets      top-level extends is exactly ["config:recommended"], no other object in either
#                    file has extends, and dependencyDashboard is never anything but true (DUA-4, DUA-7)
#   T11 automerge    exactly one object in both files has automerge: true: a packageRules entry of
#                    renovate.json with keys automerge, matchManagers [devcontainer, dockerfile],
#                    matchUpdateTypes [digest], rebaseWhen behind-base-branch, optional description (DUA-7)
#   T12 merge        top level platformAutomerge false, automergeType pr, automergeStrategy rebase,
#                    rebaseWhen conflicted; no other object sets those (except the automerge rule's
#                    rebaseWhen) and nothing has ignoreTests (DUA-8)
#   T13 global       the global config's keys are exactly $schema, onboarding false and
#                    requireConfig required (DUA-7, DUA-8)
#   T14 image rule   a rule on matchPackageNames [ghcr.io/renovatebot/renovate] with its own
#                    groupName, a one-weekday schedule, updateNotScheduled false and automerge false,
#                    after the github actions rule (DUA-3)
#   T15 helm         a rule on matchPackageNames [helm/helm], groupName helm, after github actions and
#                    pinned build tools; no rule matches matchDepNames helm/helm (DUA-9)
#   T16 validator    hack/ci/renovate-config-validate.sh, run under a fake docker, refuses a digest-less
#                    renovate-image without calling docker, runs renovate-config-validator from that
#                    exact image with --rm twice (renovate.json --strict --no-global,
#                    .github/renovate-config.json --strict), and fails when either call fails (DUA-5)
#   T17 docs         the Renovate section of docs/development/tooling-setup.md names the hourly
#                    schedule, RENOVATE_TOKEN, the dashboard, the digest-only auto-merge scope,
#                    gh workflow disable, gh pr update-branch --rebase, the validator script, the
#                    unused hosted App and the release review (DUA-10)
#
# Every value is read parsed (yq), never from YAML comments; the action's version comment is
# read with yq's line_comment.
#
# Usage:
#   hack/test/renovate_policy_test.sh [root]              # check a tree (default: the repo root)
#   hack/test/renovate_policy_test.sh --self-test [root]  # mutate copies of a fixture tree
#
# --self-test builds a known-good FIXTURE tree from <root>: .github/ is copied, renovate.yaml,
# renovate.json, .github/renovate-config.json, the validator script and the docs page are written
# below, and the lint job of ci.yaml gets the validator step. It never reads the real files it
# writes, so it is green before sections 2 and 3 of the change land and stays green after them,
# and a bot bump of the real pins cannot turn it red. Each mutant
# breaks one clause of that fixture and must red with that clause's own message and no other
# clause's; a no-op control, a bot-bump control and comment decoys must stay green.
set -euo pipefail

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi
ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi

command -v yq >/dev/null 2>&1 || {
  echo "FAIL: yq (mikefarah v4) is required" >&2
  exit 1
}

TOKEN_EXPR='${{ secrets.RENOVATE_TOKEN }}'
VALIDATE_RUN='bash hack/ci/renovate-config-validate.sh'
ACTION_FLOOR='46.3.10'

REPO_EXPR='${{ github.repository }}'

# A yq read of one value inside a clause: a malformed value must not abort the guard under set -e,
# so a failed read prints nothing, and each caller treats an empty value as a violation. A read
# whose rows feed a loop, where no rows would mean a pass, uses yq directly and reds on failure.
yqv() {
  yq eval "$@" 2>/dev/null || true
}

FAILED=0
bad() {
  echo "FAIL: $1: $2"
  FAILED=1
}
good() {
  echo "ok - $1: $2"
}

# --- tree and Renovate step --------------------------------------------------------------

RW=""   # path of renovate.yaml
RW_OK=0 # 1 when renovate.yaml exists and parses
RJOB="" # job holding the Renovate step
RIDX="" # index of the Renovate step in that job
RS=""   # yq path of the Renovate step

locate() {
  RW="${ROOT}/.github/workflows/renovate.yaml"
  RW_OK=0
  if [[ ! -f "${RW}" ]]; then
    bad FILE ".github/workflows/renovate.yaml is missing -- the Renovate workflow is gone, so T1 to T7 cannot hold"
    return 0
  fi
  if ! yq eval 'true' "${RW}" >/dev/null 2>&1; then
    bad FILE ".github/workflows/renovate.yaml does not parse"
    return 0
  fi
  RW_OK=1
  local rows
  rows="$(yq eval '.jobs | to_entries | .[] | .key as $j | (.value.steps // []) | to_entries | .[] | select((.value.uses // "") | test("^renovatebot/github-action@")) | $j + " " + (.key | tostring)' "${RW}" 2>/dev/null || true)"
  RJOB=""
  RIDX=""
  RS=""
  if [[ -n "${rows}" && "$(wc -l <<<"${rows}")" -eq 1 ]]; then
    RJOB="${rows%% *}"
    RIDX="${rows##* }"
    RS=".jobs[\"${RJOB}\"].steps[${RIDX}]"
  fi
}

# Every parsed string scalar of a file, one per line (multi-line strings span lines).
strings_of() {
  yq eval '.. | select(tag == "!!str")' "$1" 2>/dev/null || true
}

no_step() {
  bad "$1" "renovate.yaml has no single step that uses renovatebot/github-action (found none or several)"
}

# --- T1 token ----------------------------------------------------------------------------

t1_token() {
  [[ "${RW_OK}" == 1 ]] || return 0
  local ok=1
  if [[ -z "${RS}" ]]; then
    no_step T1
    return 0
  fi
  local tok
  tok="$(yqv "${RS}.with.token // \"\"" "${RW}")"
  if [[ "${tok}" != "${TOKEN_EXPR}" ]]; then
    bad T1 "the Renovate step's token is '${tok}', expected exactly ${TOKEN_EXPR} -- a fallback to the repository token opens PRs that never start CI"
    ok=0
  fi
  if strings_of "${RW}" | grep -Ei 'github\.token|github\[.token.\]|secrets\.github_token|secrets\[.github_token.\]' >/dev/null; then
    bad T1 "renovate.yaml names the repository token (github.token or secrets.GITHUB_TOKEN) in a parsed value -- Renovate must run under RENOVATE_TOKEN only"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T1 "the Renovate step's token is exactly ${TOKEN_EXPR}"
}

# --- T2 fail closed ----------------------------------------------------------------------

# Runs a run: body the way the runner does with RENOVATE_TOKEN set to $3: an explicit
# `shell: bash` is `bash --noprofile --norc -eo pipefail {0}`, an unspecified shell is
# `bash -e {0}` (no pipefail). $1 is the shell value ("" or bash); prints the exit code and
# leaves the output in $4. stdin is closed and a body that ignores TERM is killed.
run_body() {
  local shell="$1" body="$2" value="$3" out="$4" dir rc=0
  local -a cmd=(bash -e)
  [[ "${shell}" == "bash" ]] && cmd=(bash --noprofile --norc -eo pipefail)
  dir="$(mktemp -d)"
  printf '%s\n' "${body}" >"${dir}/body.sh"
  (cd "${dir}" && env -i PATH="${PATH}" HOME="${dir}" RENOVATE_TOKEN="${value}" \
    timeout -k 5 20 "${cmd[@]}" "${dir}/body.sh" </dev/null) >"${out}" 2>&1 || rc=$?
  rm -rf "${dir:?}"
  echo "${rc}"
}

t2_fail_closed() {
  [[ "${RW_OK}" == 1 ]] || return 0
  if [[ -z "${RS}" ]]; then
    no_step T2
    return 0
  fi
  local i step reason="" rc out body shell
  out="$(mktemp)"
  for ((i = 0; i < RIDX; i++)); do
    step=".jobs[\"${RJOB}\"].steps[${i}]"
    [[ "$(yqv "${step}.env.RENOVATE_TOKEN // \"\"" "${RW}")" == "${TOKEN_EXPR}" ]] || continue
    if [[ "$(yqv "${step} | has(\"if\")" "${RW}")" == "true" ]]; then
      reason="the token check step has an if: -- a condition can skip the check and let Renovate start without the token"
      continue
    fi
    if [[ "$(yqv "${step} | has(\"continue-on-error\")" "${RW}")" == "true" ]]; then
      reason="the token check step has continue-on-error: -- a failed check must stop the job"
      continue
    fi
    shell="$(yq eval "${step}.shell // .jobs[\"${RJOB}\"].defaults.run.shell // .defaults.run.shell // \"\"" "${RW}" 2>/dev/null || echo '<unreadable>')"
    if [[ -n "${shell}" && "${shell}" != "bash" ]]; then
      reason="the token check step runs under shell '${shell}', not bash -- its body is judged under bash"
      continue
    fi
    if [[ "$(yqv "${step}.run | tag" "${RW}")" != "!!str" ]]; then
      reason="the token check step has no run: body"
      continue
    fi
    body="$(yqv "${step}.run" "${RW}")"
    rc="$(run_body "${shell}" "${body}" "" "${out}")"
    if [[ "${rc}" == 0 ]]; then
      reason="the token check step exits 0 when RENOVATE_TOKEN is empty -- the job must fail before Renovate starts"
      continue
    fi
    if ! grep -F 'RENOVATE_TOKEN' "${out}" >/dev/null; then
      reason="the token check step fails on an empty RENOVATE_TOKEN without naming RENOVATE_TOKEN in its message"
      continue
    fi
    rc="$(run_body "${shell}" "${body}" "probe-value-not-a-secret" "${out}")"
    if [[ "${rc}" != 0 ]]; then
      reason="the token check step exits ${rc} when RENOVATE_TOKEN is set -- it must pass a configured secret"
      continue
    fi
    rm -f "${out}"
    good T2 "step ${i} fails closed on an empty RENOVATE_TOKEN and passes a set one"
    return 0
  done
  rm -f "${out}"
  if [[ -z "${reason}" ]]; then
    reason="no step before the Renovate step sets env.RENOVATE_TOKEN to exactly ${TOKEN_EXPR} -- nothing fails the job on a missing secret"
  fi
  bad T2 "${reason}"
}

# --- T3 permissions ----------------------------------------------------------------------

perm_ok() { # $1: yq path of a permissions value; prints why it is wrong, nothing when it is fine
  local t
  t="$(yqv "$1 | tag" "${RW}")"
  case "${t}" in
    '!!map')
      if [[ "$(yqv "$1 | to_entries | map(select(.key != \"contents\" or .value != \"read\")) | length" "${RW}")" != 0 ]]; then
        echo "grants $(yq eval -o=json -I=0 "$1" "${RW}")"
      fi
      ;;
    '!!null') echo "is absent" ;;
    *) echo "is the string '$(yq eval "$1" "${RW}")'" ;;
  esac
}

t3_permissions() {
  [[ "${RW_OK}" == 1 ]] || return 0
  local ok=1 why job
  if [[ "$(yqv 'has("permissions")' "${RW}")" == "true" ]]; then
    why="$(perm_ok '.permissions')"
    if [[ -n "${why}" ]]; then
      bad T3 "the workflow permissions ${why}, expected contents: read only"
      ok=0
    fi
  fi
  while IFS= read -r job; do
    [[ -n "${job}" ]] || continue
    if [[ "$(yqv ".jobs[\"${job}\"] | has(\"permissions\")" "${RW}")" == "true" ]]; then
      why="$(perm_ok ".jobs[\"${job}\"].permissions")"
    else
      why="$(perm_ok '.permissions')"
    fi
    if [[ -n "${why}" ]]; then
      bad T3 "job '${job}': its effective permissions ${why}, expected contents: read only -- Renovate writes with RENOVATE_TOKEN, never with GITHUB_TOKEN"
      ok=0
    fi
  done < <(yq eval '.jobs // {} | keys | .[]' "${RW}" 2>/dev/null || true)
  [[ "${ok}" == 0 ]] || good T3 "every job grants GITHUB_TOKEN contents: read only"
}

# --- T4 schedule -------------------------------------------------------------------------

t4_schedule() {
  [[ "${RW_OK}" == 1 ]] || return 0
  local ok=1 trig cron f daily=0
  trig="$(yqv '."on" | select(tag == "!!map") | keys | sort | join(" ")' "${RW}" 2>/dev/null || true)"
  if [[ "${trig}" != "schedule workflow_dispatch" ]]; then
    bad T4 "the triggers are '$(yq eval -o=json -I=0 '."on"' "${RW}")', expected exactly schedule and workflow_dispatch"
    ok=0
  fi
  while IFS= read -r cron; do
    read -r -a f <<<"${cron}"
    [[ "${#f[@]}" -eq 5 ]] || continue
    [[ "${f[0]}" =~ ^[0-9]{1,2}$ ]] && ((10#${f[0]} <= 59)) || continue
    [[ "${f[1]}" == '*' || "${f[1]}" =~ ^\*/[0-9]+$ ]] || { [[ "${f[1]}" =~ ^[0-9]{1,2}$ ]] && ((10#${f[1]} <= 23)); } || continue
    [[ "${f[2]}" == '*' && "${f[3]}" == '*' && "${f[4]}" == '*' ]] || continue
    daily=1
  done < <(yq eval '.["on"].schedule // [] | .[] | .cron // ""' "${RW}" 2>/dev/null || true)
  if [[ "${daily}" != 1 ]]; then
    bad T4 "no cron entry fires every day (numeric minute; hour a number, * or */N; * for day-of-month, month and day-of-week)"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T4 "schedule and workflow_dispatch only, and a cron entry fires every day"
}

# --- T5 concurrency ----------------------------------------------------------------------

t5_concurrency() {
  [[ "${RW_OK}" == 1 ]] || return 0
  local ok=1
  if [[ "$(yqv '.concurrency.group | tag' "${RW}" 2>/dev/null || true)" != '!!str' ||
    "$(yqv '.concurrency.group' "${RW}")" != "renovate" ]]; then
    bad T5 "concurrency.group is not the literal 'renovate' -- a per-run group never serialises two runs"
    ok=0
  fi
  if [[ "$(yqv '.concurrency["cancel-in-progress"] | tag' "${RW}" 2>/dev/null || true)" != '!!bool' ||
    "$(yqv '.concurrency["cancel-in-progress"]' "${RW}")" != "false" ]]; then
    bad T5 "concurrency.cancel-in-progress is not the boolean false -- a queued run must not cancel one mid-merge"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T5 "one fixed concurrency group, cancel-in-progress false"
}

# --- T6 pin and inputs -------------------------------------------------------------------

version_at_least() { # $1 >= $2, both X.Y.Z
  local a b i
  IFS=. read -r -a a <<<"$1"
  IFS=. read -r -a b <<<"$2"
  for i in 0 1 2; do
    ((10#${a[i]} > 10#${b[i]})) && return 0
    ((10#${a[i]} < 10#${b[i]})) && return 1
  done
  return 0
}

t6_pin() {
  [[ "${RW_OK}" == 1 ]] || return 0
  if [[ -z "${RS}" ]]; then
    no_step T6
    return 0
  fi
  local ok=1 uses ver image extra
  uses="$(yqv "${RS}.uses" "${RW}")"
  if [[ ! "${uses}" =~ ^renovatebot/github-action@[0-9a-f]{40}$ ]]; then
    bad T6 "the Renovate step uses '${uses}', expected renovatebot/github-action@<40-hex commit SHA>"
    ok=0
  fi
  ver="$(yqv "${RS}.uses | line_comment" "${RW}")"
  if [[ ! "${ver}" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)$ ]] || ! version_at_least "${BASH_REMATCH[1]}" "${ACTION_FLOOR}"; then
    bad T6 "the Renovate action's version comment is '# ${ver}', expected '# vX.Y.Z' of at least v${ACTION_FLOOR} (DUA-3: a release that takes the full digest-pinned renovate-image reference)"
    ok=0
  fi
  image="$(yqv "${RS}.with[\"renovate-image\"] // \"\"" "${RW}")"
  if [[ ! "${image}" =~ ^ghcr\.io/renovatebot/renovate:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}$ ]]; then
    bad T6 "renovate-image is '${image}', expected ghcr.io/renovatebot/renovate:<X.Y.Z>@sha256:<64 hex> -- a floating or digest-less image changes the Renovate that runs without a review"
    ok=0
  fi
  if [[ "$(yqv "${RS}.with | tag" "${RW}")" == '!!map' ]]; then
    extra="$(yqv "${RS}.with | keys | map(select(. != \"configurationFile\" and . != \"token\" and . != \"renovate-image\")) | join(\" \")" "${RW}")"
  else
    extra="<with: is not a mapping>"
  fi
  if [[ -n "${extra}" ]]; then
    bad T6 "the Renovate step's with: has '${extra}' besides configurationFile, token and renovate-image"
    ok=0
  fi
  # The global config the policy checks is .github/renovate-config.json; another file would carry
  # a global config no clause reads.
  if [[ "$(yqv "${RS}.with.configurationFile" "${RW}")" != ".github/renovate-config.json" ]]; then
    bad T6 "the Renovate step's configurationFile is '$(yqv "${RS}.with.configurationFile" "${RW}")', expected exactly .github/renovate-config.json"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T6 "action SHA-pinned at ${ver}, renovate-image pinned by digest"
}

# --- T7 environment ----------------------------------------------------------------------

t7_environment() {
  [[ "${RW_OK}" == 1 ]] || return 0
  local ok=1 p ptag pval ntok=0 rows
  local re_repo='^(env|jobs\.[^.]+\.env|jobs\.[^.]+\.steps\.[0-9]+\.env)\.RENOVATE_REPOSITORIES$'
  local re_tok='^jobs\.([^.]+)\.steps\.([0-9]+)\.env\.RENOVATE_TOKEN$'
  if ! rows="$(yq eval '.. | select((key | tag) == "!!str" and (key | test("(?i)^renovate_"))) | (path | join(".")) + "\t" + tag + "\t" + (. | @json)' "${RW}" 2>&1)"; then
    bad T7 "the RENOVATE_* keys of renovate.yaml cannot be read (${rows%%$'\n'*})"
    return 0
  fi
  while IFS=$'\t' read -r p ptag pval; do
    [[ -n "${p}" ]] || continue
    if [[ "${p}" =~ ${re_repo} ]]; then
      if [[ "${ptag}" != '!!str' || "${pval}" != "\"${REPO_EXPR}\"" ]]; then
        bad T7 "'${p}' is ${pval}, expected exactly ${REPO_EXPR} -- the maintainer token must not point Renovate at another repository"
        ok=0
      fi
      continue
    fi
    # The token check step's RENOVATE_TOKEN (T2): one run step, without uses:, before the Renovate
    # step in its job. Anywhere else the secret reaches code that does not need it.
    if [[ "${p}" =~ ${re_tok} && "${BASH_REMATCH[1]}" == "${RJOB}" && -n "${RIDX}" ]] && ((BASH_REMATCH[2] < RIDX)) &&
      [[ "$(yqv ".jobs[\"${BASH_REMATCH[1]}\"].steps[${BASH_REMATCH[2]}] | has(\"run\") and (has(\"uses\") | not)" "${RW}")" == "true" ]]; then
      ntok=$((ntok + 1))
      if ((ntok > 1)); then
        bad T7 "'${p}' sets RENOVATE_TOKEN on a second step -- only the token check step may hold the secret"
        ok=0
      fi
      continue
    fi
    bad T7 "'${p}' sets a RENOVATE_* variable -- only RENOVATE_REPOSITORIES and the RENOVATE_TOKEN of the token check step (before the Renovate step) are allowed; an override such as RENOVATE_FORCE can drop the schedule or force automerge"
    ok=0
  done <<<"${rows}"
  while IFS= read -r p; do
    [[ -n "${p}" ]] || continue
    bad T7 "'${p}' is an env: that is not a mapping -- its RENOVATE_* keys cannot be checked"
    ok=0
  done < <(yq eval '.. | select((key | tag) == "!!str" and key == "env" and tag != "!!map") | path | join(".")' "${RW}" 2>/dev/null || true)
  if strings_of "${RW}" | grep -F 'GITHUB_ENV' >/dev/null; then
    bad T7 "renovate.yaml names GITHUB_ENV in a parsed value -- a step can set RENOVATE_* variables for later steps through it"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T7 "no RENOVATE_* override besides the repository name and the token check"
}

# --- T8 validator wiring -----------------------------------------------------------------

t8_validator() {
  local ci="${ROOT}/.github/workflows/ci.yaml" n
  if [[ ! -f "${ci}" ]]; then
    bad T8 ".github/workflows/ci.yaml is missing -- nothing runs the config validator"
    return 0
  fi
  n="$(VR="${VALIDATE_RUN}" yq eval '[.jobs.lint.steps // [] | .[] | select(.run == strenv(VR))] | length' "${ci}" 2>/dev/null || echo 0)"
  if [[ "${n}" == 0 ]]; then
    bad T8 "the lint job has no step whose run is exactly '${VALIDATE_RUN}' -- the Renovate config is not validated in a required job"
    return 0
  fi
  n="$(VR="${VALIDATE_RUN}" yq eval '[.jobs.lint.steps[] | select(.run == strenv(VR)) | select((has("if") or has("continue-on-error")) | not)] | length' "${ci}" 2>/dev/null || echo 0)"
  if [[ "${n}" == 0 ]]; then
    bad T8 "the lint job's '${VALIDATE_RUN}' step has an if: or continue-on-error: -- the validation can be skipped or ignored"
    return 0
  fi
  good T8 "lint runs '${VALIDATE_RUN}' unconditionally"
}

# --- T9 no merge workflow ----------------------------------------------------------------

# Workflows, and the composite actions they call (a step there runs inside the calling job).
t9_no_merge() {
  local ok=1 wf rel bodies trig
  for wf in "${ROOT}"/.github/workflows/*.yaml "${ROOT}"/.github/workflows/*.yml \
    "${ROOT}"/.github/actions/*/action.yaml "${ROOT}"/.github/actions/*/action.yml; do
    [[ -f "${wf}" ]] || continue
    rel="${wf#"${ROOT}"/}"
    if ! bodies="$(yq eval '.. | select(tag == "!!map") | (.run, (.with | select(tag == "!!map") | .script)) | select(tag == "!!str")' "${wf}" 2>&1)"; then
      bad T9 "${rel} cannot be read (${bodies%%$'\n'*}) -- its run: bodies cannot be checked"
      ok=0
      continue
    fi
    if grep -E 'pulls\.merge|gh[[:space:]]+pr[[:space:]]+merge|pulls/[^/[:space:]]+/merge|enablePullRequestAutoMerge|mergePullRequest' <<<"${bodies}" >/dev/null; then
      bad T9 "${rel} merges pull requests from a run: body or github-script (pulls.merge, gh pr merge, the pulls/<n>/merge API, enablePullRequestAutoMerge or mergePullRequest) -- only Renovate merges, after every check"
      ok=0
    fi
    if ! trig="$(yq eval '."on" | ((select(tag == "!!map") | keys | .[]), (select(tag == "!!seq") | .[]), select(tag == "!!str"))' "${wf}" 2>&1)"; then
      bad T9 "${rel}: its triggers cannot be read (${trig%%$'\n'*})"
      ok=0
      continue
    fi
    if grep -Fx 'pull_request_target' <<<"${trig}" >/dev/null; then
      bad T9 "${rel} triggers on pull_request_target -- a workflow with a write token on untrusted PR code"
      ok=0
    fi
  done
  [[ "${ok}" == 0 ]] || good T9 "no workflow merges pull requests or uses pull_request_target"
}

# --- config files (T10 to T15) -------------------------------------------------------------

RJ=""     # path of renovate.json (repo config)
GC=""     # path of .github/renovate-config.json (global config)
CFG_OK=0  # 1 when both exist and parse as JSON
ROWS=""   # rows of the last cfg_rows call
RULES=""  # rows of the last rules_rows call

# One value of a JSON config as compact JSON ("null" when absent); a failed read prints nothing,
# and every caller compares against an exact expected value, so nothing reads as a pass.
jv() {
  yq -p json -o json -I0 "$@" 2>/dev/null || true
}

locate_config() {
  RJ="${ROOT}/renovate.json"
  GC="${ROOT}/.github/renovate-config.json"
  CFG_OK=1
  local f rel
  for f in "${RJ}" "${GC}"; do
    rel="${f#"${ROOT}"/}"
    if [[ ! -f "${f}" ]]; then
      bad FILE "${rel} is missing -- T10 to T15 cannot hold"
      CFG_OK=0
    elif ! yq -p json 'true' "${f}" >/dev/null 2>&1; then
      bad FILE "${rel} does not parse as JSON -- T10 to T15 cannot be checked"
      CFG_OK=0
    fi
  done
}

# Runs a yq expression (one string per row) over both config files and leaves the rows in ROWS,
# each prefixed "<file>:". Called directly (never in $(...)): a read error reds clause $1, since
# no rows would read as a pass.
cfg_rows() { # $1 clause, $2 yq expression
  local f rel out line
  ROWS=""
  for f in "${RJ}" "${GC}"; do
    rel="${f#"${ROOT}"/}"
    if ! out="$(yq -p json -oy "$2" "${f}" 2>&1)"; then
      bad "$1" "${rel} cannot be read (${out%%$'\n'*})"
      continue
    fi
    while IFS= read -r line; do
      if [[ -n "${line}" ]]; then
        ROWS+="${rel}:${line}"$'\n'
      fi
    done <<<"${out}"
  done
}

# One row per packageRules entry of renovate.json: index, then "=" + compact JSON (just "=" when
# absent) of matchPackageNames, matchDepNames, groupName, schedule, updateNotScheduled, automerge.
# The "=" keeps empty fields from collapsing under IFS=tab. Returns non-zero on a read error.
rules_rows() {
  local out
  RULES=""
  if ! out="$(yq -p json -oy '.packageRules // [] | to_entries | .[] | (.key | tostring) + "\t=" + (.value.matchPackageNames | @json) + "\t=" + (.value.matchDepNames | @json) + "\t=" + (.value.groupName | @json) + "\t=" + (.value.schedule | @json) + "\t=" + (.value.updateNotScheduled | @json) + "\t=" + (.value.automerge | @json)' "${RJ}" 2>&1)"; then
    return 1
  fi
  RULES="${out}"
}

# --- T10 dashboard and presets ------------------------------------------------------------

t10_presets() {
  [[ "${CFG_OK}" == 1 ]] || return 0
  local ok=1 ext row
  ext="$(jv '.extends' "${RJ}")"
  if [[ "${ext}" != '["config:recommended"]' ]]; then
    bad T10 "renovate.json's top-level extends is ${ext:-unreadable}, expected exactly [\"config:recommended\"] -- any other preset (:disableDependencyDashboard, :automergeAll, a remote preset) changes the policy outside this file"
    ok=0
  fi
  cfg_rows T10 '.. | select(tag == "!!map" and has("extends")) | "/" + (path | join("."))'
  while IFS= read -r row; do
    [[ -n "${row}" && "${row}" != "renovate.json:/" ]] || continue
    bad T10 "'${row}' has an extends key -- a nested preset such as :automergeAll enables automerge without a literal automerge: true"
    ok=0
  done <<<"${ROWS}"
  cfg_rows T10 '.. | select(tag == "!!map" and has("dependencyDashboard") and ((.dependencyDashboard | tag) != "!!bool" or .dependencyDashboard != true)) | "/" + (path | join("."))'
  while IFS= read -r row; do
    [[ -n "${row}" ]] || continue
    bad T10 "'${row}' sets dependencyDashboard to something other than true -- the dashboard is where pending, rate-limited and errored updates show (DUA-4)"
    ok=0
  done <<<"${ROWS}"
  [[ "${ok}" == 0 ]] || good T10 "extends is exactly [\"config:recommended\"], no nested preset, dashboard not disabled"
}

# --- T11 automerge scope -------------------------------------------------------------------

t11_automerge() {
  [[ "${CFG_OK}" == 1 ]] || return 0
  local ok=1 rows n idx v
  cfg_rows T11 '.. | select(tag == "!!map" and has("automerge") and (.automerge | tag) == "!!bool" and .automerge == true) | "/" + (path | join("."))'
  rows="${ROWS%$'\n'}"
  n=0
  [[ -z "${rows}" ]] || n="$(wc -l <<<"${rows}")"
  if [[ "${n}" != 1 ]]; then
    rows="${rows//$'\n'/, }"
    bad T11 "${n} objects set automerge: true (${rows:-none}), expected exactly 1, the digest rule -- automerge anywhere else merges updates no one reviewed (DUA-7)"
    return 0
  fi
  if [[ ! "${rows}" =~ ^renovate\.json:/packageRules\.([0-9]+)$ ]]; then
    bad T11 "automerge: true is set by '${rows}', expected a packageRules entry of renovate.json"
    return 0
  fi
  idx="${BASH_REMATCH[1]}"
  v="$(jv ".packageRules[${idx}] | keys | map(select(. != \"description\")) | sort" "${RJ}")"
  if [[ "${v}" != '["automerge","matchManagers","matchUpdateTypes","rebaseWhen"]' ]]; then
    bad T11 "the automerge rule packageRules[${idx}] has keys ${v} besides an optional description, expected exactly automerge, matchManagers, matchUpdateTypes, rebaseWhen -- any other key changes what it matches"
    ok=0
  fi
  v="$(jv ".packageRules[${idx}].matchManagers | sort" "${RJ}")"
  if [[ "${v}" != '["devcontainer","dockerfile"]' ]]; then
    bad T11 "the automerge rule's matchManagers is ${v:-not a list}, expected [\"devcontainer\",\"dockerfile\"]"
    ok=0
  fi
  v="$(jv ".packageRules[${idx}].matchUpdateTypes" "${RJ}")"
  if [[ "${v}" != '["digest"]' ]]; then
    bad T11 "the automerge rule's matchUpdateTypes is ${v}, expected [\"digest\"]"
    ok=0
  fi
  v="$(jv ".packageRules[${idx}].rebaseWhen" "${RJ}")"
  if [[ "${v}" != '"behind-base-branch"' ]]; then
    bad T11 "the automerge rule's rebaseWhen is ${v}, expected \"behind-base-branch\" -- a PR behind main must be rebased and pass again before it merges (DUA-8)"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T11 "one automerge: true, on the digest-only dockerfile/devcontainer rule"
}

# --- T12 merge settings --------------------------------------------------------------------

t12_merge() {
  [[ "${CFG_OK}" == 1 ]] || return 0
  local ok=1 k want v loc keys am key
  local -a ks
  for k in 'platformAutomerge=false' 'automergeType="pr"' 'automergeStrategy="rebase"' 'rebaseWhen="conflicted"'; do
    want="${k#*=}"
    k="${k%%=*}"
    v="$(jv ".${k}" "${RJ}")"
    if [[ "${v}" != "${want}" ]]; then
      bad T12 "renovate.json's top-level ${k} is ${v}, expected ${want} (DUA-8)"
      ok=0
    fi
  done
  cfg_rows T12 '.. | select(tag == "!!map") | "/" + (path | join(".")) + "\t=" + (keys | map(select(. == "platformAutomerge" or . == "automergeStrategy" or . == "automergeType" or . == "rebaseWhen" or . == "ignoreTests")) | join(",")) + "\t=" + ((has("automerge") and (.automerge | tag) == "!!bool" and .automerge == true) | tostring)'
  # The "=" before each field keeps an empty key list from collapsing under IFS=tab.
  while IFS=$'\t' read -r loc keys am; do
    keys="${keys#=}"
    am="${am#=}"
    [[ -n "${loc}" && -n "${keys}" ]] || continue
    IFS=, read -r -a ks <<<"${keys}"
    for key in "${ks[@]}"; do
      if [[ "${key}" == "ignoreTests" ]]; then
        bad T12 "'${loc}' has an ignoreTests key -- a bot merge must wait for every check"
        ok=0
      elif [[ "${loc}" == "renovate.json:/" ]]; then
        continue
      elif [[ "${key}" == "rebaseWhen" && "${am}" == "true" && "${loc}" =~ ^renovate\.json:/packageRules\.[0-9]+$ ]]; then
        continue
      else
        bad T12 "'${loc}' sets ${key} -- only renovate.json's top level (and rebaseWhen on the automerge rule) may set merge settings"
        ok=0
      fi
    done
  done <<<"${ROWS}"
  [[ "${ok}" == 0 ]] || good T12 "platformAutomerge false, pr, rebase, conflicted at the top level only; no ignoreTests"
}

# --- T13 global config ---------------------------------------------------------------------

t13_global() {
  [[ "${CFG_OK}" == 1 ]] || return 0
  local ok=1 v
  v="$(jv 'keys | sort' "${GC}")"
  if [[ "${v}" != '["$schema","onboarding","requireConfig"]' ]]; then
    bad T13 ".github/renovate-config.json has keys ${v:-unreadable}, expected exactly \$schema, onboarding and requireConfig -- a global option overrides the repository policy (DUA-7, DUA-8)"
    ok=0
  fi
  v="$(jv '.onboarding' "${GC}")"
  if [[ "${v}" != 'false' ]]; then
    bad T13 ".github/renovate-config.json's onboarding is ${v}, expected false"
    ok=0
  fi
  v="$(jv '.requireConfig' "${GC}")"
  if [[ "${v}" != '"required"' ]]; then
    bad T13 ".github/renovate-config.json's requireConfig is ${v}, expected \"required\""
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T13 "global config is \$schema, onboarding false, requireConfig required"
}

# --- T14 Renovate image rule ---------------------------------------------------------------

shown() { # a rules_rows field for a message: its JSON, or "absent"
  local v="${1#=}"
  echo "${v:-absent}"
}

t14_image_rule() {
  [[ "${CFG_OK}" == 1 ]] || return 0
  if ! rules_rows; then
    bad T14 "renovate.json's packageRules cannot be read"
    return 0
  fi
  local idx mpn mdn grp sch uns am ga=-1 found=0 r reasons=""
  local -A groups=()
  while IFS=$'\t' read -r idx mpn mdn grp sch uns am; do
    [[ -n "${idx}" ]] || continue
    [[ "${grp}" != '="github actions"' ]] || ga="${idx}"
    [[ "${grp}" == "=" ]] || groups["${grp}"]=$((${groups["${grp}"]:-0} + 1))
  done <<<"${RULES}"
  while IFS=$'\t' read -r idx mpn mdn grp sch uns am; do
    [[ "${mpn}" == '=["ghcr.io/renovatebot/renovate"]' ]] || continue
    found=1
    r=""
    if [[ ! "${grp}" =~ ^=\"[^\"]+\"$ || "${grp}" == '="github actions"' || "${groups["${grp}"]}" != 1 ]]; then
      r+="its groupName is $(shown "${grp}"), expected one of its own; "
    fi
    [[ "${sch}" =~ ^=\[\"\*\ \*\ \*\ \*\ [0-7]\"\]$ ]] || r+="its schedule is $(shown "${sch}"), expected one whole weekday such as [\"* * * * 1\"]; "
    [[ "${uns}" == "=false" ]] || r+="its updateNotScheduled is $(shown "${uns}"), expected false; "
    [[ "${am}" == "=false" ]] || r+="its automerge is $(shown "${am}"), expected false; "
    ((ga >= 0 && idx > ga)) || r+="it is not after the 'github actions' rule (a later groupName wins); "
    if [[ -z "${r}" ]]; then
      good T14 "packageRules[${idx}] groups the Renovate image alone, weekly, never automerged"
      return 0
    fi
    reasons+="packageRules[${idx}]: ${r}"
  done <<<"${RULES}"
  if [[ "${found}" == 0 ]]; then
    bad T14 "no packageRules entry has matchPackageNames exactly [\"ghcr.io/renovatebot/renovate\"] -- the Renovate image lands in the 'github actions' group, outside its weekly window (DUA-3)"
  else
    bad T14 "the Renovate image rule: ${reasons%; }"
  fi
}

# --- T15 helm ------------------------------------------------------------------------------

t15_helm() {
  [[ "${CFG_OK}" == 1 ]] || return 0
  if ! rules_rows; then
    bad T15 "renovate.json's packageRules cannot be read"
    return 0
  fi
  local ok=1 idx mpn mdn grp sch uns am ga=-1 pbt=-1 helm=-1
  while IFS=$'\t' read -r idx mpn mdn grp sch uns am; do
    [[ -n "${idx}" ]] || continue
    [[ "${grp}" != '="github actions"' ]] || ga="${idx}"
    [[ "${grp}" != '="pinned build tools"' ]] || pbt="${idx}"
    [[ "${mpn}" != '=["helm/helm"]' || "${grp}" != '="helm"' ]] || helm="${idx}"
    if [[ "${mdn}" == *'"helm/helm"'* ]]; then
      bad T15 "packageRules[${idx}] still matches matchDepNames helm/helm -- the azure/setup-helm input in release.yaml has depName helm, so a depName rule leaves it in another group (DUA-9)"
      ok=0
    fi
  done <<<"${RULES}"
  if ((helm < 0)); then
    bad T15 "no rule has matchPackageNames [\"helm/helm\"] with groupName helm -- the Helm pins drift apart (DUA-9)"
    ok=0
  elif ((ga < 0 || pbt < 0 || helm < ga || helm < pbt)); then
    bad T15 "the helm rule packageRules[${helm}] is not after both the 'github actions' (${ga}) and the 'pinned build tools' (${pbt}) rules -- a later groupName wins and pulls Helm pins out of the helm group"
    ok=0
  fi
  [[ "${ok}" == 0 ]] || good T15 "packageRules[${helm}] groups every helm/helm pin as helm, after github actions and pinned build tools"
}

# --- T16 validator behaviour ---------------------------------------------------------------

VALIDATOR_REL="hack/ci/renovate-config-validate.sh"
# An implausible image the scratch workflow carries: a script that hard-codes the real pin fails.
T16_IMAGE="ghcr.io/renovatebot/renovate:0.0.1@sha256:$(printf 'a%.0s' {1..64})"
T16_RC=0

# A fake docker: records each call's argv (one argument per line) in $FAKE_DOCKER_DIR/call.<n>;
# `docker run` calls exit with the next status of $FAKE_DOCKER_RCS, any other call exits 0.
write_fake_docker() {
  cat >"$1" <<'EOF'
#!/usr/bin/env bash
d="${FAKE_DOCKER_DIR:?}"
n=$(($(cat "${d}/count" 2>/dev/null || echo 0) + 1))
echo "${n}" >"${d}/count"
printf '%s\n' "$@" >"${d}/call.${n}"
if [[ "${1:-}" == "run" ]]; then
  r=$(($(cat "${d}/runs" 2>/dev/null || echo 0) + 1))
  echo "${r}" >"${d}/runs"
  read -r -a rcs <<<"${FAKE_DOCKER_RCS:-}"
  exit "${rcs[r - 1]:-0}"
fi
exit 0
EOF
  chmod +x "$1"
}

# Runs the tree's validator script on a scratch root whose renovate.yaml carries image $2, with
# the fake docker first on PATH and run statuses $3. Sets T16_RC; the calls land in $1/calls.
t16_run() { # $1 case dir, $2 image, $3 statuses
  local dir="$1" root="$1/root" f
  mkdir -p "${root}/hack/ci" "${root}/.github/workflows" "${dir}/calls" "${dir}/bin"
  cp "${ROOT}/${VALIDATOR_REL}" "${root}/${VALIDATOR_REL}"
  write_fixture_workflow "${root}/.github/workflows/renovate.yaml"
  V="$2" yq eval -i '.jobs.renovate.steps[2].with["renovate-image"] = strenv(V)' "${root}/.github/workflows/renovate.yaml"
  for f in renovate.json .github/renovate-config.json; do
    if [[ -f "${ROOT}/${f}" ]]; then cp "${ROOT}/${f}" "${root}/${f}"; else echo '{}' >"${root}/${f}"; fi
  done
  write_fake_docker "${dir}/bin/docker"
  T16_RC=0
  (cd "${root}" && env -i PATH="${dir}/bin:${PATH}" HOME="${dir}" FAKE_DOCKER_DIR="${dir}/calls" FAKE_DOCKER_RCS="$3" \
    timeout -k 5 60 bash "${root}/${VALIDATOR_REL}" "${root}" </dev/null) >"${dir}/out" 2>&1 || T16_RC=$?
}

# Judges one recorded `docker run` call: prints "repo <flags>" or "global <flags>" (flags sorted,
# space-separated), or "!<why>" when the call does not run the validator from T16_IMAGE.
t16_judge_call() {
  local -a a v flags=() files=()
  local i img=-1 rm=0 ep="" f kind
  mapfile -t a <"$1"
  for ((i = 1; i < ${#a[@]}; i++)); do
    if [[ "${a[i]}" == "${T16_IMAGE}" ]]; then
      img="${i}"
      break
    fi
  done
  if ((img < 0)); then
    echo "!does not name the renovate-image of renovate.yaml (${T16_IMAGE}) as its image: $(paste -sd' ' "$1")"
    return 0
  fi
  for ((i = 1; i < img; i++)); do
    case "${a[i]}" in
      --rm) rm=1 ;;
      --entrypoint) ep="${a[i + 1]:-}" ;;
      --entrypoint=*) ep="${a[i]#--entrypoint=}" ;;
    esac
  done
  if ((rm == 0)); then
    echo "!has no --rm: $(paste -sd' ' "$1")"
    return 0
  fi
  v=("${a[@]:img+1}")
  if [[ -z "${ep}" ]]; then
    ep="${v[0]:-}"
    v=("${v[@]:1}")
  fi
  if [[ "${ep}" != "renovate-config-validator" && "${ep}" != */renovate-config-validator ]]; then
    echo "!runs '${ep}', not renovate-config-validator: $(paste -sd' ' "$1")"
    return 0
  fi
  for f in "${v[@]}"; do
    if [[ "${f}" == -* ]]; then flags+=("${f}"); else files+=("${f}"); fi
  done
  if ((${#files[@]} != 1)); then
    echo "!validates ${#files[@]} files, expected one per call: $(paste -sd' ' "$1")"
    return 0
  fi
  f="${files[0]}"
  if [[ "${f}" =~ ^(.*/)?\.github/renovate-config\.json$ ]]; then
    kind=global
  elif [[ "${f}" =~ ^(.*/)?renovate\.json$ && "${f}" != *.github/* ]]; then
    kind=repo
  else
    echo "!validates '${f}', neither renovate.json nor .github/renovate-config.json"
    return 0
  fi
  echo "${kind} $(printf '%s\n' "${flags[@]}" | sort | paste -sd' ')"
}

t16_validator() {
  if [[ ! -f "${ROOT}/${VALIDATOR_REL}" ]]; then
    bad T16 "${VALIDATOR_REL} is missing -- nothing validates the Renovate config (DUA-5)"
    return 0
  fi
  local ok=1 work n c j kinds="" rcs pos
  work="$(mktemp -d)"

  t16_run "${work}/nodigest" "ghcr.io/renovatebot/renovate:0.0.1" "0 0"
  n="$(find "${work}/nodigest/calls" -name 'call.*' | wc -l)"
  if ((n > 0)); then
    bad T16 "with a renovate-image without a digest the script called docker ${n} time(s) -- it must refuse the image before running anything"
    ok=0
  fi
  if [[ "${T16_RC}" == 0 ]]; then
    bad T16 "with a renovate-image without a digest the script exits 0 -- it must refuse an image without a digest"
    ok=0
  fi

  t16_run "${work}/green" "${T16_IMAGE}" "0 0"
  n=0
  for c in "${work}/green/calls"/call.*; do
    [[ -f "${c}" && "$(head -n1 "${c}")" == "run" ]] || continue
    n=$((n + 1))
    j="$(t16_judge_call "${c}")"
    if [[ "${j}" == '!'* ]]; then
      bad T16 "a docker run call ${j#!}"
      ok=0
    else
      kinds+="${j}"$'\n'
    fi
  done
  if ((n != 2)); then
    bad T16 "with a digest-pinned renovate-image the script made ${n} docker run call(s), expected 2 (renovate.json, .github/renovate-config.json)"
    ok=0
  fi
  if ! grep -Fx 'repo --no-global --strict' <<<"${kinds}" >/dev/null; then
    bad T16 "no docker run call validates renovate.json with exactly --strict --no-global (repo config)"
    ok=0
  fi
  if ! grep -Fx 'global --strict' <<<"${kinds}" >/dev/null; then
    bad T16 "no docker run call validates .github/renovate-config.json with exactly --strict (global config, no --no-global)"
    ok=0
  fi
  if [[ "${T16_RC}" != 0 ]]; then
    bad T16 "the script exits ${T16_RC} when both validator calls pass -- a valid config must pass"
    ok=0
  fi

  pos=1
  for rcs in "1 0" "0 1"; do
    t16_run "${work}/red${pos}" "${T16_IMAGE}" "${rcs}"
    if [[ "${T16_RC}" == 0 ]]; then
      bad T16 "the script exits 0 when docker run call ${pos} of 2 exits 1 -- a failed validation must fail the lint job"
      ok=0
    fi
    pos=$((pos + 1))
  done
  rm -rf "${work:?}"
  [[ "${ok}" == 0 ]] || good T16 "refuses a digest-less image, validates both files in their modes from the pinned image, fails on either failure"
}

# --- T17 docs ------------------------------------------------------------------------------

DOCS_REL="docs/development/tooling-setup.md"
# name|mode|pattern: mode F is a fixed string, i a case-insensitive ERE; an item with several
# rows needs all of them in the section.
T17_ITEMS=(
  "the hourly schedule|i|hourly"
  "RENOVATE_TOKEN|F|RENOVATE_TOKEN"
  "the dependency dashboard|i|dependency dashboard"
  "the digest-only auto-merge scope|i|auto-?merge"
  "the digest-only auto-merge scope|i|digest"
  "the digest-only auto-merge scope|i|dockerfile"
  "the digest-only auto-merge scope|i|devcontainer"
  "gh workflow disable|F|gh workflow disable"
  "gh pr update-branch --rebase|F|gh pr update-branch --rebase"
  "hack/ci/renovate-config-validate.sh|F|hack/ci/renovate-config-validate.sh"
  "the unused hosted App|i|hosted Renovate App"
  "the unused hosted App|i|unused|not used|not installed"
  "the release review|i|since the last tag"
  "the release review|i|release"
)

t17_docs() {
  local doc="${ROOT}/${DOCS_REL}" section item name mode pat missing=""
  if [[ ! -f "${doc}" ]]; then
    bad T17 "${DOCS_REL} is missing -- the maintainer docs do not describe the bot (DUA-10)"
    return 0
  fi
  # The section: from the first level-2 heading naming Renovate to the next level-1/2 heading,
  # headings inside fenced code blocks ignored.
  if ! section="$(awk '/^```/ { f = !f } !f && /^##? / { if (s) exit; if ($0 ~ /Renovate/) { s = 1; found = 1; next } } s { print } END { exit !found }' "${doc}")"; then
    bad T17 "${DOCS_REL} has no '## ... Renovate ...' section (DUA-10)"
    return 0
  fi
  for item in "${T17_ITEMS[@]}"; do
    IFS='|' read -r name mode pat <<<"${item}"
    [[ "${missing}" != *"<${name}>"* ]] || continue
    if [[ "${mode}" == F ]]; then
      grep -F -- "${pat}" <<<"${section}" >/dev/null && continue
    else
      grep -Ei -- "${pat}" <<<"${section}" >/dev/null && continue
    fi
    missing+="<${name}>"
    bad T17 "the Renovate section of ${DOCS_REL} does not name ${name} (/${pat}/) (DUA-10)"
  done
  [[ -n "${missing}" ]] || good T17 "the Renovate section names schedule, token, dashboard, scope, pause, rebase, validation, App and release review"
}

# --- the clause table ---------------------------------------------------------------------

CLAUSES=(t1_token t2_fail_closed t3_permissions t4_schedule t5_concurrency t6_pin t7_environment t8_validator t9_no_merge
  t10_presets t11_automerge t12_merge t13_global t14_image_rule t15_helm t16_validator t17_docs)

check_tree() {
  locate
  locate_config
  local c
  for c in "${CLAUSES[@]}"; do
    "${c}"
  done
  if [[ "${FAILED}" != 0 ]]; then
    echo "renovate policy: FAILED (${ROOT})"
    return 1
  fi
  echo "renovate policy: all clauses hold"
}

# --- self-test ----------------------------------------------------------------------------

# The known-good workflow (DUA-1 to DUA-3, DUA-7, DUA-8). Not the real renovate.yaml: see the
# header. The image and action pins are the ones evidence 0.4/0.5 of the change resolved.
write_fixture_workflow() {
  cat >"$1" <<'EOF'
# Self-hosted Renovate: hourly, under the maintainer token only (self-test fixture).
name: Renovate

on:
  schedule:
    - cron: "17 * * * *"
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: renovate
  cancel-in-progress: false

jobs:
  renovate:
    name: renovate
    runs-on: ubuntu-latest
    permissions:
      contents: read
    env:
      RENOVATE_REPOSITORIES: ${{ github.repository }}
      LOG_LEVEL: ${{ github.event_name == 'workflow_dispatch' && 'debug' || '' }}
    steps:
      - name: Require RENOVATE_TOKEN
        env:
          RENOVATE_TOKEN: ${{ secrets.RENOVATE_TOKEN }}
        run: |
          if [ -z "${RENOVATE_TOKEN}" ]; then
            echo "::error::RENOVATE_TOKEN is not set" >&2
            exit 1
          fi
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: renovatebot/github-action@2804199d260ad00c1fae0e9e842888369391c776 # v46.3.10
        with:
          configurationFile: .github/renovate-config.json
          renovate-image: ghcr.io/renovatebot/renovate:44.179.1@sha256:f30ad41a40ab226734469eb61517d527a198a48b173cf6e0c3ee180927479293
          token: ${{ secrets.RENOVATE_TOKEN }}
EOF
}

# The known-good repo config (DUA-3, DUA-4, DUA-7 to DUA-9; task 2.2's shape). packageRules
# indexes the mutants use: 1 github actions, 2 the Renovate image, 3 docker images, 4 the digest
# automerge rule, 5 pinned build tools, 6 helm, 7 go toolchain (last group rule), 8 node/python
# (last rule). Checked with renovate-config-validator --strict --no-global (evidence 1.2).
write_fixture_config() {
  cat >"$1/renovate.json" <<'EOF'
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:recommended"],
  "platformAutomerge": false,
  "automergeType": "pr",
  "automergeStrategy": "rebase",
  "rebaseWhen": "conflicted",
  "labels": ["dependencies"],
  "customManagers": [
    {
      "customType": "regex",
      "description": "Helm release versions in CI and install scripts",
      "managerFilePatterns": ["/^\\.github/(actions|workflows)/.+\\.ya?ml$/", "/^hack/install-helm\\.sh$/"],
      "matchStrings": ["HELM_VERSION[^\\n]*?(?<currentValue>v[0-9]+\\.[0-9]+\\.[0-9]+)"],
      "datasourceTemplate": "github-releases",
      "depNameTemplate": "helm/helm"
    }
  ],
  "packageRules": [
    {
      "matchManagers": ["gomod"],
      "groupName": "go minor and patch dependencies",
      "matchUpdateTypes": ["minor", "patch"]
    },
    {
      "matchManagers": ["github-actions"],
      "groupName": "github actions",
      "pinDigests": true
    },
    {
      "description": "The Renovate image alone, weekly, never automerged (DUA-3).",
      "matchPackageNames": ["ghcr.io/renovatebot/renovate"],
      "groupName": "renovate image",
      "schedule": ["* * * * 1"],
      "updateNotScheduled": false,
      "automerge": false
    },
    {
      "matchManagers": ["dockerfile"],
      "groupName": "docker images",
      "pinDigests": true
    },
    {
      "description": "Only Docker image digests auto-merge (DUA-7).",
      "matchManagers": ["devcontainer", "dockerfile"],
      "matchUpdateTypes": ["digest"],
      "automerge": true,
      "rebaseWhen": "behind-base-branch"
    },
    {
      "matchManagers": ["custom.regex"],
      "groupName": "pinned build tools"
    },
    {
      "description": "Every Helm pin in one group (DUA-9).",
      "matchPackageNames": ["helm/helm"],
      "groupName": "helm"
    },
    {
      "matchDepNames": ["go", "golang"],
      "groupName": "go toolchain"
    },
    {
      "matchManagers": ["github-actions"],
      "matchDepTypes": ["uses-with"],
      "matchDepNames": ["node", "python"],
      "matchPackageNames": ["actions/node-versions", "actions/python-versions"],
      "enabled": false
    }
  ]
}
EOF
  cat >"$1/.github/renovate-config.json" <<'EOF'
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "onboarding": false,
  "requireConfig": "required"
}
EOF
}

# A validator that keeps T16's contract (evidence 0.5's argv). Each line a mutant edits carries
# a marker comment.
write_fixture_validator() {
  mkdir -p "$(dirname "$1/${VALIDATOR_REL}")"
  cat >"$1/${VALIDATOR_REL}" <<'EOF'
#!/usr/bin/env bash
# Validate the Renovate configs with the pinned renovate-config-validator (self-test fixture).
set -euo pipefail
root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
image="$(yq eval '.jobs[].steps[] | select((.uses // "") | test("^renovatebot/github-action@")) | .with["renovate-image"] // ""' "${root}/.github/workflows/renovate.yaml")"
if [[ ! "${image}" =~ ^ghcr\.io/renovatebot/renovate:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "renovate-config-validate: renovate-image '${image}' has no digest" >&2
  exit 1 # digest-check
fi
validate() {
  docker run --rm --network none -v "${root}:/repo:ro" -w /repo --entrypoint renovate-config-validator "${image}" "$@" # docker-call
}
rc=0
validate --strict --no-global renovate.json || rc=1 # repo-call
validate --strict .github/renovate-config.json || rc=1 # global-call
exit "${rc}" # exit-line
EOF
  chmod +x "$1/${VALIDATOR_REL}"
}

# A Renovate section that names every T17 item, one item per line so a mutant drops one.
write_fixture_docs() {
  mkdir -p "$(dirname "$1/${DOCS_REL}")"
  cat >"$1/${DOCS_REL}" <<'EOF'
# Tooling setup (self-test fixture)

## Renovate dependency updates

Renovate runs hourly from its workflow, one run at a time.
It authenticates with the `RENOVATE_TOKEN` repository secret, owned by the maintainer.
Pending, rate-limited and errored updates are listed on the Dependency Dashboard issue.
Only digest-only updates of Dockerfile and devcontainer images auto-merge, after every check.
Pause the bot with `gh workflow disable renovate.yaml`.
Rebase a stuck PR with `gh pr update-branch --rebase <number>`.
Validate the config with `bash hack/ci/renovate-config-validate.sh`.
The hosted Renovate App is unused.
Before approving a release, the maintainer reviews the bot merges since the last tag.

## Further reading

Nothing here.
EOF
}

# Paths of the tree a fixture copies; renovate.yaml, the two config files, the validator script
# and the docs page are then written by the fixture writers above, never copied.
FIXTURE_PATHS=(.github)

build_fixture() { # $1: src root, $2: destination
  local p
  mkdir -p "$2"
  for p in "${FIXTURE_PATHS[@]}"; do
    [[ -e "$1/${p}" ]] || continue
    mkdir -p "$(dirname "$2/${p}")"
    cp -R "$1/${p}" "$2/${p}"
  done
  write_fixture_workflow "$2/.github/workflows/renovate.yaml"
  write_fixture_config "$2"
  write_fixture_validator "$2"
  write_fixture_docs "$2"
  VR="${VALIDATE_RUN}" yq eval -i 'del(.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate"))) | .jobs.lint.steps += [{"name": "Validate the Renovate config (DUA-5)", "run": strenv(VR)}]' "$2/.github/workflows/ci.yaml"
}

FX=""   # the fixture copy a mutant edits
RSF='.jobs.renovate.steps[2]' # the Renovate step in the fixture workflow
WF=".github/workflows/renovate.yaml"
CIF=".github/workflows/ci.yaml"

die() {
  echo "FAIL: self-test: $*" >&2
  exit 1
}

# ymut <file> <yq expression> <readback expression that must print true>; values come in
# through strenv(V), never through the expression text.
ymut() {
  yq eval -i "$2" "${FX}/$1"
  [[ "$(yq eval "$3" "${FX}/$1")" == "true" ]] || die "mutant did not apply: $2"
}

# A new workflow file for the T9 mutants and decoys.
new_workflow() { # $1 name, $2 body (verbatim)
  printf '%s\n' "$2" >"${FX}/.github/workflows/$1"
}

# --- mutants: one function per mutant, named after its clause -----------------------------

m_file_missing() { rm -f "${FX}/${WF}"; [[ ! -e "${FX}/${WF}" ]] || die "renovate.yaml still there"; }
m_t1_fallback() { V='${{ secrets.RENOVATE_TOKEN != '"''"' && secrets.RENOVATE_TOKEN || github.token }}' ymut "${WF}" "${RSF}.with.token = strenv(V)" "${RSF}.with.token | test(\"github.token\")"; }
m_t1_github_token_secret() { V='${{ secrets.GITHUB_TOKEN }}' ymut "${WF}" "${RSF}.with.token = strenv(V)" "${RSF}.with.token == strenv(V)"; }
m_t1_token_elsewhere() { V='${{ github.token }}' ymut "${WF}" '.jobs.renovate.env.GH_TOKEN = strenv(V)' '.jobs.renovate.env.GH_TOKEN == strenv(V)'; }
m_t2_no_check() { ymut "${WF}" 'del(.jobs.renovate.steps[0])' '.jobs.renovate.steps | length == 2'; }
m_t2_after_renovate() { ymut "${WF}" '.jobs.renovate.steps = [.jobs.renovate.steps[1], .jobs.renovate.steps[2], .jobs.renovate.steps[0]]' '.jobs.renovate.steps[2].name == "Require RENOVATE_TOKEN"'; }
m_t2_if() { ymut "${WF}" '.jobs.renovate.steps[0].if = "github.event_name == '"'"'schedule'"'"'"' '.jobs.renovate.steps[0] | has("if")'; }
m_t2_continue() { ymut "${WF}" '.jobs.renovate.steps[0]["continue-on-error"] = true' '.jobs.renovate.steps[0]["continue-on-error"] == true'; }
m_t2_exits_zero() { V='echo "RENOVATE_TOKEN may be empty"' ymut "${WF}" '.jobs.renovate.steps[0].run = strenv(V)' '.jobs.renovate.steps[0].run == strenv(V)'; }
m_t2_silent() { V='test -n "${RENOVATE_TOKEN}"' ymut "${WF}" '.jobs.renovate.steps[0].run = strenv(V)' '.jobs.renovate.steps[0].run == strenv(V)'; }
m_t2_always_fails() { V='echo "RENOVATE_TOKEN check" >&2; exit 1' ymut "${WF}" '.jobs.renovate.steps[0].run = strenv(V)' '.jobs.renovate.steps[0].run == strenv(V)'; }
# Under the runner's unspecified shell (bash -e, no pipefail) this body exits 0 on an empty token.
m_t2_pipe_masks() { V='echo "checking RENOVATE_TOKEN" >&2; test -n "${RENOVATE_TOKEN}" | cat' ymut "${WF}" '.jobs.renovate.steps[0].run = strenv(V)' '.jobs.renovate.steps[0].run == strenv(V)'; }
m_t2_shell() { ymut "${WF}" '.jobs.renovate.steps[0].shell = "true {0}"' '.jobs.renovate.steps[0].shell == "true {0}"'; }
m_t2_other_secret() { V='${{ secrets.OTHER_TOKEN }}' ymut "${WF}" '.jobs.renovate.steps[0].env.RENOVATE_TOKEN = strenv(V)' '.jobs.renovate.steps[0].env.RENOVATE_TOKEN == strenv(V)'; }
m_t3_write_all() { ymut "${WF}" '.permissions = "write-all"' '.permissions == "write-all"'; }
m_t3_job_write() { ymut "${WF}" '.jobs.renovate.permissions = {"contents": "write", "pull-requests": "write"}' '.jobs.renovate.permissions.contents == "write"'; }
m_t3_job_read_all() { ymut "${WF}" '.jobs.renovate.permissions = "read-all"' '.jobs.renovate.permissions == "read-all"'; }
m_t3_absent() { ymut "${WF}" 'del(.permissions) | del(.jobs.renovate.permissions)' '(has("permissions") or (.jobs.renovate | has("permissions"))) | not'; }
m_t4_weekly() { ymut "${WF}" '.["on"].schedule[0].cron = "0 4 * * 1"' '.["on"].schedule[0].cron == "0 4 * * 1"'; }
m_t4_month() { ymut "${WF}" '.["on"].schedule[0].cron = "0 4 * 1 *"' '.["on"].schedule[0].cron == "0 4 * 1 *"'; }
m_t4_every_minute() { ymut "${WF}" '.["on"].schedule[0].cron = "* * * * *"' '.["on"].schedule[0].cron == "* * * * *"'; }
m_t4_push() { ymut "${WF}" '.["on"].push = {"branches": ["main"]}' '.["on"] | has("push")'; }
m_t4_no_dispatch() { ymut "${WF}" 'del(.["on"].workflow_dispatch)' '.["on"] | has("workflow_dispatch") | not'; }
m_t5_per_run() { V='renovate-${{ github.run_id }}' ymut "${WF}" '.concurrency.group = strenv(V)' '.concurrency.group == strenv(V)'; }
m_t5_cancel() { ymut "${WF}" '.concurrency["cancel-in-progress"] = true' '.concurrency["cancel-in-progress"] == true'; }
m_t5_cancel_string() { ymut "${WF}" '.concurrency["cancel-in-progress"] = "false"' '.concurrency["cancel-in-progress"] | tag == "!!str"'; }
m_t5_absent() { ymut "${WF}" 'del(.concurrency)' 'has("concurrency") | not'; }
m_t6_tag_ref() { ymut "${WF}" "${RSF}.uses = \"renovatebot/github-action@v46.3.10\"" "${RSF}.uses == \"renovatebot/github-action@v46.3.10\""; }
m_t6_old_comment() { ymut "${WF}" "${RSF}.uses line_comment = \"v0.0.1\"" "${RSF}.uses | line_comment == \"v0.0.1\""; }
m_t6_no_comment() { ymut "${WF}" "${RSF}.uses line_comment = \"\"" "${RSF}.uses | line_comment == \"\""; }
m_t6_floating() { ymut "${WF}" "${RSF}.with[\"renovate-image\"] = \"ghcr.io/renovatebot/renovate:latest\"" "${RSF}.with[\"renovate-image\"] == \"ghcr.io/renovatebot/renovate:latest\""; }
m_t6_no_digest() { ymut "${WF}" "${RSF}.with[\"renovate-image\"] |= sub(\"@sha256:.*\", \"\")" "${RSF}.with[\"renovate-image\"] | test(\"@\") | not"; }
m_t6_other_config() { ymut "${WF}" "${RSF}.with.configurationFile = \".github/other-config.json\"" "${RSF}.with.configurationFile == \".github/other-config.json\""; }
m_t6_version_input() { ymut "${WF}" "${RSF}.with[\"renovate-version\"] = \"99.0.0\"" "${RSF}.with | has(\"renovate-version\")"; }
m_t6_image_in_name() { ymut "${WF}" "${RSF}.name = ${RSF}.with[\"renovate-image\"] | del(${RSF}.with[\"renovate-image\"])" "(${RSF}.name | test(\"@sha256:\")) and (${RSF}.with | has(\"renovate-image\") | not)"; }
m_t7_force_job() { V='{"schedule":null}' ymut "${WF}" '.jobs.renovate.env.RENOVATE_FORCE = strenv(V)' '.jobs.renovate.env | has("RENOVATE_FORCE")'; }
m_t7_workflow_env() { ymut "${WF}" '.env.RENOVATE_PLATFORM_AUTOMERGE = "true"' '.env | has("RENOVATE_PLATFORM_AUTOMERGE")'; }
m_t7_step_token() { V='${{ secrets.RENOVATE_TOKEN }}' ymut "${WF}" "${RSF}.env.RENOVATE_TOKEN = strenv(V)" "${RSF}.env | has(\"RENOVATE_TOKEN\")"; }
m_t7_other_repo() { ymut "${WF}" '.jobs.renovate.env.RENOVATE_REPOSITORIES = "other-org/other-repo"' '.jobs.renovate.env.RENOVATE_REPOSITORIES == "other-org/other-repo"'; }
m_t7_second_token_step() { V='${{ secrets.RENOVATE_TOKEN }}' B='echo "using RENOVATE_TOKEN"' ymut "${WF}" '.jobs.renovate.steps = [.jobs.renovate.steps[0], {"name": "extra", "env": {"RENOVATE_TOKEN": strenv(V)}, "run": strenv(B)}, .jobs.renovate.steps[1], .jobs.renovate.steps[2]]' '.jobs.renovate.steps[1].env | has("RENOVATE_TOKEN")'; }
m_t7_token_after() { V='${{ secrets.RENOVATE_TOKEN }}' ymut "${WF}" '.jobs.renovate.steps += [{"name": "after", "env": {"RENOVATE_TOKEN": strenv(V)}, "run": "echo done"}]' '.jobs.renovate.steps[3].env | has("RENOVATE_TOKEN")'; }
m_t7_github_env() { V='echo "RENOVATE_FORCE={}" >> "$GITHUB_ENV"' ymut "${WF}" '.jobs.renovate.steps = [.jobs.renovate.steps[0], {"name": "export", "run": strenv(V)}, .jobs.renovate.steps[1], .jobs.renovate.steps[2]]' '.jobs.renovate.steps[1].run | test("GITHUB_ENV")'; }
m_t7_env_expression() { V='${{ fromJSON(vars.RENOVATE_ENV) }}' ymut "${WF}" '.jobs.renovate.env = strenv(V)' '.jobs.renovate.env | tag == "!!str"'; }
m_t8_ci_missing() { rm -f "${FX}/${CIF}"; [[ ! -e "${FX}/${CIF}" ]] || die "ci.yaml still there"; }
m_t8_unwired() { ymut "${CIF}" 'del(.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate")))' '[.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate"))] | length == 0'; }
m_t8_if() { ymut "${CIF}" '(.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate"))).if = "false"' '[.jobs.lint.steps[] | select(has("if") and ((.run // "") | test("renovate-config-validate")))] | length == 1'; }
m_t8_continue() { ymut "${CIF}" '(.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate")))["continue-on-error"] = true' '[.jobs.lint.steps[] | select(has("continue-on-error") and ((.run // "") | test("renovate-config-validate")))] | length == 1'; }
m_t8_or_true() { ymut "${CIF}" '(.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate"))).run |= . + " || true"' '[.jobs.lint.steps[] | select((.run // "") | test("validate.sh [|][|] true"))] | length == 1'; }
m_t8_other_job() { VR="${VALIDATE_RUN}" ymut "${CIF}" 'del(.jobs.lint.steps[] | select((.run // "") | test("renovate-config-validate"))) | .jobs["tool-pins-selftest"].steps += [{"name": "validate", "run": strenv(VR)}]' '[.jobs["tool-pins-selftest"].steps[] | select((.run // "") | test("renovate-config-validate"))] | length == 1'; }
m_t9_gh_merge() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: gh pr merge "$PR" --rebase'
}
m_t9_script_merge() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/github-script@0000000000000000000000000000000000000000 # v0.0.1
        with:
          script: |
            await github.rest.pulls.merge({owner, repo, pull_number: 1})'
}
m_t9_api_merge() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: gh api -X PUT "repos/o/r/pulls/42/merge"'
}
m_t9_automerge() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: >-
          gh api graphql -f query="mutation { enablePullRequestAutoMerge(input: {}) { clientMutationId } }"'
}
m_t9_graphql_merge() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: >-
          gh api graphql -f query="mutation { mergePullRequest(input: {}) { clientMutationId } }"'
}
# A workflow yq cannot read must red, not pass with no bodies seen.
m_t9_unreadable() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    steps:
      - run: echo a: b'
}
m_t9_action_merge() {
  mkdir -p "${FX}/.github/actions/zz-merge"
  printf '%s\n' 'name: zz' 'runs:' '  using: composite' '  steps:' '    - shell: bash' '      run: gh pr merge "$PR" --rebase' >"${FX}/.github/actions/zz-merge/action.yml"
}
m_t9_with_seq() {
  new_workflow zz-merge.yaml 'name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/github-script@0000000000000000000000000000000000000000 # v0.0.1
        with: [1, 2]
      - run: gh pr merge "$PR" --rebase'
}
m_t9_prt_map() {
  new_workflow zz-prt.yaml 'name: zz
on:
  pull_request_target:
    types: [opened]
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi'
}
m_t9_prt_list() {
  new_workflow zz-prt.yaml 'name: zz
on: [push, pull_request_target]
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi'
}
m_t9_prt_str() {
  new_workflow zz-prt.yaml 'name: zz
on: pull_request_target
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi'
}

# --- mutants T10 to T17 ---------------------------------------------------------------------

RJF="renovate.json"
GCF=".github/renovate-config.json"
VSF="hack/ci/renovate-config-validate.sh"
DCF="docs/development/tooling-setup.md"

# jmut <file> <yq expression> <readback expression that must print true>, on a JSON file.
jmut() {
  yq -p json -o json -i "$2" "${FX}/$1"
  [[ "$(yq -p json -oy "$3" "${FX}/$1")" == "true" ]] || die "mutant did not apply: $2"
}

# smut <file> <sed expression> <ERE the result must contain>: a line edit of a text file.
smut() {
  sed -i -e "$2" "${FX}/$1"
  grep -E -- "$3" "${FX}/$1" >/dev/null || die "mutant did not apply: $2"
}

# dmut <ERE>: drop every line of the docs fixture matching it (case-insensitive).
dmut() {
  grep -Eiv -- "$1" "${FX}/${DCF}" >"${FX}/${DCF}.new" || true
  mv "${FX}/${DCF}.new" "${FX}/${DCF}"
  ! grep -Ei -- "$1" "${FX}/${DCF}" >/dev/null || die "docs mutant did not apply: $1"
}

m_rj_missing() { rm -f "${FX}/${RJF}"; }
m_gc_missing() { rm -f "${FX}/${GCF}"; }
m_rj_unparsable() { printf '{ "extends": [\n' >"${FX}/${RJF}"; }
m_t10_disable_dashboard() { jmut "${RJF}" '.extends += [":disableDependencyDashboard"]' '.extends | length == 2'; }
m_t10_remote() { jmut "${RJF}" '.extends = ["github>other-org/presets"]' '.extends[0] == "github>other-org/presets"'; }
m_t10_string() { jmut "${RJF}" '.extends = "config:recommended"' '.extends | tag == "!!str"'; }
m_t10_absent() { jmut "${RJF}" 'del(.extends)' 'has("extends") | not'; }
m_t10_nested() { jmut "${RJF}" '.packageRules[1].extends = [":automergeAll"]' '.packageRules[1] | has("extends")'; }
m_t10_dashboard_false() { jmut "${RJF}" '.dependencyDashboard = false' '.dependencyDashboard == false'; }
m_t10_global_extends() { jmut "${GCF}" '.extends = [":automergeAll"]' 'has("extends")'; }
m_t11_top() { jmut "${RJF}" '.automerge = true' '.automerge == true'; }
m_t11_patch() { jmut "${RJF}" '.patch = {"automerge": true}' '.patch.automerge == true'; }
m_t11_vulnerability() { jmut "${RJF}" '.vulnerabilityAlerts = {"automerge": true}' '.vulnerabilityAlerts.automerge == true'; }
m_t11_lockfile() { jmut "${RJF}" '.lockFileMaintenance = {"enabled": true, "automerge": true}' '.lockFileMaintenance.automerge == true'; }
m_t11_manager_block() { jmut "${RJF}" '.dockerfile = {"automerge": true}' '.dockerfile.automerge == true'; }
m_t11_second_rule() { jmut "${RJF}" '.packageRules[1].automerge = true' '.packageRules[1].automerge == true'; }
m_t11_global() { jmut "${GCF}" '.automerge = true' '.automerge == true'; }
m_t11_no_rule() { jmut "${RJF}" 'del(.packageRules[4])' '[.packageRules[] | select(.automerge == true)] | length == 0'; }
m_t11_extra_manager() { jmut "${RJF}" '.packageRules[4].matchManagers += ["github-actions"]' '.packageRules[4].matchManagers | length == 3'; }
m_t11_extra_type() { jmut "${RJF}" '.packageRules[4].matchUpdateTypes += ["patch"]' '.packageRules[4].matchUpdateTypes | length == 2'; }
m_t11_extra_key() { jmut "${RJF}" '.packageRules[4].matchPackageNames = ["golang"]' '.packageRules[4] | has("matchPackageNames")'; }
m_t11_rebase() { jmut "${RJF}" '.packageRules[4].rebaseWhen = "auto"' '.packageRules[4].rebaseWhen == "auto"'; }
m_t11_rebase_absent() { jmut "${RJF}" 'del(.packageRules[4].rebaseWhen)' '.packageRules[4] | has("rebaseWhen") | not'; }
m_t12_platform_true() { jmut "${RJF}" '.platformAutomerge = true' '.platformAutomerge == true'; }
m_t12_platform_absent() { jmut "${RJF}" 'del(.platformAutomerge)' 'has("platformAutomerge") | not'; }
m_t12_strategy() { jmut "${RJF}" '.automergeStrategy = "squash"' '.automergeStrategy == "squash"'; }
m_t12_type() { jmut "${RJF}" '.automergeType = "branch"' '.automergeType == "branch"'; }
m_t12_rebase_top() { jmut "${RJF}" '.rebaseWhen = "auto"' '.rebaseWhen == "auto"'; }
m_t12_ignore_tests() { jmut "${RJF}" '.ignoreTests = true' 'has("ignoreTests")'; }
m_t12_ignore_rule() { jmut "${RJF}" '.packageRules[3].ignoreTests = true' '.packageRules[3] | has("ignoreTests")'; }
m_t12_rule_type() { jmut "${RJF}" '.packageRules[1].automergeType = "branch"' '.packageRules[1] | has("automergeType")'; }
m_t12_rule_rebase() { jmut "${RJF}" '.packageRules[3].rebaseWhen = "behind-base-branch"' '.packageRules[3] | has("rebaseWhen")'; }
m_t12_block() { jmut "${RJF}" '.dockerfile = {"automergeStrategy": "squash"}' '.dockerfile | has("automergeStrategy")'; }
m_t12_global() { jmut "${GCF}" '.platformAutomerge = true' 'has("platformAutomerge")'; }
m_t13_extra_key() { jmut "${GCF}" '.autodiscover = true' 'has("autodiscover")'; }
m_t13_onboarding() { jmut "${GCF}" '.onboarding = true' '.onboarding == true'; }
m_t13_require() { jmut "${GCF}" '.requireConfig = "optional"' '.requireConfig == "optional"'; }
m_t13_require_absent() { jmut "${GCF}" 'del(.requireConfig)' 'has("requireConfig") | not'; }
m_t14_absent() { jmut "${RJF}" 'del(.packageRules[2])' '[.packageRules[] | select(.groupName == "renovate image")] | length == 0'; }
m_t14_before_actions() { jmut "${RJF}" '.packageRules = [.packageRules[0], .packageRules[2], .packageRules[1]] + .packageRules[3:]' '.packageRules[2].groupName == "github actions"'; }
m_t14_hours() { jmut "${RJF}" '.packageRules[2].schedule = ["* 0-3 * * 1"]' '.packageRules[2].schedule[0] == "* 0-3 * * 1"'; }
m_t14_two_days() { jmut "${RJF}" '.packageRules[2].schedule += ["* * * * 4"]' '.packageRules[2].schedule | length == 2'; }
m_t14_not_scheduled() { jmut "${RJF}" '.packageRules[2].updateNotScheduled = true' '.packageRules[2].updateNotScheduled == true'; }
m_t14_automerge_absent() { jmut "${RJF}" 'del(.packageRules[2].automerge)' '.packageRules[2] | has("automerge") | not'; }
m_t14_group_absent() { jmut "${RJF}" 'del(.packageRules[2].groupName)' '.packageRules[2] | has("groupName") | not'; }
m_t14_actions_group() { jmut "${RJF}" '.packageRules[2].groupName = "github actions"' '.packageRules[2].groupName == "github actions"'; }
m_t14_shared_group() { jmut "${RJF}" '.packageRules[2].groupName = "docker images"' '.packageRules[2].groupName == "docker images"'; }
m_t14_extra_name() { jmut "${RJF}" '.packageRules[2].matchPackageNames += ["ghcr.io/other-org/image"]' '.packageRules[2].matchPackageNames | length == 2'; }
m_t15_dep_names() { jmut "${RJF}" '.packageRules[6].matchDepNames = .packageRules[6].matchPackageNames | del(.packageRules[6].matchPackageNames)' '.packageRules[6] | has("matchPackageNames") | not'; }
m_t15_extra_dep() { jmut "${RJF}" '.packageRules[5].matchDepNames = ["helm/helm"]' '.packageRules[5] | has("matchDepNames")'; }
m_t15_before_tools() { jmut "${RJF}" '.packageRules = .packageRules[0:5] + [.packageRules[6], .packageRules[5]] + .packageRules[7:]' '.packageRules[5].groupName == "helm"'; }
m_t15_before_actions() { jmut "${RJF}" '.packageRules = [.packageRules[6]] + .packageRules[0:6] + .packageRules[7:]' '.packageRules[0].groupName == "helm"'; }
m_t15_group() { jmut "${RJF}" '.packageRules[6].groupName = "helm tools"' '.packageRules[6].groupName == "helm tools"'; }
m_t16_missing() { rm -f "${FX}/${VSF}"; }
m_t16_no_digest_check() { smut "${VSF}" '/# digest-check$/d' 'docker-call'; ! grep -F 'digest-check' "${FX}/${VSF}" >/dev/null || die "digest check still there"; }
m_t16_no_repo_call() { smut "${VSF}" '/# repo-call$/d' 'global-call'; ! grep -F 'repo-call' "${FX}/${VSF}" >/dev/null || die "repo call still there"; }
m_t16_global_no_global() { smut "${VSF}" 's/validate --strict \.github/validate --strict --no-global .github/' 'validate --strict --no-global \.github'; }
m_t16_repo_no_strict() { smut "${VSF}" 's/validate --strict --no-global renovate/validate --no-global renovate/' 'validate --no-global renovate\.json'; }
m_t16_no_rm() { smut "${VSF}" 's/docker run --rm /docker run /' 'docker run --network'; }
m_t16_hardcoded_image() { smut "${VSF}" 's|"\${image}" "\$@"|ghcr.io/renovatebot/renovate:latest "$@"|' 'renovate:latest "\$@"'; }
m_t16_wrong_command() { smut "${VSF}" 's/--entrypoint renovate-config-validator/--entrypoint renovate/' '--entrypoint renovate "'; }
m_t16_ignore_rc() { smut "${VSF}" 's/|| rc=1/|| true/' 'renovate\.json \|\| true # repo-call'; }
m_t16_ignore_second_rc() { smut "${VSF}" '/# global-call$/s/|| rc=1/|| true/' 'config\.json \|\| true # global-call'; }
m_t16_always_fails() { smut "${VSF}" 's/^exit "\${rc}" # exit-line$/exit 1 # exit-line/' '^exit 1 # exit-line'; }
m_t17_missing() { rm -f "${FX}/${DCF}"; }
m_t17_no_section() { smut "${DCF}" 's/^## Renovate dependency updates$/## Dependency updates/' '^## Dependency updates$'; }
m_t17_hourly() { dmut 'hourly'; }
m_t17_token() { dmut 'RENOVATE_TOKEN'; }
m_t17_dashboard() { dmut 'dependency dashboard'; }
m_t17_scope() { dmut 'auto-?merge'; }
m_t17_disable() { dmut 'gh workflow disable'; }
m_t17_rebase() { dmut 'gh pr update-branch'; }
m_t17_validate() { dmut 'renovate-config-validate\.sh'; }
m_t17_app() { dmut 'hosted Renovate App'; }
m_t17_release() { dmut 'since the last tag'; }
# The right text in the wrong section: it moves below the next heading.
m_t17_moved() {
  local line
  line="$(grep -F 'gh workflow disable' "${FX}/${DCF}")"
  dmut 'gh workflow disable'
  printf '%s\n' "${line}" >>"${FX}/${DCF}"
}

# --- controls: must stay green -------------------------------------------------------------

c_noop() { :; }
# A grouped bot PR bumps the action (SHA + comment) and the image (version + digest) together.
c_bot_bump() {
  V="renovatebot/github-action@$(printf 'b%.0s' {1..40})" ymut "${WF}" "${RSF}.uses = strenv(V) | ${RSF}.uses line_comment = \"v99.0.0\"" "${RSF}.uses | line_comment == \"v99.0.0\""
  V="ghcr.io/renovatebot/renovate:99.0.0@sha256:$(printf 'c%.0s' {1..64})" ymut "${WF}" "${RSF}.with[\"renovate-image\"] = strenv(V)" "${RSF}.with[\"renovate-image\"] == strenv(V)"
}
# YAML comments that name the refused strings are not parsed values.
c_comment_decoy() {
  new_workflow zz-comment.yaml '# never gh pr merge here, never pull_request_target, never pulls.merge
# github.token GITHUB_ENV RENOVATE_FORCE
name: zz
on: workflow_dispatch
jobs:
  m:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi # gh pr merge'
  printf '# github.token secrets.GITHUB_TOKEN RENOVATE_FORCE GITHUB_ENV\n' >>"${FX}/${WF}"
}
# An explicit `shell: bash` keeps the fixture's body correct.
c_shell_bash() { ymut "${WF}" '.jobs.renovate.steps[0].shell = "bash"' '.jobs.renovate.steps[0].shell == "bash"'; }
# An hourly cron with an */N hour, and a second, weekly entry, still fire every day.
c_hour_step() { ymut "${WF}" '.["on"].schedule = [{"cron": "5 */6 * * *"}, {"cron": "0 4 * * 1"}]' '.["on"].schedule | length == 2'; }

# The automerge rule's description is optional (T11).
c_no_description() { jmut "${RJF}" 'del(.packageRules[4].description)' '.packageRules[4] | has("description") | not'; }
# automerge: false anywhere is not an automerge (T11), and the image rule already sets it.
c_automerge_false() { jmut "${RJF}" '.automerge = false | .packageRules[1].automerge = false' '.packageRules[1].automerge == false'; }
c_dashboard_true() { jmut "${RJF}" '.dependencyDashboard = true' '.dependencyDashboard == true'; }
# Any one whole weekday is a window every hourly run overlaps (T14).
c_image_other_day() { jmut "${RJF}" '.packageRules[2].schedule = ["* * * * 3"]' '.packageRules[2].schedule[0] == "* * * * 3"'; }
# A "## " line inside a fenced block does not end the Renovate section (T17).
c_docs_fence() { smut "${DCF}" '/^## Renovate dependency updates$/a ```sh\n## a shell comment, not a heading\n```' '^## a shell comment'; }
# A file argument may carry the mount prefix (T16, evidence 0.5 run F).
c_validator_mount_prefix() { smut "${VSF}" 's| renovate\.json | /repo/renovate.json |; s| \.github/renovate-config\.json | /repo/.github/renovate-config.json |' '/repo/\.github/renovate-config\.json'; }
# The validator through the image's default entrypoint (evidence 0.5 run C).
c_validator_default_entrypoint() { smut "${VSF}" 's|--entrypoint renovate-config-validator "\${image}"|"${image}" renovate-config-validator|' '"\$\{image\}" renovate-config-validator'; }

# clause | mutant function | expected message (ERE, matched against that clause's FAIL lines).
# "T2+T7" judges the mutant by T2's message and also allows T7 to red: moving the token check
# after the Renovate step breaks both clauses by definition (T7 allows the secret only there).
MUTANTS=(
  "FILE|m_file_missing|renovate\.yaml is missing"
  "T1|m_t1_fallback|token is .*expected exactly"
  "T1|m_t1_github_token_secret|token is .*expected exactly"
  "T1|m_t1_token_elsewhere|names the repository token"
  "T2|m_t2_no_check|no step before the Renovate step sets env\.RENOVATE_TOKEN"
  "T2+T7|m_t2_after_renovate|no step before the Renovate step sets env\.RENOVATE_TOKEN"
  "T2|m_t2_other_secret|no step before the Renovate step sets env\.RENOVATE_TOKEN"
  "T2|m_t2_if|has an if:"
  "T2|m_t2_continue|has continue-on-error:"
  "T2|m_t2_shell|runs under shell"
  "T2|m_t2_pipe_masks|exits 0 when RENOVATE_TOKEN is empty"
  "T2|m_t2_exits_zero|exits 0 when RENOVATE_TOKEN is empty"
  "T2|m_t2_silent|without naming RENOVATE_TOKEN"
  "T2|m_t2_always_fails|when RENOVATE_TOKEN is set"
  "T3|m_t3_write_all|workflow permissions is the string 'write-all'"
  "T3|m_t3_job_write|job 'renovate': its effective permissions grants"
  "T3|m_t3_job_read_all|job 'renovate': its effective permissions is the string 'read-all'"
  "T3|m_t3_absent|effective permissions is absent"
  "T4|m_t4_weekly|no cron entry fires every day"
  "T4|m_t4_month|no cron entry fires every day"
  "T4|m_t4_every_minute|no cron entry fires every day"
  "T4|m_t4_push|expected exactly schedule and workflow_dispatch"
  "T4|m_t4_no_dispatch|expected exactly schedule and workflow_dispatch"
  "T5|m_t5_per_run|is not the literal 'renovate'"
  "T5|m_t5_absent|is not the literal 'renovate'"
  "T5|m_t5_cancel|is not the boolean false"
  "T5|m_t5_cancel_string|is not the boolean false"
  "T6|m_t6_tag_ref|expected renovatebot/github-action@<40-hex"
  "T6|m_t6_old_comment|version comment is '# v0\.0\.1'"
  "T6|m_t6_no_comment|version comment is '# '"
  "T6|m_t6_floating|renovate-image is 'ghcr\.io/renovatebot/renovate:latest'"
  "T6|m_t6_no_digest|renovate-image is 'ghcr\.io/renovatebot/renovate:[0-9.]+'"
  "T6|m_t6_image_in_name|renovate-image is ''"
  "T6|m_t6_other_config|configurationFile is '\.github/other-config\.json'"
  "T6|m_t6_version_input|has 'renovate-version' besides"
  "T7|m_t7_force_job|'jobs\.renovate\.env\.RENOVATE_FORCE' sets a RENOVATE_"
  "T7|m_t7_workflow_env|'env\.RENOVATE_PLATFORM_AUTOMERGE' sets a RENOVATE_"
  "T7|m_t7_step_token|'jobs\.renovate\.steps\.2\.env\.RENOVATE_TOKEN' sets a RENOVATE_"
  "T7|m_t7_other_repo|RENOVATE_REPOSITORIES' is .other-org/other-repo."
  "T7|m_t7_second_token_step|sets RENOVATE_TOKEN on a second step"
  "T7|m_t7_token_after|'jobs\.renovate\.steps\.3\.env\.RENOVATE_TOKEN' sets a RENOVATE_"
  "T7|m_t7_github_env|names GITHUB_ENV"
  "T7|m_t7_env_expression|is an env: that is not a mapping"
  "T8|m_t8_ci_missing|ci\.yaml is missing"
  "T8|m_t8_unwired|has no step whose run is exactly"
  "T8|m_t8_other_job|has no step whose run is exactly"
  "T8|m_t8_or_true|has no step whose run is exactly"
  "T8|m_t8_if|has an if: or continue-on-error:"
  "T8|m_t8_continue|has an if: or continue-on-error:"
  "T9|m_t9_gh_merge|zz-merge\.yaml merges pull requests"
  "T9|m_t9_script_merge|zz-merge\.yaml merges pull requests"
  "T9|m_t9_api_merge|zz-merge\.yaml merges pull requests"
  "T9|m_t9_automerge|zz-merge\.yaml merges pull requests"
  "T9|m_t9_graphql_merge|zz-merge\.yaml merges pull requests"
  "T9|m_t9_action_merge|actions/zz-merge/action\.yml merges pull requests"
  "T9|m_t9_with_seq|zz-merge\.yaml merges pull requests"
  "T9|m_t9_unreadable|zz-merge\.yaml cannot be read"
  "T9|m_t9_prt_map|zz-prt\.yaml triggers on pull_request_target"
  "T9|m_t9_prt_list|zz-prt\.yaml triggers on pull_request_target"
  "T9|m_t9_prt_str|zz-prt\.yaml triggers on pull_request_target"
  "FILE|m_rj_missing|renovate\.json is missing"
  "FILE|m_gc_missing|\.github/renovate-config\.json is missing"
  "FILE|m_rj_unparsable|renovate\.json does not parse as JSON"
  "T10|m_t10_disable_dashboard|top-level extends is \[\"config:recommended\",\":disableDependencyDashboard\"\]"
  "T10|m_t10_remote|top-level extends is \[\"github>other-org/presets\"\]"
  "T10|m_t10_string|top-level extends is \"config:recommended\""
  "T10|m_t10_absent|top-level extends is null"
  "T10|m_t10_nested|'renovate\.json:/packageRules\.1' has an extends key"
  "T10|m_t10_dashboard_false|'renovate\.json:/' sets dependencyDashboard"
  "T10+T13|m_t10_global_extends|'\.github/renovate-config\.json:/' has an extends key"
  "T11|m_t11_top|2 objects set automerge: true \(renovate\.json:/, "
  "T11|m_t11_patch|2 objects set automerge: true .*renovate\.json:/patch"
  "T11|m_t11_vulnerability|2 objects set automerge: true .*renovate\.json:/vulnerabilityAlerts"
  "T11|m_t11_lockfile|2 objects set automerge: true .*renovate\.json:/lockFileMaintenance"
  "T11|m_t11_manager_block|2 objects set automerge: true .*renovate\.json:/dockerfile"
  "T11|m_t11_second_rule|2 objects set automerge: true .*renovate\.json:/packageRules\.1"
  "T11+T13|m_t11_global|2 objects set automerge: true .*renovate-config\.json:/"
  "T11|m_t11_no_rule|0 objects set automerge: true"
  "T11|m_t11_extra_manager|matchManagers is \[\"devcontainer\",\"dockerfile\",\"github-actions\"\]"
  "T11|m_t11_extra_type|matchUpdateTypes is \[\"digest\",\"patch\"\]"
  "T11|m_t11_extra_key|has keys .*matchPackageNames"
  "T11|m_t11_rebase|rebaseWhen is \"auto\""
  "T11|m_t11_rebase_absent|automerge rule's rebaseWhen is null"
  "T12|m_t12_platform_true|top-level platformAutomerge is true"
  "T12|m_t12_platform_absent|top-level platformAutomerge is null"
  "T12|m_t12_strategy|top-level automergeStrategy is \"squash\""
  "T12|m_t12_type|top-level automergeType is \"branch\""
  "T12|m_t12_rebase_top|top-level rebaseWhen is \"auto\""
  "T12|m_t12_ignore_tests|'renovate\.json:/' has an ignoreTests key"
  "T12|m_t12_ignore_rule|'renovate\.json:/packageRules\.3' has an ignoreTests key"
  "T12|m_t12_rule_type|'renovate\.json:/packageRules\.1' sets automergeType"
  "T12|m_t12_rule_rebase|'renovate\.json:/packageRules\.3' sets rebaseWhen"
  "T12|m_t12_block|'renovate\.json:/dockerfile' sets automergeStrategy"
  "T12+T13|m_t12_global|'\.github/renovate-config\.json:/' sets platformAutomerge"
  "T13|m_t13_extra_key|has keys .*autodiscover"
  "T13|m_t13_onboarding|onboarding is true"
  "T13|m_t13_require|requireConfig is \"optional\""
  "T13|m_t13_require_absent|requireConfig is null"
  "T14|m_t14_absent|no packageRules entry has matchPackageNames exactly"
  "T14|m_t14_before_actions|it is not after the 'github actions' rule"
  "T14|m_t14_hours|its schedule is \[\"\* 0-3 \* \* 1\"\]"
  "T14|m_t14_two_days|its schedule is \[\"\* \* \* \* 1\",\"\* \* \* \* 4\"\]"
  "T14|m_t14_not_scheduled|its updateNotScheduled is true"
  "T14|m_t14_automerge_absent|its automerge is absent"
  "T14|m_t14_group_absent|its groupName is absent"
  "T14|m_t14_actions_group|its groupName is \"github actions\""
  "T14|m_t14_shared_group|its groupName is \"docker images\""
  "T14|m_t14_extra_name|no packageRules entry has matchPackageNames exactly"
  "T15|m_t15_dep_names|no rule has matchPackageNames \[\"helm/helm\"\] with groupName helm"
  "T15|m_t15_extra_dep|packageRules\[5\] still matches matchDepNames helm/helm"
  "T15|m_t15_before_tools|is not after both"
  "T15|m_t15_before_actions|is not after both"
  "T15|m_t15_group|no rule has matchPackageNames \[\"helm/helm\"\] with groupName helm"
  "T16|m_t16_missing|renovate-config-validate\.sh is missing"
  "T16|m_t16_no_digest_check|without a digest the script called docker"
  "T16|m_t16_no_repo_call|made 1 docker run call"
  "T16|m_t16_global_no_global|no docker run call validates \.github/renovate-config\.json"
  "T16|m_t16_repo_no_strict|no docker run call validates renovate\.json"
  "T16|m_t16_no_rm|has no --rm"
  "T16|m_t16_hardcoded_image|does not name the renovate-image"
  "T16|m_t16_wrong_command|runs 'renovate', not renovate-config-validator"
  "T16|m_t16_ignore_rc|exits 0 when docker run call 1 of 2 exits 1"
  "T16|m_t16_ignore_second_rc|exits 0 when docker run call 2 of 2 exits 1"
  "T16|m_t16_always_fails|exits 1 when both validator calls pass"
  "T17|m_t17_missing|tooling-setup\.md is missing"
  "T17|m_t17_no_section|has no '## \.\.\. Renovate"
  "T17|m_t17_hourly|does not name the hourly schedule"
  "T17|m_t17_token|does not name RENOVATE_TOKEN"
  "T17|m_t17_dashboard|does not name the dependency dashboard"
  "T17|m_t17_scope|does not name the digest-only auto-merge scope"
  "T17|m_t17_disable|does not name gh workflow disable"
  "T17|m_t17_rebase|does not name gh pr update-branch --rebase"
  "T17|m_t17_validate|does not name hack/ci/renovate-config-validate\.sh"
  "T17|m_t17_app|does not name the unused hosted App"
  "T17|m_t17_release|does not name the release review"
  "T17|m_t17_moved|does not name gh workflow disable"
)
CONTROLS=(c_noop c_bot_bump c_comment_decoy c_hour_step c_shell_bash
  c_no_description c_automerge_false c_dashboard_true c_image_other_day c_docs_fence
  c_validator_mount_prefix c_validator_default_entrypoint)

BASE="" # the known-good fixture every case copies

# One control or mutant in its own copy of BASE. It runs in a background subshell, so a die ends
# only this case; its output goes to a result file the parent prints in table order.
run_case() { # $1 copy dir, $2 control|mutant, $3 control function or mutant row
  local clause fn want lines other allowed out="$1.out"
  trap - EXIT
  FX="$1"
  cp -R "${BASE}" "${FX}"
  if [[ "$2" == "control" ]]; then
    fn="$3"
    "${fn}"
    if ! bash "${SELF}" "${FX}" >"${out}" 2>&1; then
      cat "${out}" >&2
      die "control ${fn} is red: the guard refuses a correct tree"
    fi
    echo "ok - control ${fn} stays green"
  else
    IFS='|' read -r clause fn want <<<"$3"
    allowed="^FAIL: (${clause//+/|}): "
    clause="${clause%%+*}"
    "${fn}"
    if bash "${SELF}" "${FX}" >"${out}" 2>&1; then
      die "mutant ${fn} (${clause}) stays green: the clause cannot fail"
    fi
    lines="$(grep '^FAIL: ' "${out}" || true)"
    if ! grep -E "^FAIL: ${clause}: .*${want}" <<<"${lines}" >/dev/null; then
      cat "${out}" >&2
      die "mutant ${fn} (${clause}) reds without its own message /${want}/"
    fi
    other="$(grep -Ev "${allowed}" <<<"${lines}" || true)"
    if [[ -n "${other}" ]]; then
      cat "${out}" >&2
      die "mutant ${fn} (${clause}) also trips another clause: ${other}"
    fi
    echo "ok - mutant ${fn} reds ${clause} with its own message"
  fi
  rm -rf "${FX:?}" "${out:?}"
}

self_test() {
  local work i fn row max failed=0 nmut=0
  local -a kinds=() args=()
  work="$(mktemp -d)"
  # shellcheck disable=SC2064 # expand now: the trap must remove this run's directory
  trap "rm -rf '${work:?}'" EXIT
  BASE="${work}/base"
  build_fixture "${ROOT}" "${BASE}"
  for fn in "${CONTROLS[@]}"; do
    kinds+=(control)
    args+=("${fn}")
  done
  for row in "${MUTANTS[@]}"; do
    kinds+=(mutant)
    args+=("${row}")
  done
  # The cases are independent copies, so they run in parallel (a full guard run per case).
  max="$(nproc 2>/dev/null || echo 2)"
  for ((i = 0; i < ${#args[@]}; i++)); do
    while (($(jobs -rp | wc -l) >= max)); do
      wait -n || true
    done
    (run_case "${work}/c${i}" "${kinds[i]}" "${args[i]}") >"${work}/r${i}" 2>&1 && touch "${work}/ok${i}" &
  done
  wait || true
  for ((i = 0; i < ${#args[@]}; i++)); do
    cat "${work}/r${i}"
    if [[ ! -f "${work}/ok${i}" ]]; then
      failed=$((failed + 1))
    elif [[ "${kinds[i]}" == "mutant" ]]; then
      nmut=$((nmut + 1))
    fi
  done
  ((failed == 0)) || die "${failed} case(s) failed (see above)"
  echo "renovate policy self-test: ${#CONTROLS[@]} controls green, ${nmut} mutants red on their own clause"
}

if [[ "${MODE}" == "self-test" ]]; then
  self_test
else
  check_tree
fi
