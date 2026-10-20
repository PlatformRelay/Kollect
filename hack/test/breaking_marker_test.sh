#!/usr/bin/env bash
# BMC-1..BMC-5 (openspec change breaking-marker-check): hack/check-breaking-marker.sh fails a
# pull request that declares a breaking change (a `type!:` PR title or a BREAKING CHANGE footer
# in the PR body) when none of its commit titles carries the `!` marker, warns on the reverse,
# and fails closed on bad input; .github/workflows/breaking-marker.yaml runs it fork-safe and
# least-privilege.
#
# Each fixture under hack/test/testdata/breaking-marker/<case>/ holds:
#   title     the PR title (one line)
#   body.txt  the PR body (optional; absent means an empty body)
#   commits   one commit title per line, committed in order on top of a base commit; a line
#             `merge:<title>` adds a side commit and a merge commit with that title
#   case      key=value lines: exit=<code>, out=<substring the output must contain> (repeatable),
#             not=<substring the output must not contain> (repeatable), crlf=1 (send the body
#             with CRLF line ends, as GitHub stores bodies edited in the web UI)
#
# Usage:
#   hack/test/breaking_marker_test.sh              # run every case against the real script
#   hack/test/breaking_marker_test.sh --self-test  # mutate copies, assert each mutant reds
#
# BMC_SCRIPT and BMC_WORKFLOW override the files under test (the self-test uses them).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REAL_SCRIPT="${BMC_SCRIPT:-${ROOT}/hack/check-breaking-marker.sh}"
REAL_WORKFLOW="${BMC_WORKFLOW:-${ROOT}/.github/workflows/breaking-marker.yaml}"
FIXTURES="${ROOT}/hack/test/testdata/breaking-marker"
ELIG="${ROOT}/hack/release/verify-eligibility.sh"

fail() {
  printf 'breaking marker test: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

command -v yq >/dev/null 2>&1 || fail "yq (mikefarah/yq v4) is required"
[[ -d "${FIXTURES}" ]] || fail "fixtures missing: ${FIXTURES}"

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK:?}"' EXIT

git_q() {
  GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.org GIT_COMMITTER_NAME=t \
    GIT_COMMITTER_EMAIL=t@example.org git -C "$1" "${@:2}"
}

# build_repo <dir> <commits-file>: a base commit, then one empty commit per title. Prints the base.
build_repo() {
  local dir="$1" commits="$2" subject
  rm -rf "${dir}"
  git init -q "${dir}"
  git_q "${dir}" commit -q --allow-empty -m "base"
  git -C "${dir}" rev-parse HEAD
  local tree side merge
  while IFS= read -r subject || [[ -n "${subject}" ]]; do
    [[ -n "${subject}" ]] || continue
    if [[ "${subject}" == merge:* ]]; then
      tree="$(git -C "${dir}" rev-parse 'HEAD^{tree}')"
      side="$(git_q "${dir}" commit-tree "${tree}" -p HEAD -m "side commit")"
      merge="$(git_q "${dir}" commit-tree "${tree}" -p HEAD -p "${side}" -m "${subject#merge:}")"
      git -C "${dir}" reset -q --hard "${merge}"
    else
      # verbatim: keep a subject's bytes as the fixture wrote them (a bare CR included)
      git_q "${dir}" commit -q --allow-empty --cleanup=verbatim -m "${subject}"
    fi
  done <"${commits}"
}

# run_fixture <script> <case-dir>: prints a reason and returns 1 when the case does not hold.
run_fixture() {
  local script="$1" c="$2" name repo base body rc out key val want_exit
  name="$(basename "${c}")"
  repo="${WORK}/repo-${name}"
  base="$(build_repo "${repo}" "${c}/commits")"
  body=""
  [[ -f "${c}/body.txt" ]] && body="$(cat "${c}/body.txt")"
  if grep -qx 'crlf=1' "${c}/case"; then
    body="$(printf '%s\n' "${body}" | sed 's/$/\r/')"
  fi
  rc=0
  out="$(cd "${repo}" && PR_TITLE="$(cat "${c}/title")" PR_BODY="${body}" bash "${script}" "${base}" HEAD 2>&1)" || rc=$?
  want_exit="$(sed -n 's/^exit=//p' "${c}/case")"
  [[ -n "${want_exit}" ]] || { echo "${name}: case file has no exit="; return 1; }
  [[ "${rc}" == "${want_exit}" ]] || { echo "${name}: exit ${rc}, want ${want_exit}: ${out}"; return 1; }
  while IFS='=' read -r key val; do
    case "${key}" in
      exit) ;;
      out) [[ "${out}" == *"${val}"* ]] || { echo "${name}: output lacks '${val}': ${out}"; return 1; } ;;
      not) [[ "${out}" != *"${val}"* ]] || { echo "${name}: output has '${val}': ${out}"; return 1; } ;;
      crlf | "") ;;
      *) echo "${name}: unknown case key '${key}'"; return 1 ;;
    esac
  done <"${c}/case"
}

# BMC-3: bad input exits 2, never 0 or 1.
case_bad_input() {
  local script="$1" repo="${WORK}/repo-bad" base rc
  printf 'feat: x\n' >"${WORK}/one-commit"
  base="$(build_repo "${repo}" "${WORK}/one-commit")"
  rc=0
  (cd "${repo}" && PR_TITLE='feat!: x' bash "${script}" deadbeefdeadbeefdeadbeefdeadbeefdeadbeef HEAD) >/dev/null 2>&1 || rc=$?
  [[ "${rc}" == 2 ]] || { echo "bad input: unknown revision exits ${rc}, want 2"; return 1; }
  rc=0
  (cd "${repo}" && env -u PR_TITLE bash "${script}" "${base}" HEAD) >/dev/null 2>&1 || rc=$?
  [[ "${rc}" == 2 ]] || { echo "bad input: unset PR_TITLE exits ${rc}, want 2"; return 1; }
  rc=0
  (cd "${repo}" && PR_TITLE='feat: x' bash "${script}" "${base}") >/dev/null 2>&1 || rc=$?
  [[ "${rc}" == 2 ]] || { echo "bad input: one argument exits ${rc}, want 2"; return 1; }
}

# BMC-4: the workflow's shape, statically.
case_workflow() {
  local wf="$1" job uses runs bad
  [[ -f "${wf}" ]] || { echo "workflow: ${wf} is missing"; return 1; }
  if grep -v '^[[:space:]]*#' "${wf}" | grep -q 'pull_request_target'; then
    echo "workflow: mentions pull_request_target -- it runs with a write token and secrets on fork code"
    return 1
  fi
  [[ "$(yq eval '."on" | keys | join(",")' "${wf}")" == "pull_request" ]] ||
    { echo "workflow: triggers are '$(yq eval '."on" | keys | join(",")' "${wf}")', want only pull_request"; return 1; }
  [[ "$(yq eval '."on".pull_request | keys | join(",")' "${wf}")" == "types" ]] ||
    { echo "workflow: pull_request has keys besides types -- a paths or branches filter would skip the check"; return 1; }
  [[ "$(yq eval '."on".pull_request.types | sort | join(",")' "${wf}")" == "edited,opened,reopened,synchronize" ]] ||
    { echo "workflow: pull_request types are not edited,opened,reopened,synchronize -- a title or body edit must re-run the check"; return 1; }
  [[ "$(yq eval -o=json -I=0 '.permissions' "${wf}")" == '{"contents":"read"}' ]] ||
    { echo "workflow: top-level permissions are not exactly contents: read"; return 1; }
  [[ "$(yq eval 'keys | join(",")' "${wf}")" == "name,on,permissions,concurrency,jobs" ]] ||
    { echo "workflow: top-level keys are not exactly name, on, permissions, concurrency, jobs (no workflow env, no defaults)"; return 1; }
  if grep -v '^[[:space:]]*#' "${wf}" | grep -Eq '(^|[^A-Za-z0-9_])secrets([^A-Za-z0-9_]|$)'; then
    echo "workflow: references secrets -- the check needs none, and same-repo PRs would receive them"
    return 1
  fi
  [[ "$(yq eval '.jobs | keys | join(",")' "${wf}")" == "breaking-marker" ]] ||
    { echo "workflow: jobs are not exactly breaking-marker"; return 1; }
  job='.jobs["breaking-marker"]'
  [[ "$(yq eval "${job}.name" "${wf}")" == "breaking-marker" ]] ||
    { echo "workflow: the job's name is not breaking-marker"; return 1; }
  [[ "$(yq eval "${job} | keys | join(\",\")" "${wf}")" == "name,runs-on,timeout-minutes,steps" ]] ||
    { echo "workflow: job keys are not exactly name, runs-on, timeout-minutes, steps -- permissions, if, continue-on-error or env would widen or silence it"; return 1; }
  [[ "$(yq eval "${job}.runs-on" "${wf}")" == "ubuntu-latest" ]] ||
    { echo "workflow: runs-on is not ubuntu-latest -- PR code must not reach a self-hosted runner"; return 1; }
  [[ "$(yq eval "${job}.timeout-minutes" "${wf}")" =~ ^([1-9]|10)$ ]] ||
    { echo "workflow: timeout-minutes is not between 1 and 10"; return 1; }
  [[ "$(yq eval "${job}.steps | length" "${wf}")" == 2 ]] ||
    { echo "workflow: the job does not have exactly two steps (checkout, check)"; return 1; }
  [[ "$(yq eval "${job}.steps[0].with | keys | sort | join(\",\")" "${wf}")" == "fetch-depth,filter,persist-credentials" ]] ||
    { echo "workflow: checkout with-keys are not exactly fetch-depth, filter, persist-credentials -- no repository or ref override"; return 1; }
  [[ "$(yq eval "${job}.steps[1].env | keys | sort | join(\",\")" "${wf}")" == "PR_BODY,PR_TITLE" ]] ||
    { echo "workflow: the check step's env keys are not exactly PR_TITLE and PR_BODY"; return 1; }
  local k
  while IFS= read -r k; do
    case "${k}" in
      name | uses | with | env | run) ;;
      *) echo "workflow: a step declares '${k}', outside name, uses, with, env, run -- if or continue-on-error would silence it"; return 1 ;;
    esac
  done < <(yq eval "${job}.steps[] | keys | .[]" "${wf}")
  while IFS= read -r uses; do
    [[ -n "${uses}" ]] || continue
    [[ "${uses}" =~ ^actions/checkout@ ]] || { echo "workflow: action '${uses}' is not actions/checkout -- no other action is allowed"; return 1; }
    [[ "${uses}" =~ @[0-9a-f]{40}$ ]] || { echo "workflow: action '${uses}' is not pinned to a commit SHA"; return 1; }
  done < <(yq eval "${job}.steps[].uses | select(. != null)" "${wf}")
  [[ "$(yq eval "[${job}.steps[] | select(.uses | test(\"^actions/checkout@\"))] | length" "${wf}")" == 1 ]] ||
    { echo "workflow: want exactly one actions/checkout step"; return 1; }
  [[ "$(yq eval "${job}.steps[] | select(.uses | test(\"^actions/checkout@\")) | .with.\"persist-credentials\"" "${wf}")" == "false" ]] ||
    { echo "workflow: checkout does not set persist-credentials: false"; return 1; }
  runs="$(yq eval "${job}.steps[].run | select(. != null)" "${wf}")"
  # shellcheck disable=SC2016 # the literal expression syntax, not a shell expansion
  if [[ "${runs}" == *'${{'* ]]; then
    # shellcheck disable=SC2016
    echo 'workflow: a run body interpolates ${{ }} -- PR text inlined into shell is a script injection; pass it through env'
    return 1
  fi
  [[ "${runs}" == *'hack/check-breaking-marker.sh HEAD^1 HEAD^2'* ]] ||
    { echo "workflow: no step runs hack/check-breaking-marker.sh HEAD^1 HEAD^2"; return 1; }
  [[ "$(yq eval "${job}.steps[1].run" "${wf}")" == "bash hack/check-breaking-marker.sh HEAD^1 HEAD^2" ]] ||
    { echo "workflow: the check step's run body is not exactly 'bash hack/check-breaking-marker.sh HEAD^1 HEAD^2' -- '|| true' or extra commands would silence or widen it"; return 1; }
  bad="$(yq eval "${job}.steps[] | select(.run != null and (.run | test(\"check-breaking-marker\"))) | .env.PR_TITLE + \"|\" + .env.PR_BODY" "${wf}")"
  # shellcheck disable=SC2016
  [[ "${bad}" == '${{ github.event.pull_request.title }}|${{ github.event.pull_request.body }}' ]] ||
    { echo "workflow: the check step does not take PR_TITLE and PR_BODY from the event payload through env"; return 1; }
  if [[ -f "${ELIG}" ]] && grep -Eq '"?breaking-marker"?' "${ELIG}"; then
    echo "workflow: breaking-marker is listed in hack/release/verify-eligibility.sh -- making it required is the operator's call"
    return 1
  fi
}

# check_all <script> <workflow>: every case; one line per failure; non-zero if any failed.
check_all() {
  local script="$1" wf="$2" c out failed=0
  for c in "${FIXTURES}"/*/; do
    out="$(run_fixture "${script}" "${c%/}")" || { printf '%s\n' "${out}"; failed=1; }
  done
  out="$(case_bad_input "${script}")" || { printf '%s\n' "${out}"; failed=1; }
  out="$(case_workflow "${wf}")" || { printf '%s\n' "${out}"; failed=1; }
  return "${failed}"
}

if [[ "${1:-}" != --self-test ]]; then
  [[ -f "${REAL_SCRIPT}" ]] || fail "${REAL_SCRIPT} is missing"
  if ! check_all "${REAL_SCRIPT}" "${REAL_WORKFLOW}" >"${WORK}/failures"; then
    cat "${WORK}/failures" >&2
    fail "the breaking-marker check does not hold (failures above)"
  fi
  for c in "${FIXTURES}"/*/; do pass "$(basename "${c}")"; done
  pass "bad input exits 2"
  pass "workflow shape"
  echo "All breaking marker tests passed."
  exit 0
fi

# --self-test: one mutant per clause, a sed-edited copy of the script or the workflow. Each must
# red the check suite with its own message; the no-op control must stay green.
mutant() {
  local label="$1" target="$2" expect="$3" sed_expr="$4" script="${REAL_SCRIPT}" wf="${REAL_WORKFLOW}" copy out
  copy="${WORK}/m-${label}"
  if [[ "${target}" == script ]]; then
    sed -e "${sed_expr}" "${REAL_SCRIPT}" >"${copy}"
    cmp -s "${copy}" "${REAL_SCRIPT}" && fail "self-test: mutant '${label}' is identical to the script -- the sed anchor no longer matches"
    script="${copy}"
  else
    sed -e "${sed_expr}" "${REAL_WORKFLOW}" >"${copy}"
    cmp -s "${copy}" "${REAL_WORKFLOW}" && fail "self-test: mutant '${label}' is identical to the workflow -- the sed anchor no longer matches"
    wf="${copy}"
  fi
  if [[ -z "${expect}" ]]; then
    check_all "${script}" "${wf}" >"${WORK}/noop" || fail "self-test: the no-op control reds: $(cat "${WORK}/noop")"
    pass "self-test: no-op control stays green"
    return
  fi
  if out="$(check_all "${script}" "${wf}")"; then
    fail "self-test: mutant '${label}' survived"
  fi
  [[ "${out}" == *"${expect}"* ]] || fail "self-test: mutant '${label}' red for another reason: ${out}"
  pass "self-test: mutant '${label}' reds (${expect})"
}

[[ -f "${REAL_SCRIPT}" ]] || fail "${REAL_SCRIPT} is missing"
# The sed expressions are single-quoted on purpose: they match the files' literal text.
# shellcheck disable=SC2016
{
  mutant title-ignored script 'title-bang-plain-commits: exit 0, want 1' 's/^\[\[ "\${PR_TITLE}" =~ \${title_re} \]\] && title_marked=true$/:/'
  mutant body-ignored script 'body-footer-plain-commits: exit 0, want 1' 's/^  if \[\[ "\${line}" =~ \${footer_re} \]\]; then$/  if false; then/'
  mutant bang-optional script 'pr478: exit 0, want 1' 's/^marker_re=.*/marker_re='"'"'^(:[a-z0-9_]+: )?[a-z]+(\\([^)]*\\))?!?:'"'"'/'
  mutant gitmoji-loose script 'cliff-unparseable: exit 0, want 1' 's/^marker_re=.*/marker_re='"'"'^(:[a-z0-9_+-]+:[[:space:]]+)?[a-z]+(\\([^)]*\\))?!:'"'"'/'
  mutant merges-counted script 'merge-commit-marked: exit 0, want 1' 's/git log --no-merges --format/git log --format/'
  mutant fence-any-closes script 'fence-mixed: exit 0, want 1' 's/^    elif \[\[ "\${BASH_REMATCH\[1\]}" == "\${fence}" \]\]; then$/    else/'
  mutant cr-unescaped script "commit-cr-subject: output lacks" '/^    s="\${s\/\/\$.\\r.\/%0D}"$/d'
  mutant warn-despite-marked-title script 'warn-not-when-title-marked: output has' 's/^if ((\${#marked\[@\]} > 0)) \&\& ! \${title_marked}; then$/if ((${#marked[@]} > 0)); then/'
  mutant hyphen-unmatched script 'body-footer-hyphen: exit 0, want 1' 's/BREAKING\[ -\]CHANGE/BREAKING CHANGE/'
  mutant fence-unskipped script 'mentions-only: exit 1, want 0' 's/^  \[\[ -n "\${fence}" \]\] \&\& continue$/  :/'
  mutant commits-unread script 'title-and-commit-marked: exit 1, want 0' 's/^  \[\[ "\${s}" =~ \${marker_re} \]\] \&\& marked+=("\${s}")$/  :/'
  mutant warning-silent script "commit-marked-title-plain: output lacks '::warning title=breaking marker::'" 's/^    printf .::warning title=breaking marker::.*/    :/'
  mutant percent-unescaped script "output lacks 'keep 100%25 of rows'" 's/^    s="\${s\/\/%\/%25}"$/    :/'
  mutant warning-fails script 'commit-marked-title-plain: exit 1, want 0' 's/^  exit 0 # warnings only$/  exit 1/'
  mutant unknown-rev-accepted script 'bad input: unknown revision exits' 's/^  if ! git rev-parse --verify --quiet .*/  if false; then/'
  mutant title-unset-accepted script 'unset PR_TITLE exits' 's/^\[\[ -n "\${PR_TITLE+set}" \]\] ||.*/PR_TITLE="${PR_TITLE-}"/'
  mutant argc-unchecked script 'one argument exits' 's/^\[\[ \$# -eq 2 \]\] || usage$/:/'
  mutant wf-target workflow 'mentions pull_request_target' 's/^  pull_request:$/  pull_request_target:/'
  mutant wf-no-edited workflow 'a title or body edit must re-run the check' 's/types: \[opened, edited, synchronize, reopened\]/types: [opened, synchronize, reopened]/'
  mutant wf-write workflow 'top-level permissions are not exactly contents: read' 's/^  contents: read$/  contents: write/'
  mutant wf-unpinned workflow 'is not pinned to a commit SHA' 's/actions\/checkout@[0-9a-f]\{40\}/actions\/checkout@v6/'
  mutant wf-persist workflow 'persist-credentials: false' 's/persist-credentials: false/persist-credentials: true/'
  mutant wf-third-party workflow 'is not actions/checkout' 's/^      - uses: actions\/checkout@\([0-9a-f]\{40\}\).*/      - uses: evil\/act@\1/'
  mutant wf-secrets workflow 'references secrets' 's/^          PR_BODY: .*/          PR_BODY: ${{ secrets.GITHUB_TOKEN }}/'
  mutant wf-job-silenced workflow 'job keys are not exactly' 's/^    timeout-minutes: 5$/    timeout-minutes: 5\n    continue-on-error: true/'
  mutant wf-step-skipped workflow "a step declares 'if'" 's/^      - name: Check breaking-change markers reach a commit title$/&\n        if: false/'
  mutant wf-tojson-secrets workflow 'references secrets' 's/^          PR_BODY: .*/          PR_BODY: ${{ toJSON(secrets) }}/'
  mutant wf-index-secrets workflow 'references secrets' "s/^          PR_BODY: .*/          PR_BODY: \${{ secrets['GITHUB_TOKEN'] }}/"
  mutant wf-or-true workflow 'run body is not exactly' 's/^          bash hack\/check-breaking-marker.sh HEAD^1 HEAD^2$/& || true/'
  mutant wf-extra-env workflow 'env keys are not exactly' 's/^          PR_BODY: .*/&\n          REF: ${{ github.event.pull_request.head.ref }}/'
  mutant wf-extra-step workflow 'exactly two steps' '$a\
      - run: echo extra'
  mutant wf-self-hosted workflow 'runs-on is not ubuntu-latest' 's/^    runs-on: ubuntu-latest$/    runs-on: self-hosted/'
  mutant wf-paths workflow 'keys besides types' 's/^    types: \[opened, edited, synchronize, reopened\]$/&\n    paths: ["hack\/**"]/'
  mutant wf-checkout-ref workflow 'no repository or ref override' 's/^          persist-credentials: false$/&\n          ref: main/'
  mutant title-strict script 'title-loose-gitmoji: exit 0, want 1' 's/^\[\[ "\${PR_TITLE}" =~ \${title_re} \]\]/[[ "${PR_TITLE}" =~ ${marker_re} ]]/'
  mutant wf-inline workflow 'script injection' 's/^          bash hack\/check-breaking-marker.sh HEAD^1 HEAD^2$/          PR_TITLE="${{ github.event.pull_request.title }}" bash hack\/check-breaking-marker.sh HEAD^1 HEAD^2/'
  mutant noop script '' '$a\
# no-op control'
}

echo "All breaking marker self-tests passed."
