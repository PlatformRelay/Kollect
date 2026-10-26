#!/usr/bin/env bash
# renovate_policy_test.sh -- the self-hosted Renovate policy guard (openspec change
# dependency-update-automation, DUA-1 to DUA-5, DUA-7, DUA-8).
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
# Task 1.2 adds the config, validator and docs clauses (T10 to T17) to CLAUSES and MUTANTS below.
#
# Every value is read parsed (yq), never from YAML comments; the action's version comment is
# read with yq's line_comment.
#
# Usage:
#   hack/test/renovate_policy_test.sh [root]              # check a tree (default: the repo root)
#   hack/test/renovate_policy_test.sh --self-test [root]  # mutate copies of a fixture tree
#
# --self-test builds a known-good FIXTURE tree from <root>: .github/ is copied, renovate.yaml is
# replaced by a correct workflow written below, and the lint job of ci.yaml gets the validator
# step. It never reads the real renovate.yaml, so it is green before section 2 of the change
# lands and stays green after it, and a bot bump of the real pins cannot turn it red. Each mutant
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

# --- the clause table ---------------------------------------------------------------------

CLAUSES=(t1_token t2_fail_closed t3_permissions t4_schedule t5_concurrency t6_pin t7_environment t8_validator t9_no_merge)

check_tree() {
  locate
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

# Paths of the tree a fixture copies (1.2 adds the config, validator and docs files).
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
)
CONTROLS=(c_noop c_bot_bump c_comment_decoy c_hour_step c_shell_bash)

self_test() {
  local work base out row clause fn want n=0 lines other allowed
  work="$(mktemp -d)"
  # shellcheck disable=SC2064 # expand now: the trap must remove this run's directory
  trap "rm -rf '${work:?}'" EXIT
  base="${work}/base"
  build_fixture "${ROOT}" "${base}"
  out="${work}/out"

  for fn in "${CONTROLS[@]}"; do
    FX="${work}/copy"
    cp -R "${base}" "${FX}"
    "${fn}"
    if ! bash "${SELF}" "${FX}" >"${out}" 2>&1; then
      cat "${out}" >&2
      die "control ${fn} is red: the guard refuses a correct tree"
    fi
    rm -rf "${FX:?}"
    echo "ok - control ${fn} stays green"
  done

  for row in "${MUTANTS[@]}"; do
    IFS='|' read -r clause fn want <<<"${row}"
    allowed="^FAIL: (${clause//+/|}): "
    clause="${clause%%+*}"
    FX="${work}/copy"
    cp -R "${base}" "${FX}"
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
    rm -rf "${FX:?}"
    n=$((n + 1))
    echo "ok - mutant ${fn} reds ${clause} with its own message"
  done
  echo "renovate policy self-test: ${#CONTROLS[@]} controls green, ${n} mutants red on their own clause"
}

if [[ "${MODE}" == "self-test" ]]; then
  self_test
else
  check_tree
fi
