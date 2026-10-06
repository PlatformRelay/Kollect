#!/usr/bin/env bash
# CI-WFS-01 -- meta-test for openspec change ci-workflow-hardening (CWS-1..CWS-7).
#
# What it locks, each in BOTH directions where a mutant can be built:
#   CWS-1  ci.yaml runs zizmor --offline at a pinned --min-severity and pinned config over
#          .github/, installed from a pinned checksum-verified release; the workflow-security
#          job is unconditional (no if / needs / continue-on-error anywhere that could skip it,
#          and no key on the job or its steps beyond the reviewed allowlists).
#   CWS-2  every suppression in .github/zizmor.yml carries a comment directly above it stating
#          why it is safe; the config names this gate as the enforcer.
#   CWS-3  dependency-review runs on pull_request only, uses a SHA-pinned action, expresses its
#          licence policy with allow-licenses, never uses deny-licenses, and is NOT a release
#          eligibility check (it does not run on main, so it cannot gate a merge).
#   CWS-4  ci.yaml and e2e-smoke.yaml declare the exact CWS-4 concurrency: the group is
#          `${{ github.workflow }}-<PR number or SHA>` and cancel-in-progress is
#          `${{ github.event_name == 'pull_request' }}` -- PR runs supersede each other, main
#          pushes are never cancelled (release eligibility reads exact-SHA checks on main).
#   CWS-5  ci.yaml keeps its push: [main] trigger, workflow-security IS a release eligibility
#          check, and ci.yaml still runs its guards on pull_request.
#   CWS-6  every guard script under hack/test/ that CI runs, in every mode its code recognises
#          (bare, --self-test, and any other flag), is invoked by a step of a job in the
#          required set; a guard in a non-required job is not a gate. The required set is
#          derived from hack/release/verify-eligibility.sh's required_checks plus
#          dependency-review, and must list lint, vulncheck, workflow-security and
#          dependency-review. This meta-test must itself be run by ci.yaml in both of its own
#          modes -- a gate CI does not run guards nothing (the LAB-DEKIND defect class).
#   CWS-7  --self-test mutates a COPY of the tree to break each of CWS-1..CWS-6 and asserts
#          each check reds with the MESSAGE of the assertion the mutation was built to trip
#          (a non-zero exit alone is not evidence), plus one unmutated no-op control.
#
# Usage:
#   hack/test/ci_workflow_security_test.sh              # check the repo tree (CI plain mode)
#   hack/test/ci_workflow_security_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/ci_workflow_security_test.sh <tree-root>  # check an arbitrary tree root
#
# Mutant-harness notes (METHOD-MUTHARNESS-01/-02, see ci_docs_gate_test.sh, which this file
# follows): a substitution's replacement half is itself interpreted -- by the shell and by the
# tool. Two hazards, both avoided here:
#   * yq's sub()/replace templates expand `${name}` as a named-capture reference to the EMPTY
#     string, so every yq mutation here is a plain assignment of a literal string;
#   * every textual substitution's replacement carries no dollar (perl `$1` captures aside --
#     single-quoted programs are not shell-expanded), and every mutant is READ BACK before the
#     gate runs, so a more-broken mutant cannot red for the wrong reason.
# One invocation per line is the convention: a guard step's `run:` body is scanned line by
# line, so a continuation `bash hack/test/x.sh \<newline> --self-test` is NOT recognised.
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

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
ROOT="${ROOT%/}"

if ! command -v yq >/dev/null 2>&1; then
  echo "yq not found; install yq (mikefarah/yq v4) to run this test" >&2
  exit 1
fi

CI="${ROOT}/.github/workflows/ci.yaml"
SMOKE="${ROOT}/.github/workflows/e2e-smoke.yaml"
ZIZMOR_CFG="${ROOT}/.github/zizmor.yml"
INSTALLER="${ROOT}/hack/install-zizmor.sh"
ELIG="${ROOT}/hack/release/verify-eligibility.sh"

[[ -f "${CI}" ]] || fail "CWS-1: workflow not found: ${CI}"
[[ -f "${SMOKE}" ]] || fail "CWS-4: ${SMOKE} not found"
[[ -f "${ZIZMOR_CFG}" ]] || fail "CWS-2: zizmor config not found: ${ZIZMOR_CFG}"
[[ -f "${INSTALLER}" ]] || fail "CWS-1: installer not found: ${INSTALLER}"
[[ -f "${ELIG}" ]] || fail "CWS-5/CWS-6: required-checks source not found: ${ELIG}"
[[ -d "${ROOT}/.github/workflows" ]] || fail "CWS-6: no workflow directory under ${ROOT}/.github"

# --- helpers -----------------------------------------------------------------

# Run bodies are matched against a COMMENT-STRIPPED view: a commented-out command is not a
# command (the convention from ci_docs_gate_test.sh / dist_ci_wiring_test.sh).
strip_comments() {
  sed -e 's/[[:space:]]*#.*$//' <<<"$1" | sed -e 's/[[:space:]]*$//' | grep -v '^$' || true
}

in_allowlist() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    if [[ "${item}" == "${needle}" ]]; then
      return 0
    fi
  done
  return 1
}

elig_names() {
  awk '/^required_checks=\(/{f=1;next} /^\)/{f=0} f' "${ELIG}" | tr -s ' \t' '\n' | grep -v '^$' || true
}

elig_contains() {
  local names
  names="$(elig_names | tr '\n' ' ')"
  in_allowlist "$1" ${names}
}

# --- CWS-1: the workflow-security job itself ---------------------------------

cws1_job() {
  local jobtype keys key n i persist runs_on
  # yq's type() is tagged ("!!map"), hence the suffix match.
  jobtype="$(yq eval '.jobs["workflow-security"] // "" | type' "${CI}")"
  [[ "${jobtype}" == *map ]] ||
    fail "CWS-1: ci.yaml has no workflow-security job -- the gate must be a job of its own, not a step folded into another job"
  keys="$(yq eval '.jobs["workflow-security"] | keys | join(" ")' "${CI}")"
  for key in ${keys}; do
    in_allowlist "${key}" name runs-on steps ||
      fail "CWS-1: the workflow-security job declares '${key}', which is not one of (name runs-on steps) -- a job-level 'if', 'needs', 'continue-on-error' or 'strategy' is a skip switch for the one gate that audits the workflows themselves, so every key beyond the reviewed shape is reported"
  done
  n="$(yq eval '.jobs["workflow-security"].steps | length' "${CI}")"
  # Exactly three steps, each pinned below: a fourth step could overwrite the installed
  # zizmor binary with a green stub (RUNNER_TEMP is first on GITHUB_PATH), so the step LIST
  # is pinned, not just a minimum -- an extra step is a stub route, not extra coverage.
  [[ "${n}" -eq 3 ]] ||
    fail "CWS-1: the workflow-security job has ${n} step(s), expected exactly 3 (checkout, install, audit) -- an extra step can run AFTER the install and BEFORE the audit, replacing the zizmor binary with a green stub, so any step count beyond the reviewed three is rejected"
  # The required context the ruleset sees is the job's DISPLAY name, not its YAML id: rename
  # the display name while keeping the id and every check above still passes while the
  # ruleset watches a context that zizmor never produced.
  local jname
  jname="$(yq eval '.jobs["workflow-security"].name // "workflow-security"' "${CI}")"
  [[ "${jname}" == "workflow-security" ]] ||
    fail "CWS-1: the workflow-security job's display name is '${jname}', expected 'workflow-security' -- the required context is the display name, so renaming it while keeping the YAML job id reports a green 'workflow-security' that zizmor never produced"
  # The display name is what the ruleset sees; exactly ONE job across ALL workflows may
  # declare it -- the ci.yaml job that runs zizmor. Any second one is a decoy check that
  # reports green without auditing anything.
  local decoys
  decoys="$(yq eval-all '.jobs | to_entries[] | select(.value.name == "workflow-security") | .key' "${ROOT}"/.github/workflows/*.yaml 2>/dev/null | grep -c . || true)"
  [[ "${decoys}" -le 1 ]] ||
    fail "CWS-1: ${decoys} jobs across the workflows declare the display name 'workflow-security' -- the required context must be produced by exactly one job, the one that runs zizmor; a second job with the same display name is a decoy check that reports green without auditing anything"
  for ((i = 0; i < n; i++)); do
    local sname
    sname="$(yq eval ".jobs[\"workflow-security\"].steps[${i}].name // \"\"" "${CI}")"
    case "${i}" in
      0) yq eval ".jobs[\"workflow-security\"].steps[0].uses" "${CI}" | grep -q 'actions/checkout@' ||
        fail "CWS-1: step 0 of the workflow-security job is '${sname}', expected the checkout -- the reviewed step order is checkout, install, audit and the audit must come last" ;;
      1) [[ "${sname}" == "Install zizmor" ]] ||
        fail "CWS-1: step 1 of the workflow-security job is '${sname}', expected 'Install zizmor'" ;;
      2) [[ "${sname}" == "Audit workflows (zizmor, offline)" ]] ||
        fail "CWS-1: step 2 of the workflow-security job is '${sname}', expected the zizmor audit step" ;;
    esac
    for key in $(yq eval ".jobs[\"workflow-security\"].steps[${i}] | keys | join(\" \")" "${CI}"); do
      if ! in_allowlist "${key}" name run uses with env; then
        fail "CWS-1: step ${i} of the workflow-security job declares '${key}', which is not one of (name run uses with env) -- an 'if' or 'continue-on-error' here is a skip switch, 'shell' can replace the audit body with 'true {0}' so the job passes without zizmor running, and a 'timeout-minutes: 0' or 'strategy' can remove the gate without any assertion below noticing"
      fi
    done
    # Only the reviewed env binding may exist on a workflow-security step: anything else (for
    # example KOLLECT_FORCE_SHA256, which hack/install-zizmor.sh honours to override the pinned
    # digest) is a skip switch the spec's CWS-1 scenario names.
    for key in $(yq eval ".jobs[\"workflow-security\"].steps[${i}].env // {} | keys | join(\" \")" "${CI}"); do
      [[ "${key}" == "ZIZMOR_VERSION" ]] ||
        fail "CWS-1: step ${i} of the workflow-security job declares env '${key}', which is not ZIZMOR_VERSION -- a second binding here is a skip switch (KOLLECT_FORCE_SHA256 overrides the pinned digest; any other var can short-circuit the audit)"
    done
  done
  local install_idx install_body install_stripped
  install_idx="$(yq eval '.jobs["workflow-security"].steps | to_entries[] | select(.value.run != null) | select(.value.run | test("install-zizmor.sh")) | .key' "${CI}")"
  [[ -n "${install_idx}" ]] ||
    fail "CWS-1: no workflow-security step runs hack/install-zizmor.sh -- a gate installed any other way (pipx, a stub on GITHUB_PATH) is not the gate that was reviewed"
  install_body="$(yq eval ".jobs[\"workflow-security\"].steps[${install_idx}].run" "${CI}")"
  install_stripped="$(strip_comments "${install_body}" | tr -s ' \n' ' ' | sed -e 's/[[:space:]]*$//')"
  [[ "${install_stripped}" == "bash hack/install-zizmor.sh \"\${RUNNER_TEMP}\" echo \"\${RUNNER_TEMP}\" >> \"\${GITHUB_PATH}\"" ]] ||
    fail "CWS-1: the workflow-security install step must run exactly 'bash hack/install-zizmor.sh \"\${RUNNER_TEMP}\"' then add the dir to GITHUB_PATH; got '${install_stripped}' -- the reviewed installer is the only thing that may put zizmor on PATH"
  persist="$(yq eval '.jobs["workflow-security"].steps[] | select(.uses != null) | select(.uses | test("actions/checkout")) | .with."persist-credentials"' "${CI}")"
  [[ "${persist}" == "false" ]] ||
    fail "CWS-1: the workflow-security checkout must set persist-credentials: false -- the audit is offline and needs no token, so a persisted one is an exfiltration surface with no purpose"
  runs_on="$(yq eval '.jobs["workflow-security"].runs-on' "${CI}")"
  [[ "${runs_on}" == "ubuntu-latest" ]] ||
    fail "CWS-1: workflow-security runs on '${runs_on}', expected ubuntu-latest -- a self-hosted or private runner label changes what the audit can see"
  pass "CWS-1: workflow-security is an unconditional job (no if, needs or continue-on-error), token-free, on ubuntu-latest"
}

ws_audit_step_index() {
  local idx
  idx="$(yq eval '.jobs["workflow-security"].steps | to_entries[] | select(.value.run != null) | select(.value.run | test("^zizmor ")) | .key' "${CI}")"
  [[ -n "${idx}" ]] ||
    fail "CWS-1: no workflow-security step starts the zizmor audit (a step whose run body starts with 'zizmor')"
  echo "${idx}"
}

cws1_invocation() {
  local idx body stripped
  idx="$(ws_audit_step_index)"
  body="$(yq eval ".jobs[\"workflow-security\"].steps[${idx}].run" "${CI}")"
  stripped="$(strip_comments "${body}" | tr -s ' \n' ' ' | sed -e 's/[[:space:]]*$//')"
  [[ "${stripped}" == "zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/" ]] ||
    fail "CWS-1: the workflow-security audit step must run exactly 'zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/'; got '${stripped}' -- each pinned element is load-bearing: --offline forbids every online audit, --min-severity is the recorded threshold decision, the config path is where the reviewed suppressions live, and .github/ is the audit scope"
}

cws1_version_pin() {
  local ci_version installer_version pins
  ci_version="$(yq eval '.jobs["workflow-security"].steps[] | select(.env.ZIZMOR_VERSION != null) | .env.ZIZMOR_VERSION' "${CI}")"
  [[ -n "${ci_version}" ]] ||
    fail "CWS-1: the workflow-security install step must pin ZIZMOR_VERSION in its env -- a floating release cannot be checksum-verified"
  installer_version="$(sed -n 's/.*ZIZMOR_VERSION:-\([0-9][0-9.]*\).*/\1/p' "${INSTALLER}" | head -1)"
  [[ -n "${installer_version}" ]] ||
    fail "CWS-1: hack/install-zizmor.sh must default ZIZMOR_VERSION to a pinned release (VERSION=\"\${ZIZMOR_VERSION:-X.Y.Z}\")"
  [[ "${ci_version}" == "${installer_version}" ]] ||
    fail "CWS-1: zizmor version drift -- ci.yaml pins ${ci_version}, hack/install-zizmor.sh defaults to ${installer_version}; bump both in one commit or the checksum pins stop matching the release"
  pins="$(grep -c 'PINNED_SHA256=' "${INSTALLER}" || true)"
  [[ "${pins}" -ge 4 ]] ||
    fail "CWS-1: hack/install-zizmor.sh pins ${pins} SHA256 digests, expected one per supported platform (>=4) -- the install fails closed on a version bump whose digests were not updated"
  grep -q 'verify_sha256' "${INSTALLER}" ||
    fail "CWS-1: hack/install-zizmor.sh must verify the tarball through hack/lib/verify-sha256.sh -- an unverified download is the defect class this gate exists for"
  grep -q 'fetch_to' "${INSTALLER}" ||
    fail "CWS-1: hack/install-zizmor.sh must download through hack/lib/fetch.sh (pinned protocol, retries) like every other hack/install-*.sh"
  pass "CWS-1: zizmor ${ci_version} is pinned in both ci.yaml and the installer, with >=4 fail-closed checksum pins"
}

# --- CWS-2: suppressions are justified ---------------------------------------

cws2_suppressions() {
  # Suppression semantics (round-one/round-two review consensus, tightened after the approval
  # round): a suppression is a RULE under `rules:`, and each rule must carry its justification
  # comment directly above the rule key. The check goes through yq, not raw text, so flow
  # style (`rules: {cache-poisoning: {ignore: [...]}}`) and deeper indentation cannot bypass
  # it: a flow-style rules value is rejected outright, and every key of a block rules map
  # must carry a head comment. With `rules: {}` there is nothing to check, which is why the
  # config carries that exact spelling.
  local rules_type flow_style key
  rules_type="$(yq eval '.rules | type' "${ZIZMOR_CFG}" 2>&1)" ||
    fail "CWS-2: cannot parse ${ZIZMOR_CFG}: ${rules_type}"
  [[ "${rules_type}" == *map ]] || return 0
  # A non-empty rules map must be block style: a flow-style rules value hides the entries
  # from the line-level review this gate enforces. An EMPTY map (rules: {}) is fine and is
  # why the config carries that exact spelling.
  if [[ -n "$(yq eval '.rules | keys | .[]' "${ZIZMOR_CFG}")" ]]; then
    flow_style="$(yq eval '.rules | style' "${ZIZMOR_CFG}")"
    if [[ "${flow_style}" == "flow" ]]; then
      fail "CWS-2: suppressions in .github/zizmor.yml must be written in block style -- a flow-style rules map hides the entries from the line-level review this gate enforces; write the rule as a block with its justification comment directly above the key"
    fi
  fi
  awk '
    BEGIN { in_rules = 0; prev = "" }
    /^rules:/ {
      if ($0 ~ /^rules:[[:space:]]*\{\}[[:space:]]*$/) { exit 0 }
      in_rules = 1
      next
    }
    in_rules && /^[[:space:]]*$/ { next }
    in_rules && /^[[:space:]]*#/ { prev = $0; next }
    in_rules {
      # An ignore list item carries its own entry comment (the spec names the entry).
      if ($0 ~ /^[[:space:]]+-[[:space:]]/) {
        if (prev !~ /^[[:space:]]*#/) {
          printf "FAIL: CWS-2: suppression entry `%s` in .github/zizmor.yml has no comment directly above it stating why it is safe -- the config names this gate as the enforcer of exactly this contract\n", $0
          exit 1
        }
        prev = ""
        next
      }
      # Every other key under a rule -- the rule key itself AND a policy switch inside it
      # (for example `disable: true`, which switches the audit off with no list to inspect) --
      # carries the block comment. `ignore:` is exempt: it is the structure that holds the
      # items this walk checks one by one.
      if ($0 ~ /^[[:space:]]*[a-zA-Z0-9_-]+:/) {
        key = $0
        sub(/^[[:space:]]*/, "", key)
        sub(/:.*/, "", key)
        if (key != "ignore" && prev !~ /^[[:space:]]*#/) {
          printf "FAIL: CWS-2: suppression `%s` in .github/zizmor.yml has no comment directly above it stating why it is safe -- the config names this gate as the enforcer of exactly this contract\n", $0
          exit 1
        }
        prev = ""
        next
      }
      prev = ""
    }
  ' "${ZIZMOR_CFG}"
  # Inline suppressions count too: `# zizmor: ignore[...]` in any workflow or action silences
  # a finding zizmor would report, and none of them is reviewed unless the config lists it.
  local inline_hit
  inline_hit="$(grep -rn -- 'zizmor: ignore' "${ROOT}/.github/" 2>/dev/null | head -1 || true)"
  if [[ -n "${inline_hit}" ]]; then
    fail "CWS-2: inline zizmor suppression found at ${inline_hit%%:*} -- inline '# zizmor: ignore[...]' comments are not reviewed suppressions; a finding must be fixed or justified in .github/zizmor.yml where this gate can see it"
  fi
  pass "CWS-2: every suppression in .github/zizmor.yml is commented with its justification (none exist, or each has one)"
}

# --- CWS-3: dependency-review ------------------------------------------------

cws3_job() {
  local present cond uses with_keys checkout_persist key line prev di
  present="$(yq eval '.jobs | has("dependency-review")' "${CI}")"
  [[ "${present}" == "true" ]] || fail "CWS-3: ci.yaml has no dependency-review job"
  for key in $(yq eval '.jobs["dependency-review"] | keys | join(" ")' "${CI}"); do
    in_allowlist "${key}" name if runs-on steps ||
      fail "CWS-3: the dependency-review job declares '${key}', which is not one of (name if runs-on steps) -- 'continue-on-error' or a 'timeout-minutes: 0' turns the job into a silent no-op that still reports green, which is the exact starvation CWS-6 exists to prevent"
  done
  local nsteps
  nsteps="$(yq eval '.jobs["dependency-review"].steps | length' "${CI}")"
  for ((di = 0; di < nsteps; di++)); do
    for key in $(yq eval ".jobs[\"dependency-review\"].steps[${di}] | keys | join(\" \")" "${CI}"); do
      if ! in_allowlist "${key}" name run uses with env; then
        fail "CWS-3: step ${di} of the dependency-review job declares '${key}', which is not one of (name run uses with env) -- an 'if' or 'continue-on-error' on the review step keeps the job green on a vulnerable dependency, a 'shell:' can replace the review body with 'true {0}', and a 'timeout-minutes: 0' or 'strategy' can remove the gate without any assertion below noticing"
      fi
    done
  done
  cond="$(yq eval '.jobs["dependency-review"].if' "${CI}")"
  [[ "${cond}" == "github.event_name == 'pull_request'" ]] ||
    fail "CWS-3: the dependency-review job's 'if' is '${cond}', expected github.event_name == 'pull_request' -- it must not run on main (a push event carries no base SHA to compare against) and must not be skippable on a PR"
  uses="$(yq eval '.jobs["dependency-review"].steps[] | select(.uses != null) | select(.uses | test("dependency-review-action")) | .uses' "${CI}")"
  [[ "${uses}" =~ ^actions/dependency-review-action@[0-9a-f]{40} ]] ||
    fail "CWS-3: actions/dependency-review-action is not SHA-pinned ('${uses}') -- a tag or branch ref lets upstream swap the code the gate runs"
  with_keys="$(yq eval '.jobs["dependency-review"].steps[] | select(.uses != null) | select(.uses | test("dependency-review-action")) | .with | keys | join(" ")' "${CI}")"
  in_allowlist "allow-licenses" ${with_keys} ||
    fail "CWS-3: the dependency-review action declares no allow-licenses licence policy -- the job must name the licences the module graph actually uses"
  # The review step's policy inputs are an allowlist: a 'config-file' input would move the
  # severity/licence policy into a file this gate cannot see, and 'warn-only' or other
  # unreviewed inputs can mute the job (deny-licenses and warn-only have their own messages).
  for key in ${with_keys}; do
    in_allowlist "${key}" allow-licenses allow-dependencies-licenses fail-on-severity ||
      fail "CWS-3: the dependency-review action declares input '${key}', which is not one of the reviewed policy inputs (allow-licenses, allow-dependencies-licenses, fail-on-severity) -- 'config-file' moves the severity and licence policy into a file this gate cannot see, and warn-only-style inputs mute the gate"
  done
  # The policy inputs are allow-listed too: any UNREVIEWED input is a bypass route. In
  # particular 'config-file' moves the licence/severity policy into a file this gate cannot
  # see, and 'warn-only' turns the gate into an annotation.
  for key in ${with_keys}; do
    if ! in_allowlist "${key}" allow-licenses allow-dependencies-licenses fail-on-severity; then
      fail "CWS-3: the dependency-review action declares input '${key}', which is not one of the reviewed policy inputs (allow-licenses, allow-dependencies-licenses, fail-on-severity) -- 'config-file' moves the severity/licence policy into a file this gate cannot see, and any other unreviewed input can mute the gate"
    fi
  done
  if in_allowlist "deny-licenses" ${with_keys}; then
    fail "CWS-3: deny-licenses is a deprecated input -- express the policy with allow-licenses (an allow-list ages well; a deny-list silently admits every new licence)"
  fi
  if in_allowlist "warn-only" ${with_keys}; then
    fail "CWS-3: warn-only turns dependency-review into an advisory -- the job must fail the PR on a finding, not annotate it"
  fi
  if in_allowlist "fail-on-severity" ${with_keys}; then
    # Anchor on the dependency-review with-block line, not the first mention anywhere in the
    # file (a comment mentioning the input further up must not satisfy the why-line), and
    # only when the threshold is actually RAISED above the action's default ('low').
    local sev
    sev="$(yq eval '.jobs["dependency-review"].steps[] | select(.with != null) | select(.with | has("fail-on-severity")) | .with."fail-on-severity"' "${CI}")"
    if [[ "${sev}" != "low" ]]; then
      line="$(awk '/dependency-review:/{in_job=1} in_job && /^[[:space:]]+fail-on-severity:/{print NR; exit}' "${CI}")"
      [[ -n "${line}" ]] ||
        fail "CWS-3: fail-on-severity appears outside the dependency-review with: block -- set it there or remove it"
      prev=$((line - 1))
      sed -n "${prev}p" "${CI}" | grep -q '# why:' ||
        fail "CWS-3: fail-on-severity is set without a '# why:' line above it citing a measured trial -- raising the default threshold mutes exactly the findings the job exists to report"
    fi
  fi
  checkout_persist="$(yq eval '.jobs["dependency-review"].steps[] | select(.uses != null) | select(.uses | test("actions/checkout")) | .with."persist-credentials"' "${CI}")"
  [[ "${checkout_persist}" == "false" ]] ||
    fail "CWS-3: the dependency-review checkout must set persist-credentials: false -- it needs no token, so a persisted one is pure exfiltration surface"
  pass "CWS-3: dependency-review is pull_request-only, SHA-pinned, allow-listed, and not a release check"
}

cws3_not_eligibility() {
  if elig_contains dependency-review; then
    fail "CWS-3: dependency-review appears in hack/release/verify-eligibility.sh required_checks, but it runs on pull_request only and can never report on a release SHA -- listing it there makes release eligibility structurally unpassable"
  fi
  pass "CWS-3: dependency-review is not in the release eligibility required_checks (it does not run on main)"
}

# --- CWS-4: concurrency ------------------------------------------------------

GROUP='${{ github.workflow }}-${{ github.event.pull_request.number || github.sha }}'
CANCEL="\${{ github.event_name == 'pull_request' }}"

cws4_concurrency() {
  local file group cancel
  for file in "${CI}" "${SMOKE}"; do
    group="$(yq eval '.concurrency.group' "${file}")"
    cancel="$(yq eval '.concurrency["cancel-in-progress"]' "${file}")"
    [[ "${group}" == "${GROUP}" ]] ||
      fail "CWS-4: ${file##*/} concurrency.group is '${group}', expected the exact expression '${GROUP}' -- keyed by PR number (SHA on push), prefixed with the workflow so ci.yaml and e2e-smoke.yaml never cancel each other, and never github.ref, which would make two quick merges to main share one group and lose the middle commit's run"
    [[ "${cancel}" == "${CANCEL}" ]] ||
      fail "CWS-4: ${file##*/} concurrency.cancel-in-progress is '${cancel}', expected the exact expression '${CANCEL}' -- PR runs supersede each other; a push-to-main run is never cancelled"
  done
  pass "CWS-4: ci.yaml and e2e-smoke.yaml declare the exact PR-supersede, main-never-cancel concurrency"
}

# --- CWS-5: push trigger and eligibility -------------------------------------

cws5_trigger() {
  local has_push has_main
  has_push="$(yq eval '."on" | has("push")' "${CI}")"
  [[ "${has_push}" == "true" ]] ||
    fail "CWS-5: ci.yaml lost its push trigger -- release eligibility (verify-eligibility.sh) reads exact-SHA checks on main, so a change that stops push runs silently bricks release eligibility"
  # has("push") is true here, so .on.push is a map and branches is safe to read; the //
  # covers a push block that declares no branches at all.
  has_main="$(yq eval '."on".push // {} | .branches // [] | contains(["main"])' "${CI}")"
  [[ "${has_main}" == "true" ]] ||
    fail "CWS-5: ci.yaml's push trigger no longer lists main -- verify-eligibility.sh demands a complete check run for every main SHA, and a push event restricted to other branches produces none"
  pass "CWS-5: ci.yaml still runs on push to main"
}

cws5_eligibility() {
  elig_contains workflow-security ||
    fail "CWS-5: workflow-security is missing from hack/release/verify-eligibility.sh required_checks -- a workflow-security failure on the release SHA must block the release"
  pass "CWS-5: workflow-security is a required release-eligibility check"
}

# --- CWS-6: guards that cannot block a merge are not gates -------------------

cws6_required_set() {
  local names m
  names="$(elig_names | tr '\n' ' ')"
  [[ -n "${names}" ]] ||
    fail "CWS-6: no required_checks could be parsed from hack/release/verify-eligibility.sh -- the declared required set is the only record of which contexts gate a merge"
  # The three eligibility-listed names are checked against real data; dependency-review's
  # presence is a property of ci.yaml (checked by CWS-3's has() probe), so listing it here
  # would be tautological.
  for m in lint vulncheck workflow-security; do
    if ! in_allowlist "${m}" ${names}; then
      fail "CWS-6: the declared required set no longer lists '${m}' (declared: ${names}) -- the ruleset itself is checked by the operator (post-merge evidence), but the set that gates merges must keep listing the guards' home jobs"
    fi
  done
  pass "CWS-6: the required set (dependency-review + ${names}) lists lint, vulncheck and workflow-security"
}

# Every mode a guard script's code recognises: bare always, plus `--self-test` when a
# non-comment line of the script mentions the flag. Deliberately NOT a generic flag parser:
# guard scripts embed foreign command lines in their fixtures (a mocked `curl --fail ...`
# in ci_fetch_lib_hardening_test.sh looks exactly like a mode), so a generic parse
# false-positives. The spec's "any other mode" is therefore enforced for the one flag mode
# the repo's convention uses (--self-test), and a script adopting a NEW flag mode must be
# re-reviewed -- it will not be pinned by accident. One mode per line: the caller loops
# with `read -r mode`.
guard_modes() {
  echo bare
  if grep -v '^[[:space:]]*#' "${1}" 2>/dev/null | grep -q -- '--self-test'; then
    echo --self-test
  fi
}

guard_tokens_in_run() {
  strip_comments "$1" | grep -oE 'hack/test/[A-Za-z0-9_*?./-]+' | sed 's/[.,;]$//' || true
}

cws6_guards() {
  local tmp
  tmp="$(mktemp -d)"

  [[ "$(yq eval '."on" | has("pull_request")' "${CI}")" == "true" ]] ||
    fail "CWS-6: ci.yaml no longer runs on pull_request -- the guards wired into it then never report a verdict on a PR, and every one of them is a no-op"

  local invocations="${tmp}/invocations.tsv"
  : >"${invocations}"

  local yaml_file yq_out job jname nsteps sidx run_body line tok mode_token
  local action_step action_nsteps action_line atok
  local yaml_files=()
  mapfile -t yaml_files < <(
    ls "${ROOT}"/.github/workflows/*.yaml \
      "${ROOT}"/.github/workflows/*.yml \
      "${ROOT}"/.github/actions/*/action.yml 2>/dev/null || true
  )
  for yaml_file in "${yaml_files[@]}"; do
    # A composite action has runs: and no jobs:; a parse error is still a hard failure.
    # A composite action can never be a required check, so a guard invoked from one would be
    # invisible to every gate below -- fail loudly instead of skipping silently.
    if [[ "$(yq eval '.jobs | type' "${yaml_file}" 2>&1)" != *map ]]; then
      action_nsteps="$(yq eval '.runs.steps // [] | length' "${yaml_file}")"
      for ((action_step = 0; action_step < action_nsteps; action_step++)); do
        action_line="$(yq eval ".runs.steps[${action_step}].run // \"\"" "${yaml_file}")"
        while IFS= read -r atok; do
          [[ -z "${atok}" ]] && continue
          fail "CWS-6: composite action ${yaml_file#"${ROOT}"/} runs step ${action_step} which invokes ${atok} -- a composite action is not a job and can never be a required check, so a guard reached this way can never block a merge; invoke the guard from a required job instead"
        done < <(guard_tokens_in_run "${action_line}")
      done
      continue
    fi
    yq_out="$(yq eval '.jobs | keys | .[]' "${yaml_file}" 2>&1)" ||
      fail "CWS-6: cannot parse jobs in ${yaml_file#"${ROOT}"/}: ${yq_out}"
    mapfile -t jobkeys <<<"${yq_out}"
    for job in "${jobkeys[@]}"; do
      [[ -n "${job}" ]] || continue
      jname="$(yq eval ".jobs[\"${job}\"].name // \"${job}\"" "${yaml_file}")"
      nsteps="$(yq eval ".jobs[\"${job}\"].steps | length" "${yaml_file}")"
      for ((sidx = 0; sidx < nsteps; sidx++)); do
        run_body="$(yq eval ".jobs[\"${job}\"].steps[${sidx}].run // \"\"" "${yaml_file}")"
        while IFS= read -r line; do
          [[ -z "${line}" ]] && continue
          mode_token="bare"
          case "${line}" in *'--self-test'*) mode_token="--self-test" ;; esac
          for tok in $(guard_tokens_in_run "${line}"); do
            printf '%s\t%s\t%s\t%s\n' "${tok}" "${mode_token}" "${jname}" "${yaml_file}" >>"${invocations}"
          done
        done <<<"$(strip_comments "${run_body}")"
      done
    done
  done

  if [[ ! -s "${invocations}" ]]; then
    fail "CWS-6: no workflow or action step invokes any hack/test guard script -- the gates are not wired anywhere"
  fi

  # Resolve globs to concrete scripts; record (script, mode, job) rows.
  local resolved="${tmp}/resolved.tsv"
  : >"${resolved}"
  local tok pattern match found src_yaml
  while IFS=$'\t' read -r tok mode_token jname src_yaml; do
    if [[ "${tok}" == *'*'* || "${tok}" == *'?'* ]]; then
      pattern="${tok#hack/test/}"
      found=0
      # The glob must be UNQUOTED here: the pattern is the whole point of the step.
      # shellcheck disable=SC2086
      for match in "${ROOT}"/hack/test/${pattern}; do
        [[ -e "${match}" ]] || continue
        printf '%s\t%s\t%s\t%s\n' "${match}" "${mode_token}" "${jname}" "${src_yaml}" >>"${resolved}"
        found=1
      done
      [[ "${found}" == "1" ]] ||
        fail "CWS-6: the glob hack/test/${pattern} invoked by job '${jname}' matches no script -- a renamed or deleted guard silently drops out of the gate"
    else
      printf '%s\t%s\t%s\t%s\n' "${ROOT}/${tok}" "${mode_token}" "${jname}" "${src_yaml}" >>"${resolved}"
    fi
  done <"${invocations}"

  # Every invocation must name an existing script; extra invocations from NON-required jobs
  # are permitted redundancy (the nightly and extended e2e suites legitimately re-run some
  # guards). What CWS-6 requires is that every (script, mode) pair is invoked by AT LEAST ONE
  # job in the required set -- a guard invoked ONLY from jobs outside the required set is not
  # a gate. The mode-coverage loop below runs over the required-job rows only.
  local script mode jname_row
  while IFS=$'\t' read -r script mode jname_row; do
    if [[ ! -f "${script}" ]]; then
      fail "CWS-6: guard script ${script#"${ROOT}"/} invoked by job '${jname_row}' does not exist"
    fi
  done <"${resolved}"

  # This meta-test must itself be run by ci.yaml in BOTH of its modes: a gate nobody runs
  # guards nothing, which is how this repo's earlier gates first reached CI.
  local self_script="${ROOT}/hack/test/ci_workflow_security_test.sh"
  for mode in bare --self-test; do
    if ! awk -F'\t' -v s="${self_script}" -v m="${mode}" '$1 == s && $2 == m { found = 1 } END { exit !found }' "${resolved}"; then
      fail "CWS-6: this meta-test (hack/test/ci_workflow_security_test.sh) is not itself run by ci.yaml in its '${mode}' mode -- wire it into the lint job next to ci_docs_gate_test.sh, plain and --self-test as separate steps"
    fi
  done

  # Every mode of every invoked script must be pinned to a required-job step. Coverage is
  # computed over the rows whose invoking job is in the required set AND whose workflow
  # actually runs on pull_request: required contexts gate PRs, so a job that only happens to
  # carry a required CONTEXT NAME in a workflow that never runs on a PR gates nothing (a
  # job named `lint` in a push-only workflow is not the lint gate).
  local required_rows="${tmp}/required_rows.tsv"
  : >"${required_rows}"
  local required_names
  required_names="dependency-review $(elig_names | tr '\n' ' ')"
  while IFS=$'\t' read -r script mode jname_row src_yaml; do
    if ! in_allowlist "${jname_row}" ${required_names}; then
      continue
    fi
    if [[ "$(yq eval '."on" | has("pull_request")' "${src_yaml}")" != "true" ]]; then
      continue
    fi
    printf '%s\t%s\t%s\n' "${script}" "${mode}" "${jname_row}" >>"${required_rows}"
  done <"${resolved}"

  cut -f1 "${resolved}" | sort -u >"${tmp}/scripts"
  while IFS= read -r script; do
    while IFS= read -r mode; do
      if ! awk -F'\t' -v s="${script}" -v m="${mode}" '$1 == s && $2 == m { found = 1 } END { exit !found }' "${required_rows}"; then
        fail "CWS-6: guard script ${script#"${ROOT}"/} runs in CI but its '${mode}' mode is not invoked by any step of a required job (required: ${required_names}) -- a guard that cannot block a merge is not a gate, and a mode that CI does not run can rot unnoticed"
      fi
    done < <(guard_modes "${script}")
  done <"${tmp}/scripts"

  # Guards CI runs INDIRECTLY through hack/docs/verify.sh (the Docs workflow's task target)
  # are still CI-run guards: every hack/test token in that script must also be pinned by a
  # required-job row, or a new guard added only to verify.sh runs solely in a non-required
  # job and no gate ever notices. The 15 docs-side guards were wired into lint by hand for
  # exactly this reason.
  local verify_sh="${ROOT}/hack/docs/verify.sh"
  if [[ -f "${verify_sh}" ]]; then
    local vtok
    while IFS= read -r vtok; do
      [[ -z "${vtok}" ]] && continue
      local vscript="${ROOT}/${vtok}"
      [[ -f "${vscript}" ]] || continue
      if ! awk -F'\t' -v s="${vscript}" '$1 == s { found = 1 } END { exit !found }' "${required_rows}"; then
        fail "CWS-6: guard script ${vtok} is invoked by hack/docs/verify.sh but has no required-job row -- the Docs workflow's docs:verify is not a required check, so a guard whose ONLY invocation is inside that script can never block a merge (wire it into the required lint job too)"
      fi
    done < <(guard_tokens_in_run "$(cat "${verify_sh}")" || true)
  fi

  pass "CWS-6: every hack/test guard CI runs, in every mode its code recognises, is pinned by a step of a required job"
}

# --- the whole contract ------------------------------------------------------

# Schema ratchet (added after the round-one review found a dangling name-only step in ci.yaml,
# which GitHub rejects at load time and which every key allowlist happily accepted): every step
# of every job, and of every composite action, must declare run or uses.
schema_steps() {
  local yaml_file ftype yaml_files job jobkeys has_run has_uses sidx ns a__idx
  local yaml_files=()
  mapfile -t yaml_files < <(
    ls "${ROOT}"/.github/workflows/*.yaml \
      "${ROOT}"/.github/workflows/*.yml \
      "${ROOT}"/.github/actions/*/action.yml 2>/dev/null || true
  )
  for yaml_file in "${yaml_files[@]}"; do
    ftype="$(yq eval '.jobs | type' "${yaml_file}" 2>&1)" ||
      fail "schema: cannot parse ${yaml_file#"${ROOT}"/}: ${ftype}"
    if [[ "${ftype}" == *map ]]; then
      mapfile -t jobkeys < <(yq eval '.jobs | keys | .[]' "${yaml_file}")
      for job in "${jobkeys[@]}"; do
        for sidx in $(seq 0 $(($(yq eval ".jobs[\"${job}\"].steps | length" "${yaml_file}") - 1))); do
          has_run="$(yq eval ".jobs[\"${job}\"].steps[${sidx}].run != null" "${yaml_file}")"
          has_uses="$(yq eval ".jobs[\"${job}\"].steps[${sidx}].uses != null" "${yaml_file}")"
          if [[ "${has_run}" != "true" && "${has_uses}" != "true" ]]; then
            fail "schema: step ${sidx} of job '${job}' in ${yaml_file#"${ROOT}"/} declares neither run nor uses -- GitHub rejects the whole workflow at load, so not one required context would ever report; a bare step header is a leftover from a bad edit and the workflow silently stops existing"
          fi
        done
      done
    else
      # A composite action: its runs.steps live outside .jobs, and GitHub rejects the ACTION
      # the same way, so every workflow using the action fails at load.
      ns="$(yq eval '.runs.steps // [] | length' "${yaml_file}")"
      for ((a__idx = 0; a__idx < ns; a__idx++)); do
        has_run="$(yq eval ".runs.steps[${a__idx}].run != null" "${yaml_file}")"
        has_uses="$(yq eval ".runs.steps[${a__idx}].uses != null" "${yaml_file}")"
        if [[ "${has_run}" != "true" && "${has_uses}" != "true" ]]; then
          fail "schema: step ${a__idx} of the composite action ${yaml_file#"${ROOT}"/} declares neither run nor uses -- GitHub rejects the action at load, so every workflow that uses it fails to start and not one required context would ever report"
        fi
      done
    fi
  done
  pass "schema: every step in every workflow and composite action declares run or uses"
}

check_all() {
  local root="$1"
  CI="${root}/.github/workflows/ci.yaml"
  SMOKE="${root}/.github/workflows/e2e-smoke.yaml"
  ZIZMOR_CFG="${root}/.github/zizmor.yml"
  INSTALLER="${root}/hack/install-zizmor.sh"
  ELIG="${root}/hack/release/verify-eligibility.sh"
  cws1_job
  cws1_invocation
  cws1_version_pin
  cws2_suppressions
  cws3_job
  cws3_not_eligibility
  cws4_concurrency
  cws5_trigger
  cws5_eligibility
  cws6_required_set
  cws6_guards
  schema_steps
}

SCRATCH=""

if [[ "${MODE}" == "check" ]]; then
  check_all "${ROOT}"
  echo "All workflow-security meta-tests passed."
  exit 0
fi

# --- CWS-7: the self-test ----------------------------------------------------
#
# --self-test copies the tree-relevant files into a scratch root, applies one mutant at a
# time, runs this gate against the copy and asserts the gate REDS WITH THE MESSAGE of the
# assertion the mutation was built to trip -- a non-zero exit alone is not evidence (an
# unparseable file exits non-zero too). One unmutated no-op copy must pass.

REAL_ROOT="${ROOT}"

scratch_add() {
  SCRATCH="${SCRATCH:+${SCRATCH} }$1"
}

make_copy() {
  local dst="$1"
  rm -rf "${dst}"
  mkdir -p "${dst}"
  cp -R "${REAL_ROOT}/.github" "${dst}/.github"
  mkdir -p "${dst}/hack"
  cp "${REAL_ROOT}/hack/install-zizmor.sh" "${dst}/hack/install-zizmor.sh"
  cp -R "${REAL_ROOT}/hack/test" "${dst}/hack/test"
  cp -R "${REAL_ROOT}/hack/release" "${dst}/hack/release"
  cp -R "${REAL_ROOT}/hack/lib" "${dst}/hack/lib"
  cp -R "${REAL_ROOT}/hack/docs" "${dst}/hack/docs"
}

trap 'rm -rf ${SCRATCH:-} 2>/dev/null' EXIT

mutant_rejected() {
  local label="$1" sentinel="$2"
  shift 2
  local copy
  copy="$(mktemp -d)"
  scratch_add "${copy}"
  make_copy "${copy}"
  (cd "${copy}" && "$@") ||
    fail "self-test: the mutation step for '${label}' failed -- the mutant was not applied, so nothing was tested"
  local out rc
  out="$(bash "${SELF}" "${copy}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  echo "${out}" | grep -q -- "${sentinel}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected the assertion '${sentinel}', got: $(echo "${out}" | head -2 | tr '\n' ' ')"
  rm -rf "${copy}"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

# CWS-7 no-op control: an unmutated copy must pass. This also proves the checks read the copy
# they are pointed at and not a hardcoded path.
CONTROL="$(mktemp -d)"
scratch_add "${CONTROL}"
make_copy "${CONTROL}"
if out="$(bash "${SELF}" "${CONTROL}" 2>&1)"; then
  :
else
  fail "self-test: the no-op control failed on an unmutated copy -- the checks themselves are broken: $(echo "${out}" | head -3 | tr '\n' ' ')"
fi
rm -rf "${CONTROL}"
pass "self-test: no-op control passes on an unmutated copy"

# sed mutant: replacement carries no dollar; the probe proves the mutation landed.
sed_mutant_rejected() {
  local label="$1" sentinel="$2" file="$3" sed_script="$4" probe="$5"
  mutant_rejected "${label}" "${sentinel}" bash -c '
    set -euo pipefail
    sed -i.bak "$2" "$1" && rm -f "$1.bak"
    grep -qF -- "$3" "$1" || { echo "mutant did not apply"; exit 1; }
  ' _ "${file}" "${sed_script}" "${probe}"
}

# yq assignment mutant: a plain string literal, not a sub() template, so no $-expansion
# hazard applies (METHOD-MUTHARNESS-02). The probe proves the mutation landed.
yq_mutant_rejected() {
  local label="$1" sentinel="$2" file="$3" expr="$4" probe="$5"
  mutant_rejected "${label}" "${sentinel}" bash -c '
    set -euo pipefail
    yq eval "$2" "$1" > "$1.mutant" && mv "$1.mutant" "$1"
    grep -qF -- "$3" "$1" || { echo "mutant did not apply"; exit 1; }
  ' _ "${file}" "${expr}" "${probe}"
}

# --- CWS-1 mutants: the gate cannot be silenced by omission ------------------

sed_mutant_rejected "CWS-1 audit without --offline" "must run exactly" \
  ".github/workflows/ci.yaml" \
  "s/zizmor --offline/zizmor/" \
  "run: zizmor --no-progress --min-severity=high"

sed_mutant_rejected "CWS-1 audit with a loosened --min-severity" "must run exactly" \
  ".github/workflows/ci.yaml" \
  "s/--min-severity=high/--min-severity=low/" \
  "run: zizmor --offline --no-progress --min-severity=low"

sed_mutant_rejected "CWS-1 audit with a renamed config" "must run exactly" \
  ".github/workflows/ci.yaml" \
  "s|--config .github/zizmor.yml|--config .github/zizmor-renamed.yml|" \
  "--config .github/zizmor-renamed.yml"

sed_mutant_rejected "CWS-1 audit scope narrowed to workflows/" "must run exactly" \
  ".github/workflows/ci.yaml" \
  "s|--config .github/zizmor.yml .github/|--config .github/zizmor.yml .github/workflows/|" \
  "run: zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/workflows/"

yq_mutant_rejected "CWS-1 job with a skip switch" "is a skip switch" \
  ".github/workflows/ci.yaml" \
  '.jobs["workflow-security"].if = "false"' \
  'if: "false"'

mutant_rejected "CWS-1 job removed" "has no workflow-security job" bash -c '
  set -euo pipefail
  yq eval "del(.jobs[\"workflow-security\"])" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  if yq eval ".jobs | has(\"workflow-security\")" .github/workflows/ci.yaml | grep -q true; then
    echo "mutant did not apply"; exit 1
  fi
'

sed_mutant_rejected "CWS-1 install step without the version pin" "is not ZIZMOR_VERSION" \
  ".github/workflows/ci.yaml" \
  's/ZIZMOR_VERSION: "1.30.1"/ZIZMOR_VERSION_REMOVED: "1.30.1"/' \
  "ZIZMOR_VERSION_REMOVED"

# CWS-1 round-two ratchets: the install step may not be swapped out, and a workflow-security
# step may not carry a shell override or a second env binding (skip switches).
mutant_rejected "CWS-1 install step replaced" "no workflow-security step runs hack/install-zizmor.sh" bash -c '
  set -euo pipefail
  yq eval "(.jobs[\"workflow-security\"].steps[] | select(.run | test(\"install-zizmor.sh\")) | .run) = \"pipx install zizmor\"" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  grep -q "pipx install zizmor" .github/workflows/ci.yaml || { echo "mutant did not apply"; exit 1; }
'

yq_mutant_rejected "CWS-1 audit step with a shell override" "is a skip switch" \
  ".github/workflows/ci.yaml" \
  '.jobs["workflow-security"].steps[2].shell = "true {0}"' \
  "shell: true {0}"

yq_mutant_rejected "CWS-1 install step with a second env binding" "is a skip switch" \
  ".github/workflows/ci.yaml" \
  '.jobs["workflow-security"].steps[1].env.KOLLECT_FORCE_SHA256 = "e65324f4430c2717591937edcec90ccbefaf14c174f8ec9415e03ca875b46e1b"' \
  "KOLLECT_FORCE_SHA256"

# --- CWS-2 mutants: bare suppressions ----------------------------------------

mutant_rejected "CWS-2 bare suppression" "has no comment directly above it" bash -c '
  set -euo pipefail
  perl -0pi -e "s/rules: \{\}/rules:\n  cache-poisoning:\n    ignore:\n      - .github\/workflows\/release.yaml/" .github/zizmor.yml
  grep -q "cache-poisoning" .github/zizmor.yml || { echo "mutant did not apply"; exit 1; }
'

mutant_rejected "CWS-2 inline zizmor: ignore" "inline zizmor suppression" bash -c '
  set -euo pipefail
  perl -0pi -e "s/(run: bash hack\/test\/demo_task_aliases_test.sh)/\$1 # zizmor: ignore[template-injection]/" .github/workflows/ci.yaml
  grep -q "zizmor: ignore" .github/workflows/ci.yaml || { echo "mutant did not apply"; exit 1; }
'

# CWS-2 per-item: a justified RULE key does not justify an uncommented ignore entry.
mutant_rejected "CWS-2 justified rule with a bare ignore entry" "has no comment directly above it stating why" bash -c '
  set -euo pipefail
  perl -0pi -e "s/rules: \{\}/rules:\n  # why: fixture -- a block-level comment does not reach the entry\n  cache-poisoning:\n    ignore:\n      - .github\/workflows\/release.yaml/" .github/zizmor.yml
  grep -q "cache-poisoning" .github/zizmor.yml || { echo "mutant did not apply"; exit 1; }
'

# CWS-2 approval-3 ratchet: a policy switch under a rule (disable: true) switches the audit
# off with no list to inspect, so it needs the block comment too.
mutant_rejected "CWS-2 uncommented disable policy" "has no comment directly above it stating why" bash -c '
  set -euo pipefail
  perl -0pi -e "s/rules: \{\}/rules:\n  template-injection:\n    disable: true/" .github/zizmor.yml
  grep -q "disable" .github/zizmor.yml || { echo "mutant did not apply"; exit 1; }
'

# CWS-3 approval-round ratchets: step-level mute switches and a config-file policy bypass.
yq_mutant_rejected "CWS-3 continue-on-error on the review step" "is not one of (name run uses with env)" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].steps[1].continue-on-error = "true"' \
  'continue-on-error: "true"'

yq_mutant_rejected "CWS-3 policy moved into a config file" "is not one of the reviewed policy inputs" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].steps[1].with["config-file"] = ".github/dep-review.yml"' \
  "config-file"

# CWS-6: a guard whose only invocation is inside hack/docs/verify.sh must red.
mutant_rejected "CWS-6 guard reachable only through verify.sh" "has no required-job row" bash -c '
  set -euo pipefail
  cat > hack/test/zz_verify_only_test.sh <<"SH"
#!/usr/bin/env bash
echo ok
SH
  perl -0pi -e "s/(bash hack\/test\/docs_removed_api_fields_test.sh\n)/\$1bash hack\/test\/zz_verify_only_test.sh\n/" hack/docs/verify.sh
  grep -q "zz_verify_only_test" hack/docs/verify.sh || { echo "mutant did not apply"; exit 1; }
'

# CWS-2 approval-round ratchets: flow style and an uncommented 4-space rule key both bypass a
# line-level parser, so the suppression check is yq-based; each shape must still red.
mutant_rejected "CWS-2 flow-style suppression" "must be written in block style" bash -c '
  set -euo pipefail
  perl -0pi -e "s/rules: \{\}/rules: \{cache-poisoning: \{ignore: [.github\/workflows\/release.yaml]\}\}/" .github/zizmor.yml
  grep -q "cache-poisoning" .github/zizmor.yml || { echo "mutant did not apply"; exit 1; }
'

# CWS-1 approval-round ratchets: the required context name and its uniqueness.
yq_mutant_rejected "CWS-1 job display name renamed" "expected .workflow-security." \
  ".github/workflows/ci.yaml" \
  '.jobs["workflow-security"].name = "workflow-security-renamed"' \
  "workflow-security-renamed"

mutant_rejected "CWS-1 decoy job with the same display name" "a decoy check that reports green" bash -c '
  set -euo pipefail
  cat > .github/workflows/zzz-decoy.yaml <<"YAML"
name: decoy
on: [push]
jobs:
  decoy-job:
    name: workflow-security
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YAML
  grep -q "workflow-security" .github/workflows/zzz-decoy.yaml || { echo "mutant did not apply"; exit 1; }
'

# --- CWS-3 mutants: the dependency-review job --------------------------------

yq_mutant_rejected "CWS-3 dependency-review without the pull_request if" "must not run on main" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].if = "always()"' \
  "if: always()"

yq_mutant_rejected "CWS-3 fail-on-severity raised without a why" "measured trial" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].steps[1].with["fail-on-severity"] = "critical"' \
  "fail-on-severity"

yq_mutant_rejected "CWS-3 deny-licenses used" "is not one of the reviewed policy inputs" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].steps[1].with["deny-licenses"] = "GPL-3.0"' \
  "deny-licenses"

sed_mutant_rejected "CWS-3 dependency-review added to release eligibility" "structurally unpassable" \
  "hack/release/verify-eligibility.sh" \
  "s/^\([[:space:]]*\)gitleaks/\1dependency-review gitleaks/" \
  "dependency-review gitleaks"

yq_mutant_rejected "CWS-3 warn-only dependency-review" "is not one of the reviewed policy inputs" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].steps[1].with["warn-only"] = "true"' \
  "warn-only"

yq_mutant_rejected "CWS-3 continue-on-error on dependency-review" "is not one of (name if runs-on steps)" \
  ".github/workflows/ci.yaml" \
  '.jobs["dependency-review"].continue-on-error = "true"' \
  'continue-on-error: "true"'

# --- CWS-4 mutants: the concurrency contract ---------------------------------

yq_mutant_rejected "CWS-4 group keyed by ref" "share one group and lose the middle" \
  ".github/workflows/ci.yaml" \
  '.concurrency.group = "${{ github.ref }}"' \
  "group: \${{ github.ref }}"

mutant_rejected "CWS-4 group without the workflow prefix" "share one group and lose the middle" bash -c '
  set -euo pipefail
  yq eval ".concurrency.group = \"\${{ github.event.pull_request.number || github.sha }}\"" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  if yq eval ".concurrency.group" .github/workflows/ci.yaml | grep -q "github.workflow"; then
    echo "mutant did not apply"; exit 1
  fi
'

yq_mutant_rejected "CWS-4 cancel-in-progress unconditional" "push-to-main run is never cancelled" \
  ".github/workflows/e2e-smoke.yaml" \
  '.concurrency["cancel-in-progress"] = "true"' \
  'cancel-in-progress: "true"'

# --- CWS-5 mutants: the push trigger and eligibility -------------------------

mutant_rejected "CWS-5 push trigger removed" "lost its push trigger" bash -c '
  set -euo pipefail
  yq eval "del(.\"on\".push)" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  if yq eval ".\"on\" | has(\"push\")" .github/workflows/ci.yaml | grep -q true; then
    echo "mutant did not apply"; exit 1
  fi
'

sed_mutant_rejected "CWS-5 push branch is not main" "no longer lists main" \
  ".github/workflows/ci.yaml" \
  "s/branches: \[main\]/branches: [develop]/" \
  "branches: [develop]"

mutant_rejected "CWS-5 workflow-security dropped from eligibility" "missing from hack/release" bash -c '
  set -euo pipefail
  sed -i.bak "s/ workflow-security//" hack/release/verify-eligibility.sh && rm -f hack/release/verify-eligibility.sh.bak
  if grep -q workflow-security hack/release/verify-eligibility.sh; then
    echo "mutant did not apply"; exit 1
  fi
'

# --- CWS-6 mutants: guards that cannot block a merge -------------------------

mutant_rejected "CWS-6 required set shrinks (lint dropped)" "no longer lists 'lint'" bash -c '
  set -euo pipefail
  sed -i.bak "s/ lint / /" hack/release/verify-eligibility.sh && rm -f hack/release/verify-eligibility.sh.bak
  if grep -q " lint " hack/release/verify-eligibility.sh; then
    echo "mutant did not apply"; exit 1
  fi
'

mutant_rejected "CWS-6 guard only in a non-required job" "runs in CI but its" bash -c '
  set -euo pipefail
  cat > hack/test/zz_free_guard_test.sh <<"SH"
#!/usr/bin/env bash
echo ok
SH
  cat > .github/workflows/zzz-guard-mutant.yaml <<"YAML"
name: guard-mutant-not-required
on: [push]
jobs:
  docs-only-note:
    runs-on: ubuntu-latest
    steps:
      - name: run a guard outside the required set
        run: bash hack/test/zz_free_guard_test.sh
YAML
  grep -q "zz_free_guard_test" .github/workflows/zzz-guard-mutant.yaml || { echo "mutant did not apply"; exit 1; }
'

mutant_rejected "CWS-6 self-test mode not pinned" "mode is not invoked by any step of a required job" bash -c '
  set -euo pipefail
  cat > hack/test/zz_mode_probe_test.sh <<"SH"
#!/usr/bin/env bash
if [[ "${1:-}" == "--self-test" ]]; then
  echo "self-test ok"
  exit 0
fi
echo ok
SH
  yq eval ".jobs.lint.steps += {\"name\": \"mode probe\", \"run\": \"bash hack/test/zz_mode_probe_test.sh\"}" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  grep -q "zz_mode_probe_test" .github/workflows/ci.yaml || { echo "mutant did not apply"; exit 1; }
'

mutant_rejected "CWS-6 this gate not itself run by ci.yaml" "is not itself run by ci.yaml" bash -c '
  set -euo pipefail
  yq eval "(.jobs.lint.steps |= map(select(.run // \"\" | test(\"ci_workflow_security_test.sh\") | not)))" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  if yq eval ".jobs.lint.steps | map(.run) | join(\" \")" .github/workflows/ci.yaml | grep -q "ci_workflow_security_test"; then
    echo "mutant did not apply"; exit 1
  fi
'

# schema ratchet: a name-only step (the round-one CRITICAL) must red the meta-test.
mutant_rejected "schema dangling name-only step" "declares neither run nor uses" bash -c '
  set -euo pipefail
  yq eval ".jobs.lint.steps += {\"name\": \"dangling leftover step\"}" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  yq eval ".jobs.lint.steps[-1].name" .github/workflows/ci.yaml | grep -q "dangling leftover" || { echo "mutant did not apply"; exit 1; }
'

# CWS-6 ratchet: a guard invoked from a composite action can never be a required check.
mutant_rejected "CWS-6 guard in a composite action" "is not a job and can never be a required check" bash -c '
  set -euo pipefail
  yq eval ".runs.steps += {\"name\": \"guard inside an action (mutant)\", \"run\": \"bash hack/test/dev_mise_pin_drift_test.sh\"}" .github/actions/go-cache/action.yml > m.yaml && mv m.yaml .github/actions/go-cache/action.yml
  grep -q "dev_mise_pin_drift_test" .github/actions/go-cache/action.yml || { echo "mutant did not apply"; exit 1; }
'

# CWS-6: a job that carries a required CONTEXT NAME in a workflow that never runs on PRs is
# not the required gate -- required contexts gate PRs.
mutant_rejected "CWS-6 required-named job without a pull_request trigger" "runs in CI but its" bash -c '
  set -euo pipefail
  cat > hack/test/zz_fake_lint_guard_test.sh <<"SH"
#!/usr/bin/env bash
echo ok
SH
  cat > .github/workflows/zzz-fake-lint.yaml <<"YAML"
name: fake-lint
on: [push]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: run a guard from a job with a required context name
        run: bash hack/test/zz_fake_lint_guard_test.sh
YAML
  grep -q "zz_fake_lint_guard_test" .github/workflows/zzz-fake-lint.yaml || { echo "mutant did not apply"; exit 1; }
'

echo "All workflow-security self-tests passed."
