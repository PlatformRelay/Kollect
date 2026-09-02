#!/usr/bin/env bash
# DIST-AH-03: the release workflow derives the Helm chart's OCI coordinate EXACTLY ONCE.
#
# ADR-0709 moves the chart from ghcr.io/<owner>/kollect -- where it shared an OCI
# repository with the controller image, the DR-FIND-07 tag convention -- to
# ghcr.io/<owner>/charts/kollect. The controller image does NOT move: its digest is
# pinned immutably in already-merged OLM bundles.
#
# The defect this gate exists to prevent is not "the path is wrong" but "the path is
# derived three times". Before DIST-AH-03 the coordinate was rebuilt independently by
# (1) the `chart` step for `helm push`, (2) the publish step for `cosign sign`, and
# (3) the Artifact Hub metadata step -- which built it from env.IMAGE_NAME, the *image*
# variable, and is `continue-on-error: true`, so stranding it at the old path loses
# Verified Publisher while the release still goes green. Editing one copy left the
# others silently pointing elsewhere.
#
# WHY THIS GATE SIMULATES INSTEAD OF GREPPING
# -------------------------------------------
# Text assertions over a workflow are easy to pass vacuously: a grep for `charts/` is
# satisfied by a comment, and "the publish step mentions steps.chart.outputs" is
# satisfied by a step that mentions it and then ignores it. So this gate EXECUTES the
# three `run:` bodies against stub helm/cosign/oras binaries and asserts the references
# those tools actually receive.
#
# That is only possible because every untrusted value reaches those bodies through
# `env:` rather than `${{ }}` interpolation (githubactions:S7636) -- so the executability
# assertion and the injection-safety assertion are the same assertion, and each reinforces
# the other. The simulation runs with a SYNTHETIC registry, owner and chart name
# ("registry.example.test", "ExampleOrg", "widget"), so any hardcoded `ghcr.io`,
# `platformrelay` or `kollect` inside a consumer is caught: the stub would receive the
# hardcoded literal instead of the derived one. A second, production-shaped run then
# pins the exact coordinate the ADR names.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/release.yaml"

fail() {
  printf 'dist release chart path: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${WORKFLOW}" ]] || fail "${WORKFLOW} is missing"

# Structural inspection only. A free-text scan cannot tell a step's `run:` body from a
# comment about it, and cannot read the `env:` map that carries every value into it.
command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the release job step graph"

CHART_STEP_SEL='.id == "chart"'
PUBLISH_STEP_SEL='.name == "Publish and sign Helm chart (OCI)"'
METADATA_STEP_SEL='.id == "push-artifacthub-metadata"'
GUARD_STEP_SEL='.name == "Guard against image/chart GHCR tag collision (DR-FIND-07)"'
NOTES_STEP_SEL='.name == "Assemble release notes"'

step_count() {
  yq eval "[.jobs.release.steps[] | select($1)] | length" "${WORKFLOW}"
}

step_run() {
  yq eval "[.jobs.release.steps[] | select($1) | .run // \"\"] | .[0] // \"\"" "${WORKFLOW}"
}

step_attr() {
  yq eval "[.jobs.release.steps[] | select($1) | .$2] | .[0]" "${WORKFLOW}"
}

# key<TAB>value for each entry of a step's `env:` map. Read as lines in bash rather than
# matched inside yq on purpose: yq's string `==` GLOB-matches, so a value containing a
# metacharacter would compare surprisingly.
step_env_pairs() {
  yq eval "[.jobs.release.steps[] | select($1)] | .[0].env // {} | to_entries | .[] | .key + \"\t\" + (.value | tostring)" "${WORKFLOW}"
}

# ---------------------------------------------------------------------------
# 1. Non-vacuity: every step the assertions below select must exist, exactly once.
#    Without this a renamed or deleted step makes each later `select(...)` return an
#    empty list, and every assertion over it passes while asserting nothing.
# ---------------------------------------------------------------------------
while IFS='|' read -r label sel; do
  [[ -n "${label}" ]] || continue
  count="$(step_count "${sel}")"
  case "${count}" in
  1) ;;
  0) fail "the release job has no ${label} step (selector: ${sel}) -- every assertion about it below would pass vacuously" ;;
  *) fail "the release job has ${count} ${label} steps -- keep exactly one so the assertions below cannot lock onto the first match" ;;
  esac
done <<EOF
chart-coordinate (id: chart)|${CHART_STEP_SEL}
chart publish|${PUBLISH_STEP_SEL}
Artifact Hub metadata push|${METADATA_STEP_SEL}
DR-FIND-07 collision guard|${GUARD_STEP_SEL}
release notes assembly|${NOTES_STEP_SEL}
EOF

pass "release job declares exactly one of each step this gate inspects"

# ---------------------------------------------------------------------------
# 2. Workflow-level constants. The chart name is the last path segment of the chart
#    repository and ADR-0709 forbids changing it (`helm push` appends it, Artifact Hub
#    requires `.../chart-name`, and `.Chart.Name` feeds the immutable Deployment
#    selector). Read it from the workflow rather than hardcoding it here, so the
#    simulation below exercises the workflow's own value.
# ---------------------------------------------------------------------------
WF_REGISTRY="$(yq eval '.env.REGISTRY // ""' "${WORKFLOW}")"
WF_CHART_NAME="$(yq eval '.env.CHART_NAME // ""' "${WORKFLOW}")"
WF_IMAGE_NAME="$(yq eval '.env.IMAGE_NAME // ""' "${WORKFLOW}")"

[[ -n "${WF_REGISTRY}" ]] || fail "the workflow must define env.REGISTRY"
[[ -n "${WF_CHART_NAME}" ]] ||
  fail "the workflow must define env.CHART_NAME -- the chart's name is a separate concept from env.IMAGE_NAME (the controller image), and reusing IMAGE_NAME for the chart is the exact confusion ADR-0709 unpicks"
[[ -n "${WF_IMAGE_NAME}" ]] || fail "the workflow must still define env.IMAGE_NAME for the controller image"

pass "workflow declares env.REGISTRY, env.CHART_NAME and env.IMAGE_NAME as separate constants"

# env.CHART_NAME is not a free-form label: it is the chart's real name in two places that
# both fail hard and late if it drifts.
#   * The publish step builds CHART_PKG as dist/${CHART_NAME}-${VERSION}.tgz, and
#     `helm package` names that tarball from Chart.yaml's `name`. A divergence points the
#     push at a file that was never produced -- discovered mid-release, after the images
#     are pushed and signed.
#   * `helm push` appends the chart's own name to the target, so the last segment of the
#     chart repository is Chart.yaml's `name` whatever this variable says. Artifact Hub
#     also requires the URL to end in the chart name.
# The chart is located by SEARCHING charts/ rather than by looking under
# charts/${CHART_NAME}: deriving the path from the value under test would make this
# assertion circular and it would pass on any consistent-but-wrong pair.
mapfile -t CHART_YAMLS < <(find "${ROOT}/charts" -mindepth 2 -maxdepth 2 -name Chart.yaml | sort)
[[ "${#CHART_YAMLS[@]}" -eq 1 ]] ||
  fail "expected exactly one chart under charts/, found ${#CHART_YAMLS[@]} -- with more than one, env.CHART_NAME cannot be anchored unambiguously and this gate must be taught which chart the release publishes"
DECLARED_CHART_NAME="$(yq eval '.name // ""' "${CHART_YAMLS[0]}")"
[[ -n "${DECLARED_CHART_NAME}" ]] || fail "${CHART_YAMLS[0]} declares no name"
[[ "${WF_CHART_NAME}" == "${DECLARED_CHART_NAME}" ]] ||
  fail "env.CHART_NAME is '${WF_CHART_NAME}' but ${CHART_YAMLS[0]} declares name '${DECLARED_CHART_NAME}'. The publish step builds dist/${WF_CHART_NAME}-<version>.tgz, which 'helm package' never produced, and 'helm push' would append '${DECLARED_CHART_NAME}' to the target regardless"

pass "env.CHART_NAME matches the name declared in ${CHART_YAMLS[0]#"${ROOT}"/}"

# ---------------------------------------------------------------------------
# 3. Simulation harness.
# ---------------------------------------------------------------------------
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

STUB_BIN="${TMP}/bin"
mkdir -p "${STUB_BIN}"

# `helm push <chart.tgz> <oci://target>` APPENDS the chart name (read from the package)
# to the target, and reports the reference it actually wrote as "Pushed: <ref>", with the
# manifest digest as "Digest: <sha256:...>". Both go to STDERR, and this stub keeps them
# there deliberately: the publish step captures `2>&1`, so a stub that wrote to stdout
# would keep passing after someone dropped that redirection.
#
# The Pushed line is DERIVED FROM ARGV, not from a fixed string the test chooses. That is
# load-bearing. With a fixed string, changing the workflow's push target from
# ${CHART_OCI} to oci://${CHART_REPOSITORY} -- which publishes the chart to
# .../charts/<chart>/<chart> -- left this gate green, because the stub kept reporting the
# correct reference and the assertion below was a substring match. Deriving it also means
# the workflow's OWN post-push assertion is exercised end to end on the happy path: the
# publish step only succeeds if helm's append actually lands under the derived repository.
#
# STUB_HELM_FORCE_REF overrides the derivation, so the negative case further down can
# simulate helm writing somewhere else entirely.
cat >"${STUB_BIN}/helm" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${STUB_LOG}/helm.args"

# Scan argv for the target and the package rather than reading $3 and $2 positionally.
# A benign `helm push --debug "${CHART_PKG}" "${CHART_OCI}"` would otherwise shift both
# and produce a false red whose message blamed the wrong thing -- fails closed, but
# misdirects, and a misdirecting gate costs more than the mutation it catches.
target=""
pkg=""
for arg in "$@"; do
  case "${arg}" in
  oci://*) target="${arg}" ;;
  *.tgz) pkg="${arg}" ;;
  esac
done

# Recorded ONLY for `push`, and APPENDED. Writing on every invocation with `>` would let a
# later `helm registry login` (or a second push) silently decide what the assertion reads;
# appending makes an unexpected extra push a visible failure instead.
if [[ "${1:-}" == "push" ]]; then
  printf '%s\n' "${target}" >>"${STUB_LOG}/helm.target"
fi

if [[ -n "${STUB_HELM_FORCE_REF:-}" ]]; then
  printf 'Pushed: %s\n' "${STUB_HELM_FORCE_REF}" >&2
else
  # <name>-<version>.tgz, the name `helm package` gives a chart. Simple semver only,
  # which is all the simulation uses.
  base="$(basename "${pkg}" .tgz)"
  printf 'Pushed: %s/%s:%s\n' "${target#oci://}" "${base%-*}" "${base##*-}" >&2
fi
printf 'Digest: %s\n' "${STUB_HELM_DIGEST}" >&2
STUB

cat >"${STUB_BIN}/cosign" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${STUB_LOG}/cosign.args"
STUB

cat >"${STUB_BIN}/oras" <<'STUB'
#!/usr/bin/env bash
# `oras login` reads the token from stdin; drain it so the upstream pipe does not SIGPIPE.
if [[ "${1:-}" == "login" ]]; then cat >/dev/null; fi
printf '%s\n' "$*" >>"${STUB_LOG}/oras.args"
STUB

chmod +x "${STUB_BIN}/helm" "${STUB_BIN}/cosign" "${STUB_BIN}/oras"

# Synthetic inputs. None of these strings appears anywhere in the workflow, so a
# hardcoded coordinate in any consumer produces a visibly wrong reference here.
SIM_REGISTRY="registry.example.test"
SIM_OWNER="ExampleOrg"
SIM_OWNER_LC="exampleorg"
SIM_CHART_NAME="widget"
SIM_VERSION="9.9.9"
SIM_ACTOR="sim-actor"
SIM_TOKEN="sim-token"
SIM_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# Set by resolve_step_env from the chart step's real outputs.
SIM_CHART_OCI=""
SIM_CHART_REPO=""

# Resolve one `${{ ... }}` expression from a step's env: map to a simulation value,
# into the global RESOLVED. Deliberately NOT a value-returning function called through
# `$( )`: fail()'s `exit 1` inside a command substitution only kills the subshell, so a
# rejected expression would print its diagnostic and the simulation would carry on with
# an empty value -- which is exactly how an early revision of this gate reported the
# wrong failure for the IMAGE_NAME mutation.
#
# Unknown expressions are a hard failure, not a default: a new env entry must be
# considered here deliberately, or the simulation would silently run with an empty
# value and still "pass".
RESOLVED=""
resolve_expr() {
  local raw="$1" step="$2" allow_owner="$3"
  RESOLVED=""
  case "${raw}" in
  '${{ steps.chart.outputs.oci }}') RESOLVED="${SIM_CHART_OCI}" ;;
  '${{ steps.chart.outputs.repository }}') RESOLVED="${SIM_CHART_REPO}" ;;
  '${{ steps.version.outputs.version }}') RESOLVED="${SIM_VERSION}" ;;
  '${{ env.REGISTRY }}') RESOLVED="${SIM_REGISTRY}" ;;
  '${{ env.CHART_NAME }}') RESOLVED="${SIM_CHART_NAME}" ;;
  '${{ github.actor }}') RESOLVED="${SIM_ACTOR}" ;;
  '${{ secrets.GITHUB_TOKEN }}') RESOLVED="${SIM_TOKEN}" ;;
  '${{ github.repository_owner }}')
    # THE central assertion of this gate. github.repository_owner is the raw material
    # of the chart coordinate: any step that takes it is deriving the coordinate for
    # itself, which is precisely the three-way drift DIST-AH-03 removes. Only the
    # `chart` step may have it.
    [[ "${allow_owner}" == "yes" ]] ||
      fail "'${step}' takes github.repository_owner -- that is a SECOND, independent derivation of the chart coordinate. Only the 'chart' step may derive it; every other consumer must read steps.chart.outputs.*"
    RESOLVED="${SIM_OWNER}"
    ;;
  '${{ env.IMAGE_NAME }}')
    fail "'${step}' references env.IMAGE_NAME -- that is the CONTROLLER IMAGE name (${WF_IMAGE_NAME}), not the chart. Building a chart coordinate from it is the DIST-AH-03 defect: the chart lands next to the image at ${WF_REGISTRY}/<owner>/${WF_IMAGE_NAME} instead of under charts/"
    ;;
  *)
    fail "'${step}' passes an env value this gate does not know how to simulate: '${raw}'. Add it to resolve_expr() deliberately -- defaulting it to empty would let the simulation below pass while asserting nothing"
    ;;
  esac
}

# Build an `env -i` argument list for one step: the workflow-level constants every step
# inherits, plus the step's own resolved env: map.
SIM_ENV=()
resolve_step_env() {
  local sel="$1" step="$2" allow_owner="$3"
  local k v
  SIM_ENV=(
    "PATH=${STUB_BIN}:/usr/local/bin:/usr/bin:/bin"
    "HOME=${TMP}"
    "REGISTRY=${SIM_REGISTRY}"
    "IMAGE_NAME=${WF_IMAGE_NAME}"
    "CHART_NAME=${SIM_CHART_NAME}"
    "GITHUB_OUTPUT=${TMP}/github_output"
    "STUB_LOG=${TMP}"
    "STUB_HELM_FORCE_REF=${STUB_HELM_FORCE_REF:-}"
    "STUB_HELM_DIGEST=${SIM_DIGEST}"
  )
  # `< <(...)` rather than a pipe: the loop must run in THIS shell so a rejected
  # expression's `exit 1` takes the whole gate down instead of one subshell.
  while IFS=$'\t' read -r k v; do
    [[ -n "${k}" ]] || continue
    resolve_expr "${v}" "${step}" "${allow_owner}"
    SIM_ENV+=("${k}=${RESOLVED}")
  done < <(step_env_pairs "${sel}")
}

# Run one step's `run:` body under `env -i`, so nothing leaks in from this test's own
# environment and an unset value is an empty value, not an accidental pass.
#
# $4/$5 redirect the STEP BODY's stdout/stderr, and nothing else. Callers must NOT wrap
# the call in their own redirection: run_step's diagnostics come from fail(), which writes
# to stderr and then exits, so a caller-side `>file 2>&1` captures them into a file the
# EXIT trap deletes -- leaving a bare rc=1 after a green `ok -` line, which reads as a
# broken gate rather than as the violation it is. A gate whose whole product is naming
# what broke cannot afford to swallow its own diagnostics.
#
# (`>file 2>&1`, in that order, is the capturing one: it points stdout at the file and
# THEN aims stderr at stdout's new destination. The reverse, `2>&1 >file`, aims stderr at
# stdout's OLD destination -- the terminal -- and leaves the diagnostics visible. The
# order is the whole bug, so it is spelled the capturing way here on purpose.)
#
# An OMITTED $4/$5 means "this gate's own stdout/stderr", and that is implemented by
# DUPLICATING the descriptor, never by re-opening a /dev/stdout path -- see
# open_step_fds below for why.

# Point fd 3 at a step body's stdout and fd 4 at its stderr. An empty path means "this
# gate's own descriptor", and it is reached with `>&1` / `>&2`, a dup.
#
# `exec 3>"/dev/stdout"` would look equivalent and is not. /dev/stdout is a symlink to
# /proc/self/fd/1, so opening it with `>` OPENS THE UNDERLYING FILE AFRESH, with O_TRUNC
# and its own file offset. When this gate's own output is a regular file --
# `bash hack/test/dist_release_chart_path_test.sh &> gate.log`, which is how a human
# debugs it -- every run_step call then truncated gate.log back to zero and wrote at
# offset 0 while the gate's real stdout carried on writing at its much larger offset,
# leaving a NUL-padded file with most of the `ok -` lines gone. `file` called the result
# `data` and plain `grep` refused to search it as binary, so the one artefact a human
# reads to find out what this gate said was the artefact the gate destroyed. A dup shares
# the offset and truncates nothing.
#
# CI pipes this gate's output rather than redirecting it to a file, so the defect never
# reached CI -- it was only ever aimed at the person debugging locally, which is exactly
# when a gate's diagnostics are the whole point.
open_step_fds() {
  local out="$1" err="$2"
  if [[ -n "${out}" ]]; then exec 3>"${out}"; else exec 3>&1; fi
  if [[ -n "${err}" ]]; then exec 4>"${err}"; else exec 4>&2; fi
}

close_step_fds() {
  exec 3>&- 4>&-
}

run_step() {
  local sel="$1" step="$2" allow_owner="$3"
  local out="${4:-}" err="${5:-}"
  local body script rc=0
  body="$(step_run "${sel}")"
  [[ -n "${body}" ]] || fail "'${step}' has an empty run: body"

  # No `${{ }}` in the body. Two things at once: (a) githubactions:S7636 -- untrusted
  # context and secrets must arrive through env: and be read as shell variables, never
  # rendered into the script text; (b) a body free of Actions interpolation is a body
  # this gate can execute, which is what makes every assertion below behavioural rather
  # than textual.
  if printf '%s' "${body}" | grep -Fq '${{'; then
    fail "'${step}' interpolates a \${{ ... }} expression into its run: body -- pass it via env: and read it as a shell variable instead (githubactions:S7636), which also keeps this gate able to execute the body"
  fi

  script="${TMP}/step.sh"
  printf '%s\n' "${body}" >"${script}"
  resolve_step_env "${sel}" "${step}" "${allow_owner}"
  # Opened AFTER resolve_step_env: a rejected env expression exits from inside it, and
  # opening an output path before that would truncate the caller's file on the way out.
  open_step_fds "${out}" "${err}"
  env -i "${SIM_ENV[@]}" bash "${script}" >&3 2>&4 || rc=$?
  close_step_fds
  return "${rc}"
}

output_value() {
  # Last write wins, matching how Actions collects GITHUB_OUTPUT.
  grep -E "^$1=" "${TMP}/github_output" 2>/dev/null | tail -1 | cut -d= -f2-
}

# ---------------------------------------------------------------------------
# 3a. Self-test of the harness's own redirection.
#
# Every assertion below reports through fail() and pass(), so this gate's product IS the
# text it writes to stdout/stderr. A run_step that truncates those descriptors destroys
# that product without failing anything: the gate still exits 0 and still prints every
# `ok -` line, they simply stop surviving in the file the human is reading. Nothing else
# in this file can notice that, so it is asserted here, against the real open_step_fds,
# by logging to a regular file exactly the way a human does.
# ---------------------------------------------------------------------------
# The two descriptors are exercised against SEPARATE files, with the other one left
# alone, so a regression on one side cannot be reported as the other's. With both aimed
# at one log (`&>`), re-opening either would blank it and the diagnostic would have to
# guess which.
#
# $1 log, $2 the human name of the side, $3 the fd run_step gives that side.
assert_redirect_intact() {
  local log="$1" side="$2" fdnum="$3"
  local sentinels nuls dup
  dup=$((fdnum - 2)) # fd 3 mirrors this gate's fd 1, fd 4 its fd 2
  # Counted rather than `grep -q`-ed (GATE-SIGPIPE-01), read straight from the file
  # rather than through a pipe, and with -a because the failing case makes it look binary.
  sentinels="$(grep -ac 'redirect-selftest sentinel' "${log}" || true)"
  nuls="$(tr -dc '\000' <"${log}" | wc -c)"

  # One assertion, two witnesses, both numbers in the message: re-opening the descriptor
  # LOSES the lines written before it and leaves NUL padding where the two file offsets
  # diverged. Reporting only "lines are missing" would leave the reader guessing why the
  # log is also unsearchable.
  [[ "${sentinels}" -eq 20 && "${nuls}" -eq 0 ]] ||
    fail "run_step's default ${side} redirection did not leave this gate's own ${side} intact: ${sentinels} of the 20 lines written before it survived, and the log carries ${nuls} NUL bytes. An omitted \$4/\$5 must be reached with a dup ('exec ${fdnum}>&${dup}'), never by re-opening a /dev/${side} path -- that path is /proc/self/fd/${dup}, so '>' re-opens the underlying file with O_TRUNC and a fresh offset. Logged to a regular file ('bash ${BASH_SOURCE[0]##*/} &> gate.log') every simulated step then chops the log back to nothing, 'file' calls the result 'data', and plain grep refuses to search it as binary"
  grep -aFqx "redirect-selftest via-fd${fdnum}" "${log}" ||
    fail "a step body's ${side} did not reach this gate's ${side} at all -- run_step's default redirection must forward it, or a simulated step's output vanishes instead of being reported"
  grep -aFqx 'redirect-selftest tail' "${log}" ||
    fail "this gate's own ${side} was unusable after close_step_fds -- closing the step descriptors must leave fd 1 and fd 2 as they were"
}

# This gate's real stdout/stderr are parked on fd 8/9 for the duration and restored
# before anything is asserted. open_step_fds and close_step_fds use `exec`, which acts on
# the whole shell, so a regression in them could otherwise take this gate's OWN
# stdout/stderr with it -- and then the assertion that noticed would exit 1 with its
# diagnostic sent nowhere. A self-test of the diagnostic channel cannot report through the
# channel it is testing.
exec 8>&1 9>&2

# stdout side: fd 1 is a regular file for the duration, fd 2 is left as it is.
{
  printf 'redirect-selftest sentinel %s\n' {01..20}
  open_step_fds "" ""
  printf 'redirect-selftest via-fd3\n' >&3
  close_step_fds
  printf 'redirect-selftest tail\n'
} >"${TMP}/redirect-selftest-out.log"

# stderr side: fd 2 is a regular file for the duration, fd 1 is left as it is.
{
  printf 'redirect-selftest sentinel %s\n' {01..20} >&2
  open_step_fds "" ""
  printf 'redirect-selftest via-fd4\n' >&4
  close_step_fds
  printf 'redirect-selftest tail\n' >&2
} 2>"${TMP}/redirect-selftest-err.log"

exec 1>&8 2>&9 8>&- 9>&-

assert_redirect_intact "${TMP}/redirect-selftest-out.log" "stdout" 3
assert_redirect_intact "${TMP}/redirect-selftest-err.log" "stderr" 4

pass "run_step duplicates this gate's own descriptors rather than re-opening (and truncating) them"

# ---------------------------------------------------------------------------
# 4. The `chart` step is the single source of truth.
# ---------------------------------------------------------------------------
: >"${TMP}/github_output"
run_step "${CHART_STEP_SEL}" "the chart-coordinate step" "yes" /dev/null ||
  fail "the chart-coordinate step's run: body failed to execute"

SIM_CHART_OCI="$(output_value oci)"
SIM_CHART_REPO="$(output_value repository)"

[[ -n "${SIM_CHART_OCI}" ]] ||
  fail "the chart step must emit an 'oci' output (the target for 'helm push')"
[[ -n "${SIM_CHART_REPO}" ]] ||
  fail "the chart step must emit a 'repository' output (the full chart repository, for 'cosign sign' and the Artifact Hub metadata tag) -- with only one output every consumer has to append the chart name itself, which is how the copies drifted"

# helm push APPENDS the chart name, so the push target is the PARENT path and must not
# already end in it. Getting this backwards publishes to <owner>/charts/widget/widget.
[[ "${SIM_CHART_OCI}" == oci://* ]] ||
  fail "the chart step's 'oci' output must be an oci:// URL for 'helm push', got '${SIM_CHART_OCI}'"
[[ "${SIM_CHART_OCI}" != *"/${SIM_CHART_NAME}" ]] ||
  fail "the chart step's 'oci' output ends in the chart name ('${SIM_CHART_OCI}') -- 'helm push' appends the chart name itself, so this would publish to '${SIM_CHART_OCI}/${SIM_CHART_NAME}'"

# The two outputs must be the same coordinate, so they cannot drift. This is the
# structural half of "single source of truth": repository IS oci plus the chart name.
[[ "${SIM_CHART_REPO}" == "${SIM_CHART_OCI#oci://}/${SIM_CHART_NAME}" ]] ||
  fail "the chart step's outputs disagree: 'helm push' would write to '${SIM_CHART_OCI#oci://}/${SIM_CHART_NAME}' but 'repository' is '${SIM_CHART_REPO}'. Derive one from the other so they cannot drift"

# ADR-0709: the chart moves OFF the image repository and under a charts/ path segment.
[[ "${SIM_CHART_REPO}" == "${SIM_REGISTRY}/${SIM_OWNER_LC}/charts/${SIM_CHART_NAME}" ]] ||
  fail "the chart must be published under a charts/ path segment (expected '${SIM_REGISTRY}/${SIM_OWNER_LC}/charts/${SIM_CHART_NAME}'), got '${SIM_CHART_REPO}'"
[[ "${SIM_CHART_REPO}" != "${SIM_REGISTRY}/${SIM_OWNER_LC}/${SIM_CHART_NAME}" ]] ||
  fail "the chart is still published directly under the owner ('${SIM_CHART_REPO}'), which is the controller image's OCI repository (DR-FIND-07). ADR-0709 moves it to <owner>/charts/${SIM_CHART_NAME}"

# GHCR namespaces are lowercase; GitHub owners need not be. The simulated owner is
# mixed-case precisely so dropping the `tr` would red here.
[[ "${SIM_CHART_REPO}" != *"${SIM_OWNER}"* ]] ||
  fail "the chart step did not lowercase the repository owner: '${SIM_CHART_REPO}' still contains '${SIM_OWNER}'. GHCR rejects uppercase namespaces"

pass "the chart step emits a push target and a chart repository, both under charts/, from one lowercased owner"

# Production-shaped run: the synthetic run above proves nothing is hardcoded; this one
# pins the exact coordinate ADR-0709 names, so a structurally-correct but wrong-valued
# derivation (say <owner>/helm-charts) cannot slip through.
PROD_OWNER="PlatformRelay"
: >"${TMP}/github_output"
(
  SIM_REGISTRY="${WF_REGISTRY}"
  SIM_OWNER="${PROD_OWNER}"
  SIM_CHART_NAME="${WF_CHART_NAME}"
  run_step "${CHART_STEP_SEL}" "the chart-coordinate step" "yes" /dev/null
) || fail "the chart-coordinate step failed to execute with production inputs"

prod_oci="$(output_value oci)"
prod_repo="$(output_value repository)"
[[ "${prod_oci}" == "oci://${WF_REGISTRY}/platformrelay/charts" ]] ||
  fail "with the real registry and owner, 'helm push' must target 'oci://${WF_REGISTRY}/platformrelay/charts' (ADR-0709), got '${prod_oci}'"
[[ "${prod_repo}" == "${WF_REGISTRY}/platformrelay/charts/${WF_CHART_NAME}" ]] ||
  fail "with the real registry and owner, the chart repository must be '${WF_REGISTRY}/platformrelay/charts/${WF_CHART_NAME}' (ADR-0709), got '${prod_repo}'"

pass "with production inputs the chart coordinate is ${prod_repo}"

# ---------------------------------------------------------------------------
# 5. The publish step consumes those outputs -- and asserts what helm actually pushed.
# ---------------------------------------------------------------------------
: >"${TMP}/github_output"
: >"${TMP}/helm.args"
: >"${TMP}/helm.target"
: >"${TMP}/cosign.args"
STUB_HELM_FORCE_REF=""
publish_status=0
run_step "${PUBLISH_STEP_SEL}" "Publish and sign Helm chart (OCI)" "no" \
  "${TMP}/publish.out" "${TMP}/publish.err" || publish_status=$?

# EXACT, not a substring match. `helm push` appends the chart name to whatever target it
# is given, so passing the full chart repository instead of its parent silently publishes
# to <repo>/<chart>/<chart>. A substring test is satisfied by both, since the parent path
# is a prefix of the child -- which is exactly how that mutation passed an earlier
# revision of this gate green.
mapfile -t helm_targets <"${TMP}/helm.target"
[[ "${#helm_targets[@]}" -eq 1 ]] ||
  fail "expected exactly one 'helm push' from the publish step, saw ${#helm_targets[@]} (targets: ${helm_targets[*]-none}) -- with more than one, the target assertion below cannot say which push it is judging"
helm_target="${helm_targets[0]}"
[[ "${helm_target}" == "${SIM_CHART_OCI}" ]] ||
  fail "'helm push' must be given exactly the chart step's 'oci' output ('${SIM_CHART_OCI}') as its target, got '${helm_target}' -- helm APPENDS the chart name, so this publishes to '${helm_target#oci://}/${SIM_CHART_NAME}'"

# Only now: the step must have succeeded. Checked after the target assertion so a
# wrong-target mutation reports the wrong target rather than the downstream symptom.
[[ "${publish_status}" -eq 0 ]] ||
  fail "the publish step failed against a faithful stub helm push to '${SIM_CHART_OCI}': $(tr '\n' ' ' <"${TMP}/publish.out") | stderr: $(tr '\n' ' ' <"${TMP}/publish.err")"

# cosign signs a DIGEST reference; the repository half of it is what DIST-AH-03 is about.
# Asserted against the synthetic coordinate, so a re-derived or hardcoded ref reds here.
grep -Fq -- "${SIM_CHART_REPO}@${SIM_DIGEST}" "${TMP}/cosign.args" ||
  fail "'cosign sign' did not receive '${SIM_CHART_REPO}@${SIM_DIGEST}' -- the signed reference must be built from steps.chart.outputs.repository, not re-derived. cosign received: $(tr '\n' ' ' <"${TMP}/cosign.args")"

chart_digest_out="$(output_value digest)"
[[ "${chart_digest_out}" == "${SIM_DIGEST}" ]] ||
  fail "the publish step must still export the chart digest (the DR-FIND-07 guard reads steps.chart-publish.outputs.digest), got '${chart_digest_out}'"

pass "helm push and cosign sign both consume the chart step's outputs, and the step's own post-push assertion accepts a faithful push"

# The post-push assertion. This is the guard against a FUTURE silent desync: if anything
# ever causes helm to write somewhere other than the derived repository, the step must
# fail rather than sign whatever landed. Simulated by a stub helm that reports the OLD
# pre-ADR-0709 coordinate -- the exact regression being defended against.
: >"${TMP}/github_output"
: >"${TMP}/cosign.args"
STUB_HELM_FORCE_REF="${SIM_REGISTRY}/${SIM_OWNER_LC}/${SIM_CHART_NAME}:${SIM_VERSION}"
if run_step "${PUBLISH_STEP_SEL}" "Publish and sign Helm chart (OCI)" "no" /dev/null /dev/null; then
  fail "the publish step accepted a 'helm push' that wrote to '${STUB_HELM_FORCE_REF}' instead of '${SIM_CHART_REPO}'. It must parse the pushed reference out of the helm push output and fail when it is not under the expected chart repository"
fi
[[ ! -s "${TMP}/cosign.args" ]] ||
  fail "the publish step signed something after detecting a wrong push target -- the assertion must abort BEFORE 'cosign sign'"

pass "the publish step fails closed when helm pushes outside the derived chart repository"

# ---------------------------------------------------------------------------
# 6. The Artifact Hub metadata push targets the CHART repository, not the image.
#    This step is continue-on-error: true, so a wrong target here is invisible at
#    release time -- it just quietly costs Verified Publisher.
# ---------------------------------------------------------------------------
: >"${TMP}/github_output"
: >"${TMP}/oras.args"
run_step "${METADATA_STEP_SEL}" "Push Artifact Hub metadata" "no" /dev/null ||
  fail "the Artifact Hub metadata step failed to execute against stub oras"

grep -Eq "^push .*${SIM_CHART_REPO}:artifacthub.io" "${TMP}/oras.args" ||
  fail "'oras push' must target '${SIM_CHART_REPO}:artifacthub.io' -- Artifact Hub reads the repository-metadata tag from the CHART repository it tracks. oras received: $(tr '\n' ' ' <"${TMP}/oras.args")"

# The old target, spelled out so the failure names the regression rather than a diff.
! grep -Fq -- "${SIM_REGISTRY}/${SIM_OWNER_LC}/${WF_IMAGE_NAME}:artifacthub.io" "${TMP}/oras.args" ||
  fail "'oras push' still targets the controller IMAGE repository (\${REGISTRY}/\${OWNER}/\${IMAGE_NAME}). ADR-0709 step 1: without moving this tag to the chart repository, Verified Publisher breaks"

grep -Fq 'artifacthub-repo.yml' "${TMP}/oras.args" ||
  fail "'oras push' must still upload artifacthub-repo.yml as the repository-metadata layer"

pass "oras push sends the artifacthub.io metadata tag to the chart repository"

# `continue-on-error` and the ::warning:: reporting are contracts of DIST-AH-02
# (dist_artifacthub_release_test.sh owns them in full); re-assert only the one that
# makes THIS defect invisible, so the reason it matters is recorded where it is caused.
[[ "$(step_attr "${METADATA_STEP_SEL}" '["continue-on-error"]')" == "true" ]] ||
  fail "the Artifact Hub metadata step must stay 'continue-on-error: true' (ADR-0708: hub metadata is discoverability only and must never abort a signed release)"

# ---------------------------------------------------------------------------
# 7. The release notes render the same coordinate. The install snippet appends the
#    chart name to CHART_OCI, so it is a fourth consumer and drifts the same way.
# ---------------------------------------------------------------------------
notes_chart_oci=""
while IFS=$'\t' read -r k v; do
  [[ "${k}" == "CHART_OCI" ]] && notes_chart_oci="${v}"
done < <(step_env_pairs "${NOTES_STEP_SEL}")
[[ "${notes_chart_oci}" == '${{ steps.chart.outputs.oci }}' ]] ||
  fail "the release notes step must take CHART_OCI from steps.chart.outputs.oci (the release body's 'helm upgrade --install' line appends the chart name to it), got '${notes_chart_oci}'"

pass "the GitHub Release install snippet reads the same chart coordinate"

# ---------------------------------------------------------------------------
# 8. The DR-FIND-07 guard is RETAINED. ADR-0709: "separate paths make a collision
#    unreachable, but the guard costs nothing and fails closed". Bare tags 0.14.0-0.19.0
#    stay in the image repository permanently, so the collision surface is still real
#    for anything that reads them.
#
#    Presence alone is not enough: a guard that is skipped or soft-failed is decoration.
# ---------------------------------------------------------------------------
guard_if="$(step_attr "${GUARD_STEP_SEL}" 'if')"
[[ "${guard_if}" == "null" ]] ||
  fail "the DR-FIND-07 guard must run unconditionally, got 'if: ${guard_if}' -- a skipped guard cannot fail closed"
guard_coe="$(step_attr "${GUARD_STEP_SEL}" '["continue-on-error"]')"
[[ "${guard_coe}" == "null" || "${guard_coe}" == "false" ]] ||
  fail "the DR-FIND-07 guard must not declare 'continue-on-error: ${guard_coe}' -- a soft-failed guard reports a collision as green"

# The guard's two checks, asserted by the values they compare rather than by their text:
# it must read the controller image's tag list and both digests. Losing either reduces it
# to an unconditional `echo`.
guard_env="$(step_env_pairs "${GUARD_STEP_SEL}")"
for expr in \
  '${{ steps.image.outputs.tags }}' \
  '${{ steps.build.outputs.digest }}' \
  '${{ steps.chart-publish.outputs.digest }}'; do
  printf '%s\n' "${guard_env}" | grep -Fq -- "${expr}" ||
    fail "the DR-FIND-07 guard no longer receives ${expr} -- without it the guard compares nothing and passes unconditionally"
done
guard_run="$(step_run "${GUARD_STEP_SEL}")"
printf '%s' "${guard_run}" | grep -Fq 'exit 1' ||
  fail "the DR-FIND-07 guard has no failing path left -- it must exit non-zero on a collision"

pass "the DR-FIND-07 collision guard is retained, unconditional and able to fail the job"

echo "All dist release chart path tests passed."
