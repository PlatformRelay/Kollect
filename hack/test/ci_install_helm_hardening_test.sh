#!/usr/bin/env bash
# CI-HELMDL-01: hack/install-helm.sh must survive a transient TCP failure to get.helm.sh
# WITHOUT weakening the SHA256 verification that is the script's entire reason for existing.
#
# The defect this gate locks down, observed live on 2026-09-01: `kind-smoke` -- one of four
# REQUIRED checks on main -- went red on PR #351 (a bash-only diff) after 46s inside
# .github/actions/kind-e2e-setup with
#
#     curl: (7) Failed to connect to get.helm.sh port 443 after 1029 ms
#
# In that SAME composite step, the `kind` binary download is wrapped in a three-attempt loop
# with `--proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL --retry 5 --retry-delay 5
# --connect-timeout 30`, while `bash hack/install-helm.sh` ran two BARE `curl -fsSL` calls.
# One hardened, one not, same failure mode, same step. This gate exists to keep the asymmetry
# from coming back -- in EITHER direction: the checksum fetch counts as much as the tarball,
# because its output feeds a `[[ -z ... ]]` guard, so a half-fetched checksum is a worse
# failure than a clean non-zero exit.
#
# WHY curl's own --retry is not sufficient, and the outer loop is not decoration: curl retries
# "transient" conditions -- 408/429/5xx and timeouts -- but a bare connect failure is NOT one
# of them unless --retry-connrefused / --retry-all-errors is passed. `curl: (7)` is exactly
# that class. So the outer loop is the half that covers the failure actually observed, and
# --retry is the half that covers a flaky origin. Both are asserted, separately.
#
# ASSERTION STRATEGY. Static greps alone cannot tell a hardened fetch from a hardened-looking
# one, so the load-bearing half of this gate is BEHAVIOURAL: a stub `curl` is put first on
# PATH, the installer is run for real against a locally built tarball into a temp prefix, and
# the assertions read the stub's request log. That log is the only thing that can prove
# "the checksum fetch retried too" or "the mismatch was NOT retried". Every count assertion is
# preceded by a guard that the stub was reached at all, so no case can pass vacuously.
#
# SAFETY: this suite executes the real installer. It therefore also stubs `install(1)` with a
# wrapper that refuses any destination outside this test's temp tree, so no mutation of the
# script under test -- including one that re-hardcodes /usr/local/bin -- can make this gate
# write outside its sandbox.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT}/hack/install-helm.sh"

# CI-FETCHLIB-01. The hardened flag list this file was written to lock down no longer lives in
# hack/install-helm.sh: it was factored into hack/lib/fetch.sh and adopted by all eight
# installers and by the kind download in .github/actions/kind-e2e-setup -- which is the repo-wide
# fix the header of the ORIGINAL local helper explicitly asked for. So every assertion below that
# reads "the flags on the curl invocation" must read the installer AND the helper it sources:
# treating only hack/install-helm.sh as the subject would leave the flag assertions with no curl
# invocation to inspect, and this gate would red on a correct tree.
LIB="${ROOT}/hack/lib/fetch.sh"

fail() {
  printf 'ci install-helm hardening: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${SCRIPT}" ]] || fail "${SCRIPT} is missing"
[[ -s "${SCRIPT}" ]] || fail "${SCRIPT} is empty -- every assertion below would pass vacuously"
[[ -f "${LIB}" ]] ||
  fail "${LIB} is missing -- hack/install-helm.sh sources it for every fetch, so the flag assertions below would have nothing to inspect"

# ---------------------------------------------------------------------------------------------
# CI-FETCHLIB-01 sibling gate, run FIRST.
#
# hack/test/ci_fetch_lib_hardening_test.sh proves the shared helper's behaviour against a real
# local HTTPS origin, and asserts that every hack/install-*.sh routes through it. This file is
# invoked from an explicit step in the lint job of .github/workflows/ci.yaml; the sibling is not,
# because the lane that added it does not own .github/workflows/**, and the dist_*/sonar_ko_*
# globs in that job do not match its filename. Invoking it here is what keeps it in CI rather
# than in the tree-but-never-run state that makes a gate decorative.
#
# It runs BEFORE this file's own assertions on purpose: install-helm.sh's hardening is now a
# property of the helper, so "the helper is broken" is the more fundamental diagnosis and is the
# one an operator should read first. The lane handover records the one-line ci.yaml step that
# would give the sibling its own name in the job log; until that lands, this is the wiring.
# ---------------------------------------------------------------------------------------------
echo "--- CI-FETCHLIB-01 shared fetch helper gate ---"
bash "${ROOT}/hack/test/ci_fetch_lib_hardening_test.sh"
echo "--- back to CI-HELMDL-01 install-helm assertions ---"

# ---------------------------------------------------------------------------------------------
# Static assertions over the source text.
# ---------------------------------------------------------------------------------------------

# `set -e` is what turns a failed fetch into a failed script. Without it the retry loop can
# exhaust its attempts, the helper can return 1, and the script would sail on to `tar -xzf` a
# file that does not exist. `pipefail` matters for the same reason on any `curl | ...` shape.
grep -Eq '^set -euo pipefail[[:space:]]*$' "${SCRIPT}" ||
  fail "${SCRIPT} must keep 'set -euo pipefail' -- a failed download must abort the script, not fall through to tar/install"
pass "installer keeps 'set -euo pipefail'"

# A logical-line view of the script: backslash continuations joined, whole-line comments
# dropped. Both matter. The hardened curl invocation is spread over three physical lines (the
# kind loop's shape), so a physical-line grep for '--connect-timeout' on the same line as
# 'curl' finds nothing and the gate would red on a CORRECT script. And a comment that merely
# quotes the flags must not be able to satisfy an assertion about code -- the house precedent
# for that mutation class is GATE-COMMENT-01 in hack/test/dist_ci_wiring_test.sh.
# INSPECTED is the installer plus the helper it sources: since CI-FETCHLIB-01 the curl
# invocation lives in the latter, and the flag assertions below must follow it there.
INSPECTED=("${SCRIPT}" "${LIB}")

logical_lines() {
  awk '
    {
      line = $0
      while (sub(/\\[[:space:]]*$/, "", line) > 0) {
        if ((getline nxt) <= 0) { break }
        sub(/^[[:space:]]+/, "", nxt)
        line = line " " nxt
      }
      if (line ~ /^[[:space:]]*#/) { next }
      print line
    }
  ' "${INSPECTED[@]}"
}

# Lines on which curl is INVOKED, not merely mentioned. fetch_to's own failure diagnostic
# contains the words "curl exit ${status}"; a permissive `[^[:alnum:]]curl[[:space:]]` match
# reads that echo as a curl invocation and then reds the helper for "missing --proto '=https'"
# on a line that never runs curl. Command position: start of the logical line, or after a shell
# operator, a control keyword, or a command substitution.
curl_command_lines() {
  logical_lines | grep -E '(^|[;&|]|\$\(|(^|[[:space:]])(if|then|else|do|!))[[:space:]]*curl[[:space:]]' || true
}

# `grep -q` MUST NOT be used at the end of a pipeline in this file. Under `set -o pipefail` it
# exits on its first match and closes the pipe, the upstream stage takes SIGPIPE, and the
# PIPELINE status goes non-zero -- so `! producer | grep -q BAD` reports "not found" precisely
# when BAD *is* found. Two negative assertions below were written that way and were silently
# inverted; the wiring assertion at the bottom was written that way and red on a correct
# workflow. `grep -c` reads its input to the end, so counting is race-free in both directions.
#
# The `|| true` this used to carry swallowed grep's exit >= 2 as well as its exit 1, so a
# malformed pattern or an unreadable input reported ZERO matches -- and zero is exactly what
# both negative assertions below are looking for. That is a vacuous pass in the one direction
# this file's header promises cannot happen. grep distinguishes the two: 1 means "no match",
# >= 2 means "I could not answer". Only the former is a count.
count_matches() {
  local pattern_type="$1" pattern="$2" hits status=0
  hits="$(grep "${pattern_type}" -c -- "${pattern}")" || status=$?
  if ((status > 1)); then
    printf 'grep-error-%s\n' "${status}"
    return 0
  fi
  printf '%s\n' "${hits:-0}"
}

# Every count_matches caller runs through this. The numeric check is what converts a
# 'grep-error-N' sentinel into a red -- a bare string comparison would silently satisfy
# whichever direction it happened to land on ("grep-error-2" is both != "0" and not == "0").
require_count() {
  local value="$1" what="$2"
  [[ "${value}" =~ ^[0-9]+$ ]] ||
    fail "${what}: could not count matches (grep reported '${value}') -- treating an unanswerable grep as zero is how a negative assertion passes vacuously"
}

mapfile -t CURL_LINES < <(curl_command_lines)
(( ${#CURL_LINES[@]} > 0 )) ||
  fail "${SCRIPT} and ${LIB} between them contain no curl invocation -- either the downloads were replaced by something this gate does not inspect, or the logical-line extraction above is broken; in both cases the per-invocation assertions would pass vacuously"
pass "found ${#CURL_LINES[@]} curl invocation(s) to inspect"

# The link between the two files in INSPECTED, and the assertion that makes reading the helper
# legitimate. Without it, deleting the `source` line from hack/install-helm.sh and going back to
# a bare `curl -fsSL` would leave every flag assertion below GREEN -- they would still be reading
# hack/lib/fetch.sh, which would no longer have anything to do with the installer under test.
grep -Eq '^source "\$\{ROOT\}/hack/lib/fetch.sh"$' "${SCRIPT}" ||
  fail "${SCRIPT} must source hack/lib/fetch.sh -- the hardened flag list lives there since CI-FETCHLIB-01 and the flag assertions in this gate read that file. An installer that stops sourcing it would leave them inspecting a helper it does not use"
fetch_to_calls="$(logical_lines | count_matches -E '(^|[^[:alnum:]_])fetch_to[[:space:]]')"
require_count "${fetch_to_calls}" "install-helm fetch_to call scan"
((fetch_to_calls >= 2)) ||
  fail "${SCRIPT} makes ${fetch_to_calls} fetch_to call(s); both the CHECKSUM and the TARBALL must go through it. Hardening one and not the other is exactly the get.helm.sh asymmetry that reddened PR #351"

# Catches the mutation "swap curl for wget to sidestep the gate": every behavioural assertion
# below rides on a stub `curl` on PATH, so a different fetcher would silently reach the real
# network and turn this whole suite into a no-op.
other_fetcher_hits="$(logical_lines | count_matches -E '(^|[^[:alnum:]_./-])(wget|aria2c|python3?[[:space:]]+-m[[:space:]]+urllib)([[:space:]]|$)')"
require_count "${other_fetcher_hits}" "non-curl fetcher scan"
[[ "${other_fetcher_hits}" == "0" ]] ||
  fail "${SCRIPT} fetches with something other than curl -- the behavioural half of this gate stubs curl on PATH and would not observe it, so keep the fetcher as curl (or extend this gate first)"

for line in "${CURL_LINES[@]}"; do
  # Mutation caught: dropping --proto/--proto-redir. These are not cosmetic. get.helm.sh is a
  # redirector; without pinning both the request scheme AND the redirect scheme, a 301 to
  # http:// downgrades the checksum fetch to plaintext, and a checksum fetched over plaintext
  # verifies nothing at all -- an attacker who can rewrite the tarball can rewrite the digest
  # to match it.
  grep -Eq -- "--proto[[:space:]]+'?=https'?" <<<"${line}" ||
    fail "curl invocation is missing --proto '=https': ${line}"
  grep -Eq -- "--proto-redir[[:space:]]+'?=https'?" <<<"${line}" ||
    fail "curl invocation is missing --proto-redir '=https' (a redirect could downgrade to plaintext HTTP): ${line}"

  # Mutation caught: --retry 0, or dropping --retry. A positive count is required -- `--retry 0`
  # is textually present and behaviourally absent, which is the whole point of matching the
  # VALUE rather than the flag name.
  retry_n="$(grep -Eo -- '--retry[[:space:]]+[0-9]+' <<<"${line}" | head -1 | grep -Eo '[0-9]+' || true)"
  [[ -n "${retry_n}" ]] ||
    fail "curl invocation is missing '--retry <n>': ${line}"
  (( retry_n >= 1 )) ||
    fail "curl invocation sets '--retry ${retry_n}', which disables curl-level retries: ${line}"

  # Mutation caught: dropping --connect-timeout. Without it a blackholed TCP connect hangs for
  # the OS default (~130s) per attempt, so three attempts can outlive the job timeout -- the
  # retry loop then makes the failure slower instead of survivable.
  connect_n="$(grep -Eo -- '--connect-timeout[[:space:]]+[0-9]+' <<<"${line}" | head -1 | grep -Eo '[0-9]+' || true)"
  [[ -n "${connect_n}" ]] ||
    fail "curl invocation is missing '--connect-timeout <n>': ${line}"
  (( connect_n >= 1 )) ||
    fail "curl invocation sets '--connect-timeout ${connect_n}' (0 means 'no timeout'): ${line}"

  # Mutation caught: dropping --tlsv1.2. Low impact on its own -- --proto '=https' still holds
  # the scheme -- but locking the flag set is this gate's stated job, and a TLS floor is part of
  # that set in the kind loop this is deliberately symmetric with. Asserted so "consistent with
  # the kind download" stays a checked claim rather than a comment.
  grep -Eq -- '(^|[[:space:]])--tlsv1\.[23]([[:space:]]|$)' <<<"${line}" ||
    fail "curl invocation lost its --tlsv1.2 TLS floor: ${line}"

  # Mutation caught: dropping --max-time, or setting it so tight it becomes a flake source.
  # --connect-timeout bounds only the CONNECT; a body that stalls after the first byte hangs
  # until the job timeout with no diagnostic, which on a required check is the same lost hour
  # this whole change exists to prevent. The >= 30 floor guards the other direction: a timeout
  # flag is itself a way to introduce flake, and 18MB inside 30s is already a 600 KB/s demand.
  maxtime_n="$(grep -Eo -- '--max-time[[:space:]]+[0-9]+' <<<"${line}" | head -1 | grep -Eo '[0-9]+' || true)"
  [[ -n "${maxtime_n}" ]] ||
    fail "curl invocation is missing '--max-time <n>', so a stalled transfer has no bound: ${line}"
  (( maxtime_n >= 30 )) ||
    fail "curl invocation sets '--max-time ${maxtime_n}', which is tight enough to abort a healthy download of an 18MB tarball and become a new flake source: ${line}"

  # curl resets the --max-time counter on every one of its OWN retries, so --max-time alone
  # bounds an attempt, not the invocation. --retry-max-time is what closes that: without it,
  # '--retry 5 --max-time 120' is a 12-minute worst case per fetch, times three outer attempts.
  # A VALUE floor, not a presence check. Both degenerate settings are textually present and
  # behaviourally wrong, in opposite directions, and both survived a presence-only assertion:
  #   --retry-max-time 0  means "do not timeout retries" (curl's own words), i.e. UNBOUNDED --
  #     it removes the very bound this flag was added to provide;
  #   --retry-max-time 1  expires the retry window before --retry-delay 5 can schedule even one
  #     retry, neutering curl's retry layer -- the half that covers a flaky origin that ANSWERS
  #     with 5xx/429, which the outer loop does not address at all.
  # 30 is the smallest window that admits a few 5s-spaced retries, so it rejects both.
  retrymax_n="$(grep -Eo -- '--retry-max-time[[:space:]]+[0-9]+' <<<"${line}" | head -1 | grep -Eo '[0-9]+' || true)"
  [[ -n "${retrymax_n}" ]] ||
    fail "curl invocation is missing '--retry-max-time <n>' -- curl resets --max-time on each of its own retries, so without it the invocation is bounded only by (--retry + 1) x --max-time: ${line}"
  ((retrymax_n >= 30)) ||
    fail "curl invocation sets '--retry-max-time ${retrymax_n}': 0 means 'do not timeout retries' (unbounded, defeating the flag's purpose) and anything under --retry-delay expires the window before a single retry can be scheduled, neutering curl's retry layer: ${line}"

  # Mutation caught: "fixing" a TLS problem by turning verification off. -k/--insecure would
  # make --proto '=https' worthless.
  ! grep -Eq -- '(^|[[:space:]])(-k|--insecure|--proto-default[[:space:]]+http)([[:space:]]|$)' <<<"${line}" ||
    fail "curl invocation disables TLS verification or defaults to plaintext: ${line}"

  # `-f` is what turns an HTTP 404/5xx body into a non-zero exit. Without it curl writes the
  # error page to the output file and exits 0 -- the checksum guard would then see an HTML
  # blob, and the retry loop would never fire because nothing looked like a failure.
  grep -Eq -- '(^|[[:space:]])(-[a-zA-Z]*f[a-zA-Z]*|--fail)([[:space:]]|$)' <<<"${line}" ||
    fail "curl invocation lost -f/--fail, so an HTTP error page would be treated as a successful download: ${line}"
done
pass "every curl invocation pins https and a TLS floor, retries, bounds the connect AND the transfer, fails on HTTP errors, and keeps TLS verification"

# Mutation caught: pointing any URL at plaintext http://. Checked over the whole file, not just
# the curl lines, because the URLs are assembled into BASE_URL well above the fetches.
plaintext_hits="$(logical_lines | count_matches -F 'http://')"
require_count "${plaintext_hits}" "plaintext http:// scan"
[[ "${plaintext_hits}" == "0" ]] ||
  fail "${SCRIPT} references a plaintext http:// URL -- every fetch in this script must be https"
pass "no plaintext http:// URL anywhere in the installer"

# The checksum comparison must still exist AND must still stand between the download and the
# install. Ordering is the half a presence-grep cannot express: a script that compares digests
# after `install` has a verification step that verifies nothing.
# Anchored on the `echo` that RAISES the diagnostic, not on any line containing the phrase.
# The prose above the fetch helper discusses "a digest that does not match", and an earlier
# version of this anchor locked onto a COMMENT at line 106 and reported it as the location of
# the comparison -- harmless for the ordering result, but the pass line then misdirects anyone
# reading it. `^[[:space:]]*echo` cannot match a comment.
mismatch_line="$(grep -nE '^[[:space:]]*echo "checksum mismatch' "${SCRIPT}" | head -1 | cut -d: -f1 || true)"
install_line="$(grep -nE '^[[:space:]]*install[[:space:]]+-m' "${SCRIPT}" | head -1 | cut -d: -f1 || true)"
[[ -n "${mismatch_line}" ]] ||
  fail "${SCRIPT} no longer reports a 'checksum mismatch' -- the SHA256 comparison is the only reason this script exists"
[[ -n "${install_line}" ]] ||
  fail "${SCRIPT} has no 'install -m ...' line -- this gate could not locate the install step, so the ordering assertion below would pass vacuously"
(( mismatch_line < install_line )) ||
  fail "the checksum mismatch branch (line ${mismatch_line}) must come before the install (line ${install_line})"
pass "checksum comparison still gates the install (mismatch at line ${mismatch_line}, install at line ${install_line})"

# PRECONDITION for the behavioural half, and a real requirement in its own right: the installer
# must accept an install prefix so it can be exercised without writing to /usr/local/bin. This
# assertion is deliberately placed BEFORE the first `bash "${SCRIPT}"` below -- against an
# installer that hardcodes /usr/local/bin this gate must red HERE, not by attempting the write.
grep -Eq '^INSTALL_DIR="\$\{1:-/usr/local/bin\}"$' "${SCRIPT}" ||
  fail "${SCRIPT} must take the install prefix as \$1 (INSTALL_DIR=\"\\\${1:-/usr/local/bin}\"), defaulting to /usr/local/bin -- without it this gate cannot execute the installer without writing to a system path"
! grep -Eq '^[[:space:]]*install[[:space:]].*[[:space:]]/usr/local/bin/' "${SCRIPT}" ||
  fail "${SCRIPT} installs to a hardcoded /usr/local/bin path; it must install into \${INSTALL_DIR}"
pass "installer takes an install prefix argument and honours it in the install step"

# ---------------------------------------------------------------------------------------------
# Behavioural assertions: run the real installer against a stub curl.
# ---------------------------------------------------------------------------------------------

REAL_INSTALL="$(command -v install)"
[[ -x "${REAL_INSTALL}" ]] || fail "install(1) not found; cannot build the safety wrapper"

TMPROOT="$(mktemp -d)"
trap 'rm -rf "${TMPROOT}"' EXIT

STUB_BIN="${TMPROOT}/stubbin"
mkdir -p "${STUB_BIN}"

# Stub curl. Handles BOTH shapes -- `-o <file>` and write-to-stdout -- on purpose, so that an
# un-hardened installer is exercised by these cases rather than erroring out on the harness.
# Failure injection is per URL kind and per attempt, which is what makes "the checksum fetch is
# retried too" and "the mismatch is not retried" observable.
cat >"${STUB_BIN}/curl" <<'STUB_CURL'
#!/usr/bin/env bash
set -uo pipefail
D="${KOLLECT_CURL_STUB_DIR}"
printf '%s\n' "$*" >>"${D}/log"

dest=""
url=""
prev=""
for a in "$@"; do
  [[ "${prev}" == "-o" || "${prev}" == "--output" ]] && dest="${a}"
  case "${a}" in
    https://* | http://*) url="${a}" ;;
  esac
  prev="${a}"
done

kind="tarball"
case "${url}" in
  *.sha256) kind="checksum" ;;
esac

n=$(( $(cat "${D}/count_${kind}" 2>/dev/null || echo 0) + 1 ))
printf '%s\n' "${n}" >"${D}/count_${kind}"

failures="$(cat "${D}/${kind}_fail_first" 2>/dev/null || echo 0)"
if (( n <= failures )); then
  printf 'curl: (7) Failed to connect to get.helm.sh port 443 after 1029 ms: Couldn'"'"'t connect to server\n' >&2
  exit 7
fi

emit() {
  if [[ -n "${dest}" ]]; then cat >"${dest}"; else cat; fi
}
if [[ "${kind}" == "checksum" ]]; then
  emit <"${D}/checksum_body"
else
  emit <"$(cat "${D}/tarball_path")"
fi
STUB_CURL

# Stub sleep: keeps the retry backoff from costing 20s per case, and records that the installer
# actually backed off between attempts instead of hammering the origin.
cat >"${STUB_BIN}/sleep" <<'STUB_SLEEP'
#!/usr/bin/env bash
printf '%s\n' "${1:-}" >>"${KOLLECT_CURL_STUB_DIR}/sleeps"
exit 0
STUB_SLEEP

# Safety wrapper described in the header: nothing this suite runs may write outside TMPROOT.
cat >"${STUB_BIN}/install" <<'STUB_INSTALL'
#!/usr/bin/env bash
set -uo pipefail
dest="${*: -1}"
case "${dest}" in
  "${KOLLECT_INSTALL_STUB_ROOT}"/*) ;;
  *)
    printf 'install stub: refusing to write outside the test tree: %s\n' "${dest}" >&2
    exit 90
    ;;
esac
exec "${KOLLECT_REAL_INSTALL}" "$@"
STUB_INSTALL

chmod +x "${STUB_BIN}/curl" "${STUB_BIN}/sleep" "${STUB_BIN}/install"

# Build a real, locally-generated helm tarball with the layout the installer unpacks. OS/ARCH
# are derived the same way the installer derives them, so this harness follows the script
# rather than restating a guess about the runner.
T_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
T_ARCH="$(uname -m)"
case "${T_ARCH}" in
  x86_64) T_ARCH="amd64" ;;
  aarch64 | arm64) T_ARCH="arm64" ;;
  *) fail "unsupported test architecture ${T_ARCH}; extend this harness alongside the installer" ;;
esac

mkdir -p "${TMPROOT}/src/${T_OS}-${T_ARCH}"
cat >"${TMPROOT}/src/${T_OS}-${T_ARCH}/helm" <<'FAKE_HELM'
#!/usr/bin/env bash
printf 'v3.21.4+gstub\n'
FAKE_HELM
chmod +x "${TMPROOT}/src/${T_OS}-${T_ARCH}/helm"

GOOD_TARBALL="${TMPROOT}/good.tar.gz"
tar -czf "${GOOD_TARBALL}" -C "${TMPROOT}/src" "${T_OS}-${T_ARCH}"
GOOD_DIGEST="$(sha256sum "${GOOD_TARBALL}" | awk '{print $1}')"
[[ "${GOOD_DIGEST}" =~ ^[0-9a-f]{64}$ ]] || fail "harness could not compute a digest for its own fixture tarball"

CASE_DIR=""
BIN_DIR=""
STUB_DIR=""

new_case() {
  CASE_DIR="${TMPROOT}/cases/$1"
  BIN_DIR="${CASE_DIR}/bin"
  STUB_DIR="${CASE_DIR}/stub"
  mkdir -p "${BIN_DIR}" "${STUB_DIR}"
  : >"${STUB_DIR}/log"
  printf '%s\n' "${GOOD_DIGEST}" >"${STUB_DIR}/checksum_body"
  printf '%s\n' "${GOOD_TARBALL}" >"${STUB_DIR}/tarball_path"
}

run_installer() {
  local status=0
  KOLLECT_CURL_STUB_DIR="${STUB_DIR}" \
    KOLLECT_INSTALL_STUB_ROOT="${TMPROOT}" \
    KOLLECT_REAL_INSTALL="${REAL_INSTALL}" \
    HELM_VERSION="v3.21.4" \
    PATH="${STUB_BIN}:${BIN_DIR}:${PATH}" \
    bash "${SCRIPT}" "${BIN_DIR}" >"${CASE_DIR}/out" 2>"${CASE_DIR}/err" || status=$?
  printf '%s\n' "${status}" >"${CASE_DIR}/status"
  return 0
}

case_status() { cat "${CASE_DIR}/status"; }
case_err() { cat "${CASE_DIR}/err"; }
requests() { cat "${STUB_DIR}/count_${1}" 2>/dev/null || printf '0\n'; }

# Every count assertion below is preceded by this: if the stub was never reached, the counts are
# all zero and an "== 0" expectation would pass while the installer talked to the real internet.
assert_stub_was_used() {
  [[ -s "${STUB_DIR}/log" ]] ||
    fail "$1: the stub curl was never invoked -- PATH injection failed, so this case proved nothing (installer stderr: $(case_err))"
}

# Mutation caught: the kind loop's fall-through bug, reproduced here. Dropping the `return 1`
# at the end of fetch_with_retry makes the helper return the status of its final `echo` -- zero
# -- so the script sails past the failed download and dies further down, at `tr`/`sha256sum`/
# `tar` complaining that a file is missing. `set -e` still fails the script, which is why an
# exit-status-only assertion is NOT enough to see this: a review mutation removing `return 1`
# passed every other case in this suite. What changes is the diagnostic -- the operator is
# handed "No such file or directory" instead of "could not reach get.helm.sh" -- and that is
# the difference between a two-minute triage and a twenty-minute one on a required check.
assert_failed_at_the_download() {
  ! grep -Eq 'No such file or directory|cannot open|Cannot open|not in gzip format' <<<"$(case_err)" ||
    fail "$1: the installer reported a downstream file error, which means the retry loop fell THROUGH after exhausting its attempts instead of returning non-zero -- the failure must be raised at the fetch that actually failed, not later at tr/sha256sum/tar. stderr was: $(case_err)"
}

# --- Case A: happy path ------------------------------------------------------------------------
new_case happy
run_installer
assert_stub_was_used "happy path"
[[ "$(case_status)" == "0" ]] ||
  fail "happy path: installer exited $(case_status) against a well-formed checksum and tarball: $(case_err)"
[[ -x "${BIN_DIR}/helm" ]] ||
  fail "happy path: installer exited 0 but did not install ${BIN_DIR}/helm"
[[ "$(requests checksum)" == "1" && "$(requests tarball)" == "1" ]] ||
  fail "happy path: expected exactly one checksum and one tarball request, got checksum=$(requests checksum) tarball=$(requests tarball)"

# Both requests, as the installer actually issued them. This is the assertion that cannot be
# fooled by a helper that hardens one call site and not the other, and it is the direct
# regression lock on the get.helm.sh asymmetry: it reads argv, not source text.
mapfile -t LOGGED < <(cat "${STUB_DIR}/log")
(( ${#LOGGED[@]} == 2 )) ||
  fail "happy path: expected 2 logged curl invocations, got ${#LOGGED[@]}"
grep -q '\.sha256' <<<"${LOGGED[0]}" ||
  fail "happy path: the FIRST fetch must be the checksum, so the tarball is never downloaded without an expected digest in hand; got: ${LOGGED[0]}"
for logged in "${LOGGED[@]}"; do
  for needle in "--proto =https" "--proto-redir =https" "--retry" "--connect-timeout"; do
    grep -Fq -- "${needle}" <<<"${logged}" ||
      fail "a real curl invocation was missing '${needle}': ${logged}"
  done
done
pass "both live fetches (checksum first, then tarball) carry https pinning, --retry and --connect-timeout"

# --- Case B: transient failure on the TARBALL fetch ---------------------------------------------
# Mutation caught: deleting the outer retry loop. curl's own --retry does not cover exit 7, so
# without the loop this case reds even though --retry is still on the command line.
new_case tarball-transient
printf '1\n' >"${STUB_DIR}/tarball_fail_first"
run_installer
assert_stub_was_used "transient tarball failure"
[[ "$(case_status)" == "0" ]] ||
  fail "transient tarball failure: a single curl exit 7 must be retried, not fatal; installer exited $(case_status): $(case_err)"
[[ -x "${BIN_DIR}/helm" ]] ||
  fail "transient tarball failure: installer exited 0 but installed nothing"
[[ "$(requests tarball)" == "2" ]] ||
  fail "transient tarball failure: expected the tarball to be re-requested (2 attempts), got $(requests tarball)"
[[ -s "${STUB_DIR}/sleeps" ]] ||
  fail "transient tarball failure: the installer retried without backing off -- a retry loop with no sleep hammers a host that is already refusing connections"
pass "a transient curl exit 7 on the tarball is retried, with a backoff, and the install still succeeds"

# --- Case C: transient failure on the CHECKSUM fetch --------------------------------------------
# THE asymmetry lock. This is the exact shape of the PR #351 failure -- the connect that died
# was the FIRST of the two, the checksum fetch -- and it is the case that reds if someone
# hardens only the tarball download.
new_case checksum-transient
printf '1\n' >"${STUB_DIR}/checksum_fail_first"
run_installer
assert_stub_was_used "transient checksum failure"
[[ "$(case_status)" == "0" ]] ||
  fail "transient checksum failure: the CHECKSUM fetch must be retried exactly like the tarball fetch -- this is the get.helm.sh asymmetry the gate exists for; installer exited $(case_status): $(case_err)"
[[ -x "${BIN_DIR}/helm" ]] ||
  fail "transient checksum failure: installer exited 0 but installed nothing"
[[ "$(requests checksum)" == "2" ]] ||
  fail "transient checksum failure: expected the checksum to be re-requested (2 attempts), got $(requests checksum)"
pass "a transient curl exit 7 on the CHECKSUM fetch is retried too (no hardening asymmetry)"

# --- Case D: permanent failure on the checksum fetch --------------------------------------------
# Two mutations caught at once. (1) An unbounded loop: attempts must stop. (2) The bug in the
# kind loop this hardening is modelled on -- it exhausts its three attempts and then falls
# THROUGH to the next command, so the real failure surfaces later as a confusing chmod/tar
# error. Here the installer must fail at the download, say so, and install nothing.
new_case checksum-permanent
printf '99\n' >"${STUB_DIR}/checksum_fail_first"
run_installer
assert_stub_was_used "permanent checksum failure"
[[ "$(case_status)" != "0" ]] ||
  fail "permanent checksum failure: installer exited 0 with no checksum -- the retry loop swallowed the final failure"
[[ "$(requests checksum)" == "3" ]] ||
  fail "permanent checksum failure: expected exactly 3 bounded attempts, got $(requests checksum)"
[[ "$(requests tarball)" == "0" ]] ||
  fail "permanent checksum failure: the tarball must not be downloaded when no expected digest could be obtained, got $(requests tarball) request(s)"
[[ ! -e "${BIN_DIR}/helm" ]] ||
  fail "permanent checksum failure: nothing may be installed"
grep -Fq 'get.helm.sh' <<<"$(case_err)" ||
  fail "permanent checksum failure: the error must name the URL that could not be reached, got: $(case_err)"
assert_failed_at_the_download "permanent checksum failure"
pass "a permanent checksum-fetch failure aborts after 3 bounded attempts, naming the URL, installing nothing"

# --- Case E: permanent failure on the tarball fetch ---------------------------------------------
new_case tarball-permanent
printf '99\n' >"${STUB_DIR}/tarball_fail_first"
run_installer
assert_stub_was_used "permanent tarball failure"
[[ "$(case_status)" != "0" ]] ||
  fail "permanent tarball failure: installer exited 0 without a tarball"
[[ "$(requests tarball)" == "3" ]] ||
  fail "permanent tarball failure: expected exactly 3 bounded attempts, got $(requests tarball)"
[[ ! -e "${BIN_DIR}/helm" ]] ||
  fail "permanent tarball failure: nothing may be installed"
grep -Fq 'get.helm.sh' <<<"$(case_err)" ||
  fail "permanent tarball failure: the error must name the URL that could not be reached, got: $(case_err)"
assert_failed_at_the_download "permanent tarball failure"
pass "a permanent tarball failure aborts after 3 bounded attempts, installing nothing"

# --- Case F: checksum MISMATCH is a hard failure, never retried ---------------------------------
# The security assertion. Retrying a digest mismatch would turn a deterministic, diagnosable
# integrity failure into a flaky one -- and a check that sometimes passes is strictly worse than
# one that always fails. `requests tarball == 1` is what proves the retry loop wraps the
# TRANSPORT and not the verification.
new_case checksum-mismatch
printf '%s\n' "0000000000000000000000000000000000000000000000000000000000000000" >"${STUB_DIR}/checksum_body"
run_installer
assert_stub_was_used "checksum mismatch"
[[ "$(case_status)" != "0" ]] ||
  fail "checksum mismatch: installer exited 0 -- the SHA256 comparison no longer gates anything"
grep -Fq 'checksum mismatch' <<<"$(case_err)" ||
  fail "checksum mismatch: expected a 'checksum mismatch' diagnostic, got: $(case_err)"
[[ ! -e "${BIN_DIR}/helm" ]] ||
  fail "checksum mismatch: a binary was installed anyway"
[[ "$(requests tarball)" == "1" ]] ||
  fail "checksum mismatch: the tarball was fetched $(requests tarball) times -- a digest mismatch must NEVER be retried, or the retry loop is wrapped around the verification instead of around the transport"
pass "a checksum mismatch fails hard, installs nothing, and is not retried"

# --- Case G: empty checksum body ----------------------------------------------------------------
# The `[[ -z ... ]]` guard the brief singles out. An HTTP 200 with an empty body is the shape a
# captive portal or a truncated CDN response produces; it must not be allowed to become
# `EXPECTED_SHA256=""`, which would compare unequal to every digest and could, under a sloppier
# rewrite, compare EQUAL to an equally-empty actual.
new_case checksum-empty
: >"${STUB_DIR}/checksum_body"
run_installer
assert_stub_was_used "empty checksum body"
[[ "$(case_status)" != "0" ]] ||
  fail "empty checksum body: installer exited 0 with no expected digest"
[[ "$(requests tarball)" == "0" ]] ||
  fail "empty checksum body: the tarball must not be downloaded at all, got $(requests tarball) request(s)"
[[ ! -e "${BIN_DIR}/helm" ]] ||
  fail "empty checksum body: nothing may be installed"
pass "an empty checksum response aborts before the tarball is ever requested"

# --- Case H: well-formed-looking but non-digest checksum body -----------------------------------
# A 200 that carries an interception page rather than a digest. The `-z` guard does not catch
# this: the body is non-empty, so it sails through into EXPECTED_SHA256.
#
# REVIEW FIX (F1). The first version of this case asserted only "exits non-zero" and "installs
# nothing", and an independent reviewer killed it by DELETING the 64-hex shape guard from the
# installer: without the guard the HTML falls through to the digest comparison, which of course
# does not match, so the script still exits non-zero and installs nothing and this case still
# went green -- passing for entirely the wrong reason while its own comment claimed to cover the
# guard. That is the worst failure mode a gate has, and it mattered here because the shape guard
# is one of the three things this change advertises as making the script STRONGER; an advertised
# hardening with no test behind it is precisely what a future simplification pass deletes with
# every gate still green.
#
# The two assertions that actually discriminate:
#   * tarball requests == 0. WITH the guard, the script aborts before the tarball is ever
#     requested. WITHOUT it, the tarball must be downloaded in full before the comparison can
#     fail -- so this count, and only this count, separates the two worlds.
#   * the diagnostic names the SHAPE failure. Falling through to the comparison reports
#     "checksum mismatch", which sends the reader hunting for a corrupt tarball that is fine,
#     when the real fault is that no checksum was ever received.
new_case checksum-garbage
printf '<html><body>Authentication required</body></html>\n' >"${STUB_DIR}/checksum_body"
run_installer
assert_stub_was_used "non-digest checksum body"
[[ "$(case_status)" != "0" ]] ||
  fail "non-digest checksum body: installer exited 0 while holding an HTML page as its expected digest"
[[ ! -e "${BIN_DIR}/helm" ]] ||
  fail "non-digest checksum body: nothing may be installed"
[[ "$(requests tarball)" == "0" ]] ||
  fail "non-digest checksum body: the tarball was requested $(requests tarball) time(s) -- the 64-hex shape guard is gone, so an interception page reached the digest comparison instead of being rejected as 'not a checksum'; the install still fails, but 18MB later and under a diagnostic that blames the tarball"
grep -Fq 'not a bare sha256 digest' <<<"$(case_err)" ||
  fail "non-digest checksum body: the error must say the checksum body is not a digest, not 'checksum mismatch' -- a mismatch diagnostic points the reader at the tarball when the fault is that no checksum was received. Got: $(case_err)"
pass "a non-digest checksum response is rejected as a bad checksum, before the tarball is requested"

# --- Case I: the install prefix does not exist yet ----------------------------------------------
# REVIEW FIX (round 2, finding 1). `mkdir -p "${INSTALL_DIR}"` was added to match the house
# pattern in hack/install-operator-sdk.sh and shipped with NO discriminating coverage: deleting
# it left the whole gate green, because new_case pre-creates BIN_DIR, so no case ever handed the
# installer a prefix that did not exist. The only evidence offered was a manual "verified live"
# run -- which is the exact standard this suite filed F1 against one commit earlier. Covering it
# rather than deleting it, because the prefix argument is a real entry point (this gate is its
# only caller today, but that is an argument for testing it, not for leaving it half-built), and
# a nested-prefix install demonstrably works.
#
# BIN_DIR is deliberately NOT created here. Without the mkdir, `install -m 0755` fails with
# "No such file or directory" and the case reds on the exit status.
new_case install-prefix-missing
BIN_DIR="${CASE_DIR}/prefix/does/not/exist/bin"
[[ ! -d "${BIN_DIR}" ]] ||
  fail "install-prefix-missing: BIN_DIR was pre-created, so this case cannot discriminate -- it would pass with or without the mkdir in the installer"
run_installer
assert_stub_was_used "missing install prefix"
[[ "$(case_status)" == "0" ]] ||
  fail "missing install prefix: the installer must create the prefix it was handed (mkdir -p \"\${INSTALL_DIR}\", as hack/install-operator-sdk.sh does) instead of failing on it; exited $(case_status): $(case_err)"
[[ -x "${BIN_DIR}/helm" ]] ||
  fail "missing install prefix: installer exited 0 but ${BIN_DIR}/helm is absent or not executable"
pass "the installer creates a nested install prefix that does not exist yet"

# ---------------------------------------------------------------------------------------------
# REVIEW FIX (F2): this gate must assert its own registration. House precedent is
# hack/test/dev_mise_pin_drift_test.sh -- "an unwired gate is not a gate".
#
# It carries more weight here than for the scripts around it. The lint job runs most of its
# meta-tests through two globs, `hack/test/dist_*_test.sh` and `hack/test/sonar_ko_*_test.sh`,
# and neither pattern matches `ci_install_helm_hardening_test.sh`. So this gate is held in CI by
# exactly one explicit two-line step, and deleting that step -- the obvious move for anyone
# trying to make a red go away -- removes it from CI while leaving the script in the tree,
# fully green when run by hand, protecting nothing.
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"
[[ -f "${CI_WORKFLOW}" ]] || fail "${CI_WORKFLOW} is missing"

# LINE-EXACT against a comment-stripped, `run:`-unwrapped, trimmed view -- not a substring
# search for the filename. Both refinements are borrowed from failures the house gates already
# paid for, and both were re-proven here: a plain `grep -Fq` for the invocation was written
# first and mutation-tested, and it passed on `run: "# bash hack/test/..."`.
#
#   * GATE-COMMENT-01 (hack/test/dist_ci_wiring_test.sh): a commented-out command is not a
#     command. Dropping YAML lines that START with `#` is not enough -- the shell comment lives
#     INSIDE the scalar, so `run: "# bash ..."` survives that filter untouched.
#   * GATE-SCOPE-01 (same file): a substring match cannot tell the real invocation from a
#     neutered lookalike. `bash <script> || true` and `bash <script> &` both contain the literal
#     while neither can fail the step.
#
# Stripping an optional `run:` prefix makes one assertion cover both spellings a step can use:
# the inline `run: bash <script>` this workflow uses today, and a `run: |` block scalar.
#
# SCOPED TO `.jobs.lint`, not to the whole file. GATE-HARDEN-01 in dist_ci_wiring_test.sh is the
# house record of why: that gate compared file-global line numbers, so moving a step out of
# `lint` into an earlier job kept it green while the job that actually needed the binary had
# none. The same class applies here and survives both this gate and dist_ci_wiring_test.sh --
# relocate this step into a path-filtered or conditional job and a file-wide grep still finds
# the string, while the gate no longer runs on the changes it exists to catch.
#
# Textual block extraction rather than yq: yq is installed by a LATER step in this same job, so
# hard-requiring it here would red the lint job for a reason unrelated to helm. Job keys sit at
# two-space indent, so the block runs from `  lint:` to the next two-space non-comment key.
lint_job_lines() {
  awk '
    /^  lint:[[:space:]]*$/ { inlint = 1; next }
    inlint && /^  [^[:space:]#]/ { inlint = 0 }
    inlint { print }
  ' "${CI_WORKFLOW}"
}

# Without this the extraction could silently yield nothing -- a renamed or re-indented job -- and
# every assertion below would be searching an empty stream.
lint_block_size="$(lint_job_lines | wc -l)"
((lint_block_size > 10)) ||
  fail "could not extract the lint job from ${CI_WORKFLOW} (got ${lint_block_size} lines) -- the job was renamed or re-indented, and the wiring assertion below would search an empty stream"

# KNOWN RESIDUAL, recorded rather than papered over. A line-exact grep proves the invocation is
# PRESENT and unneutered; it cannot prove the step RUNS. Every evasion below leaves the line
# matched here and still stops the gate failing the build. Each needs a structural yq read,
# unavailable at this point in the job for the reason above.
#
# At step level, on THIS step:
#   * `continue-on-error: true` -- failures reported as success;
#   * `if: false` -- never runs;
#   * a `shell:` override, e.g. `shell: bash -c "echo skipped {0}"` -- GitHub substitutes the
#     run body into the template, so the invocation stays byte-identical here and is echoed
#     rather than executed. Same bucket as the two above, and easy to miss precisely because
#     the `run:` line looks untouched.
#
# At job level:
#   * `strategy.matrix: []` on `lint` -- an empty matrix materialises no jobs at all, so every
#     step in it, this one included, silently never runs.
#
# What IS closed elsewhere, stated exactly rather than as "the job-level half":
# hack/test/dist_ci_wiring_test.sh asserts `.jobs.lint.if` is null and
# `.jobs.lint["continue-on-error"]` is unset-or-false. That is the whole of its job-level
# coverage -- it contains no reference to `strategy` or `matrix`, and an independent review
# confirmed it passes against a `lint` job carrying `strategy.matrix: []`. So the empty-matrix
# evasion above is seen by NEITHER gate today; it is filed as its own item against that file,
# which this lane does not own.
#
# `working-directory:` is deliberately NOT on this list: it would red in CI on a missing path,
# so surviving it is correct behaviour, not a gap.
#
# If this gate ever moves after the "Ensure yq is available" step, tighten this into a yq
# step-graph assertion in the shape dist_ci_wiring_test.sh uses, which would close the
# step-level entries above.
wiring_hits="$(lint_job_lines | grep -vE '^[[:space:]]*#' |
  sed -E 's/^[[:space:]]*//; s/[[:space:]]*$//; s/^run:[[:space:]]*//' |
  count_matches -Fx 'bash hack/test/ci_install_helm_hardening_test.sh')"
require_count "${wiring_hits}" "lint-job wiring scan"
[[ "${wiring_hits}" != "0" ]] ||
  fail "hack/test/ci_install_helm_hardening_test.sh is not invoked from .github/workflows/ci.yaml on a bare, uncommented 'bash <script>' line -- an unwired gate is not a gate, and the dist_*/sonar_ko_* globs in the lint job do NOT match this filename, so one explicit step in THE LINT JOB is the only thing keeping it in CI. The step is missing, moved to another job, commented out, or neutered (a trailing '|| true', '&' or redirect)."
pass "the gate is wired into the lint job of .github/workflows/ci.yaml by an explicit, uncommented, unguarded step"

echo "All CI-HELMDL-01 install-helm hardening tests passed."
