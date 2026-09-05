#!/usr/bin/env bash
# CI-FETCHLIB-01: every hack/install-*.sh must fetch through ONE hardened helper,
# hack/lib/fetch.sh, and that helper must actually survive the failure modes that redden
# required checks -- proven against a real local server, not against a stubbed curl.
#
# WHY THIS FILE EXISTS. On 2026-09-01 the REQUIRED `kind-smoke` check went red on PR #351
# (a bash-only diff) after 46s with
#
#     curl: (7) Failed to connect to get.helm.sh port 443 after 1029 ms
#
# CI-HELMDL-01 (PR #354) fixed hack/install-helm.sh and deliberately kept its retry helper
# LOCAL to that one file, because factoring it out would have touched files that lane did not
# own. Its own header says so. That left SEVEN sibling installers -- git-cliff, gitleaks,
# helm-docs, kubeaudit, operator-sdk, polaris, shellcheck -- issuing bare `curl -fsSL`, i.e.
# the identical defect, any one of which can reproduce the identical red. This gate is what
# makes installer number nine impossible to add un-hardened.
#
# WHAT IS ASSERTED, AND WHY IN THIS ORDER.
#
#   PART 1 -- BEHAVIOURAL, against a real HTTPS origin on 127.0.0.1 with a self-signed cert.
#     A stubbed `curl` can only prove that a script CALLED curl with certain words on the
#     command line; it cannot prove that those words mean what the comment above them claims.
#     Every hardening property this lane advertises is therefore exercised against a server
#     that really refuses connections, really returns 500 before 200, really redirects
#     https->http, and really truncates a body mid-transfer. The distinction is not academic:
#     the single most consequential curl fact in this lane -- that `--retry` does NOT cover a
#     failed connect -- is invisible to a static reading of the flag list, and a stub cannot
#     tell you that either.
#
#   PART 2 -- STATIC, repo-wide. Behavioural coverage of the helper proves the helper is good;
#     it says nothing about whether the installers USE it. Part 2 is the half that scales: it
#     enumerates hack/install-*.sh from the filesystem (never a hardcoded list, so a ninth
#     installer is picked up the day it lands) and requires each one to source the helper, to
#     route every fetch through it, and to contain no direct fetcher invocation at all.
#
#   PART 3 -- the kind download in .github/actions/kind-e2e-setup (CI-KINDLOOP-01). That loop
#     is the pattern in this repo that LOOKS hardened, so it is the one the next person copies.
#     It falls through after three failed attempts onto `chmod +x` of a file that was never
#     written, so an unreachable kind.sigs.k8s.io surfaces as "No such file or directory".
#
# SIGPIPE (GATE-SIGPIPE-01, house rule): `! producer | grep -q PATTERN` INVERTS under
# `set -o pipefail` -- grep exits at its first match, the producer takes SIGPIPE, the pipeline
# status goes non-zero, and the negation reports "not found" exactly when the thing IS found.
# Every negative assertion below counts with `grep -c` through count_matches(), which reads its
# input to the end, and every count is validated as numeric so an unanswerable grep (exit >= 2)
# cannot be silently read as zero.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LIB="${ROOT}/hack/lib/fetch.sh"

fail() {
  printf 'ci fetch-lib hardening: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

# See the SIGPIPE note in the header. grep distinguishes "no match" (1) from "I could not
# answer" (>= 2); only the former is a count, and a bare `|| true` would flatten both to zero --
# which is exactly the value every negative assertion here is looking for.
count_matches() {
  local pattern_type="$1" pattern="$2" hits status=0
  hits="$(grep "${pattern_type}" -c -- "${pattern}")" || status=$?
  if ((status > 1)); then
    printf 'grep-error-%s\n' "${status}"
    return 0
  fi
  printf '%s\n' "${hits:-0}"
}

require_count() {
  local value="$1" what="$2"
  [[ "${value}" =~ ^[0-9]+$ ]] ||
    fail "${what}: could not count matches (grep reported '${value}') -- treating an unanswerable grep as zero is how a negative assertion passes vacuously"
}

# Backslash continuations joined, whole-line comments dropped. Both halves matter: the hardened
# curl invocation is spread over several physical lines, so a physical-line grep for a flag "on
# the same line as curl" finds nothing on a CORRECT helper; and a comment quoting a flag must
# not be able to satisfy an assertion about code (GATE-COMMENT-01, hack/test/dist_ci_wiring_test.sh).
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
      # REVIEW FIX (round 3, P3). Dropping only WHOLE-line comments left a trailing comment as
      # live text, so `foo=bar  # see hack/tools/curl for details` counted as a fetcher
      # invocation once the fetcher pattern learned to accept a path-qualified name. The strip
      # is deliberately conservative -- it fires only when NOTHING on the line is quoted -- so
      # it can never remove code: a `#` inside a string (curl -H "X: #1") keeps its line intact,
      # and `${VERSION#v}` / `${#arr[@]}` are untouched anyway because the pattern requires
      # whitespace before the `#`. A false NEGATIVE here would hide a real fetcher, which is the
      # one direction this file must not risk.
      q = sprintf("%c", 39)
      if (index(line, "\"") == 0 && index(line, q) == 0) {
        sub(/[[:space:]]+#.*$/, "", line)
      }
      print line
    }
  ' "$1"
}

# Lines on which curl is INVOKED, as opposed to merely named. The distinction is load-bearing:
# fetch_to's own failure diagnostic contains the words "curl exit ${status}", and a permissive
# `[^[:alnum:]]curl[[:space:]]` match treats that echo as a curl invocation and then reds the
# helper for "missing --proto '=https'" on a line that never runs curl at all. Command position
# means: start of the logical line, or after a shell operator / control keyword / command
# substitution.
curl_command_lines() {
  logical_lines "$1" | grep -E '(^|[;&|]|\$\(|(^|[[:space:]])(if|then|else|do|!))[[:space:]]*([^[:space:];&|()]*/)?curl[[:space:]]' || true
}

# The direct-fetcher pattern, defined ONCE because Part 2 (installers) and Part 3 (the composite
# action) must agree on what "a fetch that bypasses the helper" looks like.
#
# ------------------------------------------------------------------------------------------------
# HOW THIS PATTERN GOT HERE. Three versions, and the middle one is the cautionary tale.
#
# v1 led with `(^|[^[:alnum:]_./-])`. That class excludes `/`, so a PATH-QUALIFIED fetcher was
# invisible: `/usr/bin/curl -fsSL ...` beside a surviving fetch_to left this gate GREEN, because
# the positive assertions were still satisfied by the other call site. One hardened fetch and one
# bare one in the same file -- the PR #351 shape exactly -- passing the assertion whose contract
# is "no direct fetcher invocation at all".
#
# v2 fixed that by narrowing the leading class to `(^|[[:space:]]|[;&|(])` and bolting on an
# optional `([^[:space:];&|()]*/)?` path prefix. It closed the path-qualified hole and SILENTLY
# OPENED FIVE OTHERS: backtick, `'`, `"`, `=` and `\` stopped being opening characters, so
# `\curl ...`, ``BODY=`curl ...` ``, `bash -c 'curl ...'` and `bash -c 'wget ...'` -- all of which
# v1 killed -- began to survive. A mutation count is evidence about the mutants you wrote; it is
# not evidence that nothing regressed. Changing a matcher means re-running the PRIOR round's
# mutants against the new matcher, which is what fetcher_selftest() below now does automatically,
# on every run, in both directions.
#
# v3 (this one) needs no path prefix at all, which is why it is both wider and simpler: `/` is
# ITSELF a non-word character, so restoring the permissive leading class `[^[:alnum:]_]` matches
# `/usr/bin/curl` on the `/` immediately before the name. The trailing class `[^[:alnum:]_./-]`
# then rejects `curl-config`, `curl.se` and `curl_x` while accepting the space, `)`, `"`, `'`,
# `;` and `|` that really do terminate a command word.
#
# WHY A SENTINEL SPACE INSTEAD OF `(^|...)`. POSIX leaves `^` UNDEFINED anywhere but the start
# of an ERE, so `(^|CLASS)` is relying on an extension, and implementations really do differ:
# alternating `^` with a NEGATED bracket expression is the case that breaks -- an engine that
# lets the anchor swallow the group makes `(^|[^[:alnum:]_./-])curl` match nothing at all, not
# even a line beginning with `curl`. Alternating `^` with an ordinary class does NOT diverge:
# `(^|[[:space:]])curl` behaves identically everywhere tested, which is why curl_command_lines()
# below can keep its `(^|[[:space:]])` and is not covered by this warning.
#
# Scope of the risk, stated exactly rather than dramatised: every gate here runs under `bash`
# with GNU grep, so this has never mis-fired in CI or on this machine. The construct is dropped
# because it is undefined behaviour that a future runner could resolve differently, and because
# removing it costs nothing -- fetcher_lines() prepends one space to every logical line, so every
# command-position occurrence has a real preceding character and no `^` alternative is needed.
#
# The sentinel form also happens to be far better at the actual job, which is the real argument
# for it. Measured against the 14-spelling corpus below: v1 matched 6/14 under GNU grep 3.12 and
# v2 matched 7/14 -- v2 traded five spellings for two -- while v3 matches 14/14.
#
# WHAT IT MATCHES, verified by fetcher_selftest() rather than asserted here: a bare `curl`, a
# path-qualified one (`/usr/bin/curl`, `./curl`, `"${HOME}"/bin/curl`, `$(dirname "$0")/curl`),
# a quoted one (`"/usr/bin/curl"`), a backslash-escaped one (`\curl`), one inside a command
# substitution or backticks, one inside `bash -c '...'`, and `python -m urllib`.
#
# KNOWN PERMISSIVE, both cases stated rather than discovered later. Neither is a silent pass:
# both fail RED, which a human resolves in a minute, and both are the price of catching
# `/usr/bin/curl` -- not worth trading back.
#   * A URL path and a command path are textually identical, so a string holding a URL that ENDS
#     in /curl -- `echo "get it from https://example.com/bin/curl"` -- matches.
#   * logical_lines() strips a trailing comment only when NOTHING on the line is quoted (it must
#     be conservative; see the comment there). So a QUOTED line with a trailing comment that
#     names a path ending in a fetcher -- `echo "polaris installed"  # see hack/tools/curl` --
#     survives the strip and matches.
#
# NOT used for the flag assertions on hack/lib/fetch.sh. That is a SCOPING guarantee, not a
# property of this pattern: fetch_to's own diagnostics contain the words "(curl exit ${status})"
# and "(last curl exit ${status})", both of which this pattern matches. See curl_command_lines().
FETCHER_RE='[^[:alnum:]_]((curl|wget|aria2c)([^[:alnum:]_./-]|$)|python3?[[:space:]]+-m[[:space:]]+urllib)'

# Every FETCHER_RE scan goes through this, so the sentinel space can never be forgotten at one
# call site and remembered at another.
fetcher_lines() {
  logical_lines "$1" | sed 's/^/ /'
}

# The regression harness the v1 -> v2 narrowing needed and did not have. Each spelling below is a
# real mutant some reviewer or some round actually wrote; running them on every gate invocation
# is what makes "I re-ran the prior mutants" a property of the file rather than a claim in a
# commit message.
#
# REVIEW FIX (round 4, P1). The first version of this harness applied FETCHER_RE to
# `printf ' %s\n' "${line}"` -- it RE-IMPLEMENTED the sentinel instead of calling the production
# path. So it pinned the pattern and left the pipeline that applies it completely unprotected:
# delete the `| sed 's/^/ /'` from fetcher_lines() and every assertion below still passed, while
# a bare `curl -fsSL ...` at COLUMN 0 in an installer -- the literal PR #351 defect -- shipped
# green. That is the round-2 failure mode (change the matching machinery, prior mutants silently
# survive) reproduced one layer up, inside the mechanism added to prevent it. The corpus now goes
# through a real file and a real fetcher_lines() call, which covers logical_lines() too, and the
# column-0 entry below is what makes the sentinel itself load-bearing in this suite.
fetcher_selftest() {
  local want="$1"
  shift
  local line hits fixture
  fixture="$(mktemp)"
  for line in "$@"; do
    printf '%s\n' "${line}" >"${fixture}"
    hits="$(fetcher_lines "${fixture}" | count_matches -E "${FETCHER_RE}")"
    require_count "${hits}" "FETCHER_RE self-test"
    if [[ "${want}" == "match" ]]; then
      [[ "${hits}" != "0" ]] ||
        fail "FETCHER_RE self-test: this spelling of a direct fetcher is NOT matched by fetcher_lines | FETCHER_RE, so an installer could use it to bypass hack/lib/fetch.sh with this gate green: ${line}"
    else
      [[ "${hits}" == "0" ]] ||
        fail "FETCHER_RE self-test: this line invokes no fetcher but matches, so the pattern has widened into a false-positive class: ${line}"
    fi
  done
  rm -f "${fixture}"
}

# The first entry sits at COLUMN 0 of the fixture, which is what makes the sentinel in
# fetcher_lines() load-bearing here: without it this line has no preceding character and
# FETCHER_RE cannot match, so removing the sentinel reds this suite instead of silently
# unprotecting every installer scan.
fetcher_selftest match \
  'curl -fsSL x' \
  '/usr/bin/curl -fsSL x' \
  './curl x' \
  '"/usr/bin/curl" -fsSL x' \
  '"${HOME}"/bin/curl x' \
  '$(dirname "$0")/curl -fsSL x' \
  '\curl -fsSL x' \
  'BODY=`curl -fsSL x`' \
  'X=$(curl x)' \
  '"$(command -v curl)" -fsSL x' \
  "bash -c 'curl -fsSL \"\$1\" -o \"\$2\"' _ a b" \
  "bash -c 'wget -O \"\$1\" \"\$2\"' _ a b" \
  '/usr/bin/wget -O a b' \
  'python3 -m urllib.request x' \
  'python -m urllib.request x' \
  'aria2c -o out "${DOWNLOAD_URL}"' \
  'env curl -fsSL x' \
  'eval "curl -fsSL x"'

fetcher_selftest no-match \
  'echo curly' \
  'curl-config --version' \
  'foo_curl x' \
  'mycurl x' \
  'see https://curl.se/ docs' \
  'fetch_to "${U}" "${D}" "helm tarball"'
pass "fetcher_lines | FETCHER_RE matches all 18 known bypass spellings (column 0 included) and none of the 6 lookalikes"

[[ -f "${LIB}" ]] ||
  fail "${LIB} is missing -- the whole point of CI-FETCHLIB-01 is that the hardened flag list lives in exactly ONE place"
[[ -s "${LIB}" ]] ||
  fail "${LIB} is empty -- every assertion below would pass vacuously"

# ================================================================================================
# PART 1 -- behavioural: exercise fetch_to against a real local HTTPS origin.
# ================================================================================================

for tool in python3 openssl curl; do
  command -v "${tool}" >/dev/null 2>&1 ||
    fail "${tool} is required for the behavioural half of this gate; it is present on ubuntu-latest runners and must not be skipped silently -- a skipped behavioural half would leave only static greps, which cannot tell a hardened fetch from a hardened-looking one"
done

TMPROOT="$(mktemp -d)"
SERVER_PID=""
cleanup() {
  [[ -n "${SERVER_PID}" ]] && kill "${SERVER_PID}" 2>/dev/null || true
  rm -rf "${TMPROOT}"
}
trap cleanup EXIT

CTL="${TMPROOT}/ctl"
mkdir -p "${CTL}"

# A self-signed cert for 127.0.0.1. The helper pins --proto '=https', so a plain-HTTP fixture
# could not exercise it at all -- the pin would reject the fixture rather than the defect, and
# every case would pass for the wrong reason. CURL_CA_BUNDLE (honoured by curl, and NOT a
# weakening of the helper: the helper's own flags are untouched) is what lets a real TLS
# handshake succeed against a throwaway CA.
openssl req -x509 -newkey rsa:2048 -sha256 -days 1 -nodes \
  -keyout "${TMPROOT}/key.pem" -out "${TMPROOT}/cert.pem" \
  -subj "/CN=127.0.0.1" -addext "subjectAltName=IP:127.0.0.1" >/dev/null 2>&1 ||
  fail "could not generate a self-signed certificate for the local HTTPS fixture"
cat "${TMPROOT}/cert.pem" "${TMPROOT}/key.pem" >"${TMPROOT}/chain.pem"

cat >"${TMPROOT}/server.py" <<'FIXTURE_SERVER'
"""Local origin for the CI-FETCHLIB-01 behavioural cases.

Serves an HTTPS listener (the origin the helper is allowed to talk to) and a PLAINTEXT HTTP
listener that exists only to be a trap: nothing the helper does may ever reach it. Its request
log is asserted empty, which is how "--proto/--proto-redir actually pin the scheme" becomes an
observed fact rather than a flag spelled correctly on a command line.
"""
import http.server
import os
import ssl
import sys
import threading

CTL = sys.argv[1]
CHAIN = sys.argv[2]
PORTFILE = sys.argv[3]


def bump(name):
    path = os.path.join(CTL, "count_" + name)
    n = 0
    if os.path.exists(path):
        n = int(open(path).read().strip() or 0)
    n += 1
    open(path, "w").write(str(n))
    return n


def budget(name):
    path = os.path.join(CTL, name + "_fail_first")
    if not os.path.exists(path):
        return 0
    return int(open(path).read().strip() or 0)


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    scheme = "https"

    def log_message(self, *_args):
        pass

    def record(self):
        with open(os.path.join(CTL, "requests_%s.log" % self.scheme), "a") as fh:
            fh.write("%s %s\n" % (self.command, self.path))

    def body(self, status, payload, ctype="application/octet-stream"):
        self.send_response(status)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self):
        self.record()
        path = self.path.split("?")[0]

        if path == "/ok":
            bump("ok")
            self.body(200, b"OK-BODY\n")
            return

        if path == "/flaky500":
            # A 5xx is exactly the class curl's OWN --retry covers. The shell-level sleep stub
            # records nothing here, which is what proves the recovery came from curl.
            n = bump("flaky500")
            if n <= budget("flaky500"):
                self.body(503, b"upstream busy\n", "text/plain")
                return
            self.body(200, b"OK-BODY\n")
            return

        if path == "/reset":
            # No response at all, connection dropped: curl exit 52. Like exit 7 (a failed
            # connect) this is NOT in the set curl's --retry covers, so only the OUTER bash loop
            # can recover from it. That asymmetry is the load-bearing claim of this lane.
            n = bump("reset")
            if n <= budget("reset"):
                self.close_connection = True
                try:
                    self.connection.close()
                except OSError:
                    pass
                return
            self.body(200, b"OK-BODY\n")
            return

        if path == "/truncate":
            # Content-Length promises far more than is written, then the socket closes:
            # curl exit 18. Proves --max-time/-f are not the only thing standing between a
            # short read and a "successfully downloaded" partial file.
            bump("truncate")
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Length", "100000")
            self.end_headers()
            self.wfile.write(b"TRUNCATED-")
            self.wfile.flush()
            self.close_connection = True
            try:
                self.connection.close()
            except OSError:
                pass
            return

        if path == "/notfound":
            bump("notfound")
            self.body(404, b"<html>not here</html>\n", "text/html")
            return

        if path == "/redir-http":
            # The downgrade attempt: a 302 from the https origin to the plaintext listener.
            bump("redir")
            plain = open(os.path.join(CTL, "plain_port")).read().strip()
            self.send_response(302)
            self.send_header("Location", "http://127.0.0.1:%s/ok" % plain)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return

        self.body(404, b"unknown fixture path\n", "text/plain")


class PlainHandler(Handler):
    scheme = "http"


ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain(CHAIN)

https_srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
https_srv.socket = ctx.wrap_socket(https_srv.socket, server_side=True)
http_srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), PlainHandler)

open(os.path.join(CTL, "plain_port"), "w").write(str(http_srv.server_address[1]))
with open(PORTFILE, "w") as fh:
    fh.write("%d %d\n" % (https_srv.server_address[1], http_srv.server_address[1]))

threading.Thread(target=http_srv.serve_forever, daemon=True).start()
https_srv.serve_forever()
FIXTURE_SERVER

python3 "${TMPROOT}/server.py" "${CTL}" "${TMPROOT}/chain.pem" "${TMPROOT}/ports" &
SERVER_PID=$!

for _ in $(seq 1 100); do
  [[ -s "${TMPROOT}/ports" ]] && break
  sleep 0.1
done
[[ -s "${TMPROOT}/ports" ]] ||
  fail "the local HTTPS fixture never reported its port -- every behavioural case below would be untestable, so this gate refuses to continue rather than degrade into static greps"
read -r HTTPS_PORT HTTP_PORT <"${TMPROOT}/ports"
BASE="https://127.0.0.1:${HTTPS_PORT}"

# A port that nothing is listening on, for the connect-refused cases. Bound and released by
# python so it is genuinely free rather than a guess that might collide with the fixture.
DEAD_PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));p=s.getsockname()[1];s.close();print(p)')"

# The shell-level `sleep` is stubbed so the outer retry backoff costs milliseconds instead of
# 20s per case -- and, more importantly, so "did the OUTER loop run?" becomes observable.
# curl's own retries sleep INSIDE curl and never reach this stub, which is precisely the
# discrimination Case B and Case C rest on.
STUB_BIN="${TMPROOT}/stubbin"
mkdir -p "${STUB_BIN}"
cat >"${STUB_BIN}/sleep" <<'STUB_SLEEP'
#!/usr/bin/env bash
printf '%s\n' "${1:-}" >>"${KOLLECT_SLEEP_LOG}"
exit 0
STUB_SLEEP
chmod +x "${STUB_BIN}/sleep"

CASE_DIR=""
new_case() {
  CASE_DIR="${TMPROOT}/cases/$1"
  mkdir -p "${CASE_DIR}"
  rm -f "${CTL}"/count_* "${CTL}"/*_fail_first
  # Truncated rather than removed: plain_hits() below counts lines in requests_http.log, and a
  # MISSING file would make that count unreadable at exactly the moment the assertion wants to
  # read zero -- the shape in which "the plaintext origin was never contacted" passes because
  # the evidence is absent rather than because the event did not happen.
  : >"${CTL}/requests_http.log"
  : >"${CTL}/requests_https.log"
  : >"${CASE_DIR}/sleeps"
}

# Runs fetch_to in a CHILD bash that sources the helper the same way an installer does --
# `set -euo pipefail` included -- so the helper is exercised under the shell settings it will
# really run under, not under this gate's.
run_fetch() {
  local url="$1" dest="$2" what="${3:-artifact}" status=0
  CURL_CA_BUNDLE="${TMPROOT}/cert.pem" \
    KOLLECT_SLEEP_LOG="${CASE_DIR}/sleeps" \
    PATH="${STUB_BIN}:${PATH}" \
    bash -c '
      set -euo pipefail
      # shellcheck source=/dev/null
      source "$1"
      fetch_to "$2" "$3" "$4"
    ' _ "${LIB}" "${url}" "${dest}" "${what}" >"${CASE_DIR}/out" 2>"${CASE_DIR}/err" || status=$?
  printf '%s\n' "${status}" >"${CASE_DIR}/status"
  return 0
}

case_status() { cat "${CASE_DIR}/status"; }
case_err() { cat "${CASE_DIR}/err"; }
hits() { cat "${CTL}/count_$1" 2>/dev/null || printf '0\n'; }
sleeps() { wc -l <"${CASE_DIR}/sleeps" | tr -d '[:space:]'; }
plain_hits() {
  [[ -f "${CTL}/requests_http.log" ]] ||
    fail "the plaintext-origin request log is missing -- new_case() did not run, so 'the plaintext listener was never contacted' would be asserted against no evidence at all"
  wc -l <"${CTL}/requests_http.log" | tr -d '[:space:]'
}

# --- Case A: happy path ------------------------------------------------------------------------
# Also the harness's own smoke test: if TLS, the CA bundle or the fixture were broken, every
# NEGATIVE case below would still "pass" -- for the wrong reason -- so this case is what stops
# the rest of Part 1 from being vacuous.
new_case happy
run_fetch "${BASE}/ok" "${CASE_DIR}/artifact" "fixture artifact"
[[ "$(case_status)" == "0" ]] ||
  fail "happy path: fetch_to exited $(case_status) against a healthy local HTTPS origin -- the fixture or the helper is broken, and every negative case below would then pass vacuously: $(case_err)"
[[ "$(cat "${CASE_DIR}/artifact")" == "OK-BODY" ]] ||
  fail "happy path: fetch_to exited 0 but wrote $(wc -c <"${CASE_DIR}/artifact") bytes that are not the served body"
[[ "$(hits ok)" == "1" ]] ||
  fail "happy path: expected exactly one request, got $(hits ok) -- a helper that retries a SUCCESS is hammering origins for no reason"
pass "fetch_to downloads over real TLS and writes the served body exactly once"

# --- Case B: curl's own --retry covers a 5xx ---------------------------------------------------
# Mutation caught: dropping `--retry <n>` (or setting it to 0). The outer loop cannot mask this,
# because the assertion is that ZERO shell sleeps happened: recovery had to come from inside
# curl. Without --retry the single 503 is fatal, the outer loop takes over, and the sleep count
# goes to 1 -- red, with a message that says exactly which layer went missing.
new_case flaky-5xx
printf '1\n' >"${CTL}/flaky500_fail_first"
run_fetch "${BASE}/flaky500" "${CASE_DIR}/artifact" "flaky artifact"
[[ "$(case_status)" == "0" ]] ||
  fail "flaky 5xx: a single 503 followed by a 200 must be survived; fetch_to exited $(case_status): $(case_err)"
[[ "$(hits flaky500)" == "2" ]] ||
  fail "flaky 5xx: expected the origin to see 2 requests (one 503, one 200), got $(hits flaky500)"
[[ "$(sleeps)" == "0" ]] ||
  fail "flaky 5xx: the recovery used $(sleeps) shell-level sleep(s), so it came from the OUTER loop, not from curl's own --retry. curl retries 5xx natively and far faster; if --retry is missing or 0 the outer loop silently papers over it and the two layers stop being independent"
pass "a transient 5xx is absorbed by curl's own --retry, with no outer-loop backoff"

# --- Case C: the outer loop covers what --retry does NOT ---------------------------------------
# THE central claim of this lane, and the one that cost PR #351 an hour. curl's --retry covers
# "transient" HTTP conditions -- 408/429/5xx and timeouts -- but NOT a transport failure with no
# response, unless --retry-connrefused/--retry-all-errors is passed. `curl: (7)` is that class,
# and so is the exit 52 this case injects. So the outer loop is not redundant decoration around
# --retry: it is the half that covers the failure that ACTUALLY happened.
#
# Mutation caught: deleting the outer loop. --retry is still on the command line, Case B still
# passes, and this case reds -- which is the whole reason the two cases are separate.
new_case reset-transient
printf '1\n' >"${CTL}/reset_fail_first"
run_fetch "${BASE}/reset" "${CASE_DIR}/artifact" "reset artifact"
[[ "$(case_status)" == "0" ]] ||
  fail "transient transport failure: a dropped connection with no HTTP response is NOT covered by curl's --retry (only --retry-connrefused/--retry-all-errors would), so the outer bash loop must recover it. fetch_to exited $(case_status): $(case_err)"
[[ "$(hits reset)" == "2" ]] ||
  fail "transient transport failure: expected 2 requests, got $(hits reset)"
(($(sleeps) >= 1)) ||
  fail "transient transport failure: fetch_to retried without backing off -- a loop with no sleep hammers an origin that is already failing"
# REVIEW FIX (round 2, P3). The line above counts sleep INVOCATIONS. The stub logs its argument
# whatever that argument is, so `KOLLECT_FETCH_RETRY_DELAY=0` -- `sleep 0`, a backoff that does
# not back off -- survived the entire suite while this case's own failure text claimed to forbid
# it. The delay is the property; the call is only its carrier.
first_delay="$(head -1 "${CASE_DIR}/sleeps")"
[[ "${first_delay}" =~ ^[0-9]+$ ]] ||
  fail "transient transport failure: the sleep stub recorded '${first_delay}' rather than a number of seconds -- the backoff assertion below cannot be evaluated, so it must not be allowed to pass"
((first_delay >= 1)) ||
  fail "transient transport failure: fetch_to backed off for ${first_delay}s. \`sleep 0\` is a call, not a backoff: it hammers an origin that is already refusing, which is the behaviour the retry loop exists to avoid"
pass "a dropped connection (the exit-7/exit-52 class curl's --retry ignores) is recovered by the outer loop, with a backoff"

# --- Case D: permanent connect refusal fails accurately, and is bounded ------------------------
# CI-KINDLOOP-01's defect, asserted at the source. The kind loop falls THROUGH after its last
# attempt onto `chmod +x` of a file that was never written; `set -euo pipefail` still fails the
# job, so it is not a false green, but the operator is handed "No such file or directory"
# instead of "could not reach the host". Here the failure must be raised AT the fetch, name the
# host, and carry curl's exit code so the class of failure is readable without a re-run.
new_case connect-refused
run_fetch "https://127.0.0.1:${DEAD_PORT}/ok" "${CASE_DIR}/artifact" "unreachable artifact"
[[ "$(case_status)" != "0" ]] ||
  fail "permanent connect refusal: fetch_to exited 0 with nothing downloaded -- the loop swallowed its final failure, which is how a fall-through bug is born"
[[ ! -e "${CASE_DIR}/artifact" ]] ||
  fail "permanent connect refusal: a destination file was left behind. Every caller checksums or untars this path next; a zero-byte leftover turns a network failure into a confusing digest or gzip error"
grep -Fq "127.0.0.1:${DEAD_PORT}" <<<"$(case_err)" ||
  fail "permanent connect refusal: the diagnosis must name the host that could not be reached -- that is the entire difference between CI-KINDLOOP-01's 'No such file or directory' and a two-minute triage. Got: $(case_err)"
grep -Eq 'curl exit [0-9]+' <<<"$(case_err)" ||
  fail "permanent connect refusal: the diagnosis must carry curl's exit code, so 'unreachable host' (7) is distinguishable from 'TLS failed' (35/60) or 'truncated' (18) without re-running the job. Got: $(case_err)"
[[ "$(sleeps)" == "2" ]] ||
  fail "permanent connect refusal: expected 2 backoffs across 3 bounded attempts, got $(sleeps). More means an unbounded loop; 3 means the loop slept after its FINAL attempt -- the pointless 10s the kind loop burns before failing anyway"

# REVIEW FIX (round 2, P3). The two assertions above are satisfied by the PER-ATTEMPT retry
# messages, which also name the host and carry curl's exit code. So deleting fetch_to's closing
# summary -- the "N attempts to <host> all failed (last curl exit N)" line, the single thing this
# lane advertises as the answer to CI-KINDLOOP-01's `chmod: No such file or directory` -- left
# every case green. What distinguishes a summary from a retry notice is that it is the LAST thing
# said and it does not promise another attempt: an operator scrolling to the end of a failed job
# must land on "this is over, and here is why", not on "retrying in 10s..." from an attempt that
# never came.
# awk, not `grep -v ... | tail -1`: under `set -euo pipefail` grep exits 1 when stderr is
# EMPTY, pipefail propagates it, and a bare assignment then aborts the gate with no diagnostic --
# so the "nothing at all on stderr" guard below was unreachable in exactly the case it exists
# for. It failed red, so never a false green; it just lost its message. awk exits 0 either way.
last_err_line="$(awk 'NF { line = $0 } END { print line }' <<<"$(case_err)")"
[[ -n "${last_err_line}" ]] ||
  fail "permanent connect refusal: fetch_to failed silently -- nothing at all on stderr"
grep -Fq "127.0.0.1:${DEAD_PORT}" <<<"${last_err_line}" ||
  fail "permanent connect refusal: the LAST line of the diagnosis does not name the host. Got: ${last_err_line}"
grep -Eq 'curl exit [0-9]+' <<<"${last_err_line}" ||
  fail "permanent connect refusal: the LAST line of the diagnosis does not carry curl's exit code, so the closing summary has been dropped and the final word on a doomed download is a per-attempt notice. Got: ${last_err_line}"
! grep -Eq 'retry|retrying' <<<"${last_err_line}" ||
  fail "permanent connect refusal: the diagnosis ENDS on a retry notice, so the operator's last line reads as though another attempt were coming when the loop had already given up. Got: ${last_err_line}"
pass "an unreachable origin fails after bounded attempts, naming the host and curl's exit code, with no sleep after the last attempt"

# --- Case E: an HTTP error page is never mistaken for content ----------------------------------
# Mutation caught: dropping -f/--fail. Without it curl writes the 404 body to the output file
# and exits 0; the caller then checksums an HTML error page. The digest would not match, so it
# fails -- but it fails as "checksum mismatch", pointing the reader at a corrupt release when
# the truth is that the URL is wrong.
new_case http-404
run_fetch "${BASE}/notfound" "${CASE_DIR}/artifact" "missing artifact"
[[ "$(case_status)" != "0" ]] ||
  fail "HTTP 404: fetch_to exited 0 on a 404 -- -f/--fail is missing, so an error page is being treated as a successful download"
[[ ! -e "${CASE_DIR}/artifact" ]] ||
  fail "HTTP 404: the error page was left on disk at the destination path, where the caller will checksum or untar it"
pass "an HTTP error response fails the fetch and leaves nothing on disk"

# --- Case F: a truncated body is a failure, not a short file -----------------------------------
new_case truncated
run_fetch "${BASE}/truncate" "${CASE_DIR}/artifact" "truncated artifact"
[[ "$(case_status)" != "0" ]] ||
  fail "truncated body: fetch_to exited 0 on a transfer that ended early -- a partial tarball would reach tar/sha256sum as if it were complete"
[[ ! -e "${CASE_DIR}/artifact" ]] ||
  fail "truncated body: the partial download was left at the destination path"
pass "a truncated transfer fails and the partial file is discarded"

# --- Case G: a redirect may not downgrade to plaintext -----------------------------------------
# Mutation caught: dropping the scheme pins so a 302 to http:// is followed. The trap is the
# plaintext listener: without a pin curl follows the redirect and fetches the body over http, so
# the assertion that the plain server saw ZERO requests is the one that discriminates. Asserting
# only "exit non-zero" would not -- a downgraded fetch SUCCEEDS.
#
# WHAT THIS CASE DOES NOT PROVE, established by mutation rather than assumed: it does not
# isolate --proto-redir. curl applies --proto to the redirect chain too, so deleting
# --proto-redir alone leaves this case green; only deleting BOTH pins turns the plaintext
# listener's counter non-zero. --proto-redir is therefore defence in depth -- it is what keeps
# the redirect pinned if --proto is ever widened -- and the assertion that it is PRESENT lives
# in Part 2 with the rest of the flag list, which is the honest place for a flag whose value
# cannot be observed here.
#
# This matters most for the checksum fetch: a digest retrieved over plaintext verifies nothing,
# because anyone who can rewrite the tarball can rewrite the digest to match it.
new_case redirect-downgrade
run_fetch "${BASE}/redir-http" "${CASE_DIR}/artifact" "redirected artifact"
[[ "$(case_status)" != "0" ]] ||
  fail "https->http redirect: fetch_to exited 0, so it FOLLOWED a redirect down to plaintext. A checksum fetched over http verifies nothing at all"
[[ "$(plain_hits)" == "0" ]] ||
  fail "https->http redirect: the plaintext listener received $(plain_hits) request(s) -- neither --proto '=https' nor --proto-redir '=https' is holding, and the body was fetched over http"
[[ ! -e "${CASE_DIR}/artifact" ]] ||
  fail "https->http redirect: a body was written despite the downgrade being refused"
pass "a redirect to plaintext http is refused and the plaintext origin is never contacted"

# --- Case H: a plaintext URL is refused outright -----------------------------------------------
# Mutation caught: dropping --proto '=https' (or adding --proto-default http). Distinct from
# Case G: that one guards the REDIRECT chain, this one guards the initial request, and dropping
# either flag leaves the other case green.
new_case plaintext-url
run_fetch "http://127.0.0.1:${HTTP_PORT}/ok" "${CASE_DIR}/artifact" "plaintext artifact"
[[ "$(case_status)" != "0" ]] ||
  fail "plaintext URL: fetch_to exited 0 on an http:// URL -- --proto '=https' is missing, so a mistyped or downgraded BASE_URL silently fetches over plaintext"
[[ "$(plain_hits)" == "0" ]] ||
  fail "plaintext URL: the plaintext listener received $(plain_hits) request(s) -- the scheme pin is not holding"
pass "an http:// URL is refused before a request is issued"

# ================================================================================================
# PART 2 -- static, repo-wide: every installer routes its fetches through the helper.
# ================================================================================================

# The helper's own flag list, read as logical lines. Part 1 proves the BEHAVIOUR; these
# assertions pin the specific flags so that a future rewrite which happens to still pass the
# behavioural cases (e.g. because the fixture is fast and local) cannot quietly drop a bound
# that only matters against a real, slow, hostile network.
mapfile -t LIB_CURL_LINES < <(curl_command_lines "${LIB}")
((${#LIB_CURL_LINES[@]} > 0)) ||
  fail "${LIB} contains no curl invocation -- either the fetch was replaced by something this gate does not inspect, or the logical-line extraction is broken; either way the per-flag assertions below would pass vacuously"

for line in "${LIB_CURL_LINES[@]}"; do
  grep -Eq -- "--proto[[:space:]]+'?=https'?" <<<"${line}" ||
    fail "the shared fetch helper's curl invocation is missing --proto '=https': ${line}"
  grep -Eq -- "--proto-redir[[:space:]]+'?=https'?" <<<"${line}" ||
    fail "the shared fetch helper's curl invocation is missing --proto-redir '=https': ${line}"
  grep -Eq -- '(^|[[:space:]])--tlsv1\.[23]([[:space:]]|$)' <<<"${line}" ||
    fail "the shared fetch helper's curl invocation lost its TLS floor (--tlsv1.2): ${line}"
  grep -Eq -- '(^|[[:space:]])(-[a-zA-Z]*f[a-zA-Z]*|--fail)([[:space:]]|$)' <<<"${line}" ||
    fail "the shared fetch helper's curl invocation lost -f/--fail, so an HTTP error page would be treated as a successful download: ${line}"
  ! grep -Eq -- '(^|[[:space:]])(-k|--insecure|--proto-default[[:space:]]+http)([[:space:]]|$)' <<<"${line}" ||
    fail "the shared fetch helper disables TLS verification or defaults to plaintext, which makes --proto '=https' worthless: ${line}"

  # VALUE floors, not presence checks: `--retry 0`, `--connect-timeout 0` and
  # `--retry-max-time 0` are all textually present and behaviourally absent or unbounded.
  for flag in retry connect-timeout max-time retry-max-time; do
    value="$(grep -Eo -- "--${flag}[[:space:]]+[0-9]+" <<<"${line}" | head -1 | grep -Eo '[0-9]+' || true)"
    [[ -n "${value}" ]] ||
      fail "the shared fetch helper's curl invocation is missing '--${flag} <n>': ${line}"
    case "${flag}" in
      retry | connect-timeout)
        ((value >= 1)) ||
          fail "the shared fetch helper sets '--${flag} ${value}', which disables it (0 means 'no retries' / 'no timeout'): ${line}"
        ;;
      max-time | retry-max-time)
        # curl RESETS --max-time on each of its own retries, so --max-time alone bounds an
        # attempt, not the invocation; --retry-max-time is what closes that. The >= 30 floor
        # guards the opposite direction -- a timeout set too tight is itself a flake source, and
        # `--retry-max-time 1` expires the window before --retry-delay can schedule one retry,
        # neutering the very layer Case B proves.
        ((value >= 30)) ||
          fail "the shared fetch helper sets '--${flag} ${value}': under 30s this is either unbounded (0) or tight enough to abort a healthy multi-megabyte download and become a new flake source: ${line}"
        ;;
    esac
  done
done
pass "the shared helper's curl invocation pins https end to end, keeps a TLS floor, retries, and bounds both the connect and the transfer"

# Enumerated from the FILESYSTEM, never from a hardcoded list: installer number nine is covered
# the day it lands, which is the entire reason this gate exists rather than seven copies of the
# helm gate.
mapfile -t INSTALLERS < <(find "${ROOT}/hack" -maxdepth 1 -name 'install-*.sh' -type f | sort)
((${#INSTALLERS[@]} >= 8)) ||
  fail "expected at least 8 hack/install-*.sh scripts, found ${#INSTALLERS[@]} -- the glob is broken or the installers moved, and every per-installer assertion below would pass vacuously"
pass "found ${#INSTALLERS[@]} installer script(s) to inspect"

for installer in "${INSTALLERS[@]}"; do
  rel="hack/$(basename "${installer}")"

  grep -Eq '^set -euo pipefail[[:space:]]*$' "${installer}" ||
    fail "${rel} must keep 'set -euo pipefail' -- without it a failed fetch falls through to tar/install instead of aborting"

  # THE repo-wide assertion. A direct fetcher in an installer is, by construction, a fetch that
  # does not carry the helper's flags -- that is the defect PR #351 died of, seven times over.
  fetcher_hits="$(fetcher_lines "${installer}" | count_matches -E "${FETCHER_RE}")"
  require_count "${fetcher_hits}" "${rel} direct-fetcher scan"
  [[ "${fetcher_hits}" == "0" ]] ||
    fail "${rel} invokes a fetcher directly (${fetcher_hits} occurrence(s)) instead of going through fetch_to from hack/lib/fetch.sh. That is exactly the bare-curl defect that reddened the REQUIRED kind-smoke check on PR #351: no retry, no connect bound, no protocol pin. Source hack/lib/fetch.sh and call fetch_to"

  # A script with no fetch at all would pass the assertion above vacuously, so the positive half
  # is required too: if it is named install-*, it downloads something, and that download goes
  # through the helper.
  source_hits="$(logical_lines "${installer}" | count_matches -F 'hack/lib/fetch.sh')"
  require_count "${source_hits}" "${rel} helper-source scan"
  [[ "${source_hits}" != "0" ]] ||
    fail "${rel} does not source hack/lib/fetch.sh -- either it fetches by some means this gate cannot see, or the hardened flag list has been forked back into a second copy"

  call_hits="$(logical_lines "${installer}" | count_matches -E '(^|[^[:alnum:]_])fetch_to[[:space:]]')"
  require_count "${call_hits}" "${rel} fetch_to call scan"
  [[ "${call_hits}" != "0" ]] ||
    fail "${rel} sources hack/lib/fetch.sh but never calls fetch_to -- a sourced-but-unused helper is decoration"

  plaintext_hits="$(logical_lines "${installer}" | count_matches -F 'http://')"
  require_count "${plaintext_hits}" "${rel} plaintext scan"
  [[ "${plaintext_hits}" == "0" ]] ||
    fail "${rel} references a plaintext http:// URL; every fetch in an installer must be https"
done
pass "every hack/install-*.sh sources the shared helper, fetches only through fetch_to, and names no plaintext URL"

# The other direction: nothing may weaken the digest verification while routing through the
# helper. Each installer either compares a sha256 itself or delegates to verify_sha256.
# hack/install-git-cliff.sh and hack/install-helm-docs.sh are the two that verify NOTHING today;
# they are listed here explicitly so the gap is recorded in code rather than lost in a report,
# and so adding verification to them is a one-line deletion from this list rather than a
# rediscovery.
UNVERIFIED_BY_DESIGN=("install-git-cliff.sh" "install-helm-docs.sh")
for installer in "${INSTALLERS[@]}"; do
  base="$(basename "${installer}")"
  skip=0
  for known in "${UNVERIFIED_BY_DESIGN[@]}"; do
    [[ "${base}" == "${known}" ]] && skip=1
  done
  ((skip == 1)) && continue
  # Counts the COMPARISON, not the variable that feeds it. The first version of this scan
  # matched `EXPECTED_SHA256` too, and a mutant that deleted `verify_sha256 ...` while leaving
  # `EXPECTED_SHA256=` assigned somewhere in the file passed it -- an installer with a digest
  # variable and no digest check, which is the exact shape this assertion exists to forbid.
  verify_hits="$(logical_lines "${installer}" |
    count_matches -E '(^|[^[:alnum:]_])(verify_sha256|sha256sum)([[:space:]]|$)')"
  require_count "${verify_hits}" "hack/${base} checksum scan"
  [[ "${verify_hits}" != "0" ]] ||
    fail "hack/${base} no longer verifies a SHA256. Routing a download through a hardened helper must never be traded against verifying what came back: the helper protects the transport, the digest protects the CONTENT, and neither substitutes for the other"
done
pass "every installer that verified a SHA256 before still does"

# Guards the list above from rotting into a licence to stop verifying: if someone ADDS
# verification to git-cliff or helm-docs, the entry must be removed, or the next reader will
# believe those two are still unverified.
for known in "${UNVERIFIED_BY_DESIGN[@]}"; do
  [[ -f "${ROOT}/hack/${known}" ]] ||
    fail "hack/${known} is listed as unverified-by-design but does not exist -- the exemption list is stale and now silently exempts nothing"
  verify_hits="$(logical_lines "${ROOT}/hack/${known}" |
    count_matches -E '(^|[^[:alnum:]_])(verify_sha256|sha256sum)([[:space:]]|$)')"
  require_count "${verify_hits}" "hack/${known} exemption scan"
  [[ "${verify_hits}" == "0" ]] ||
    fail "hack/${known} now verifies a checksum but is still listed in UNVERIFIED_BY_DESIGN in this gate -- drop it from the list so the exemption stops advertising a gap that has been closed"
done
pass "the unverified-by-design exemption list (${UNVERIFIED_BY_DESIGN[*]}) still matches reality"

# ================================================================================================
# PART 3 -- CI-KINDLOOP-01: the composite action's kind download.
# ================================================================================================

ACTION="${ROOT}/.github/actions/kind-e2e-setup/action.yml"
[[ -f "${ACTION}" ]] || fail "${ACTION} is missing"

action_lines() { logical_lines "${ACTION}"; }

# The original defect, stated as a shape: a `for attempt in ...` loop whose body ends without
# raising a failure, so control reaches the next command with no downloaded file. Requiring the
# helper instead of hand-rolling a fourth copy of the hardening is the assertion; it is also the
# only form that keeps the action and the installers from drifting apart the way get.helm.sh and
# kind did inside this very step.
action_fetcher_hits="$(fetcher_lines "${ACTION}" | count_matches -E "${FETCHER_RE}")"
require_count "${action_fetcher_hits}" "kind-e2e-setup direct-fetcher scan"
[[ "${action_fetcher_hits}" == "0" ]] ||
  fail "${ACTION} still invokes a fetcher directly. This is the loop everyone copies -- CI-HELMDL-01 explicitly declined to imitate it because after three failed attempts it falls THROUGH to 'chmod +x' on a file that was never written, so an unreachable kind.sigs.k8s.io surfaces as 'No such file or directory'. Route it through fetch_to from hack/lib/fetch.sh like the installers do"

action_source_hits="$(action_lines | count_matches -F 'hack/lib/fetch.sh')"
require_count "${action_source_hits}" "kind-e2e-setup helper-source scan"
[[ "${action_source_hits}" != "0" ]] ||
  fail "${ACTION} does not source hack/lib/fetch.sh -- the kind download must use the same hardened fetch as the installers, or the asymmetry inside this one step (hardened helm, hand-rolled kind) simply reverses"

action_call_hits="$(action_lines | count_matches -E '(^|[^[:alnum:]_])fetch_to[[:space:]]')"
require_count "${action_call_hits}" "kind-e2e-setup fetch_to scan"
[[ "${action_call_hits}" != "0" ]] ||
  fail "${ACTION} sources hack/lib/fetch.sh but never calls fetch_to"

# The trailing `sleep 10` after the final attempt, and the `for attempt in 1 2 3` loop that
# produced it, must both be gone -- not merely bypassed. A leftover loop is what a later editor
# re-attaches a body to.
stale_loop_hits="$(action_lines | count_matches -E 'for[[:space:]]+attempt[[:space:]]+in')"
require_count "${stale_loop_hits}" "kind-e2e-setup stale-loop scan"
[[ "${stale_loop_hits}" == "0" ]] ||
  fail "${ACTION} still carries a hand-rolled 'for attempt in ...' download loop. Retry policy now lives in hack/lib/fetch.sh; a second copy here is how the two drift"

stale_sleep_hits="$(action_lines | count_matches -E '(^|[^[:alnum:]_])sleep[[:space:]]+[0-9]')"
require_count "${stale_sleep_hits}" "kind-e2e-setup stale-sleep scan"
[[ "${stale_sleep_hits}" == "0" ]] ||
  fail "${ACTION} still sleeps in the download path. The old loop slept 10s after its FINAL attempt -- pure dead time on a job that was going to fail anyway -- and fetch_to already backs off between attempts and not after the last one"
pass "the kind download goes through the shared helper, with no hand-rolled loop and no dead sleep left behind"

# ================================================================================================
# Self-wiring. "An unwired gate is not a gate" (house precedent: hack/test/dev_mise_pin_drift_test.sh).
# ================================================================================================
#
# This gate is reached in CI through its own named step in the lint job of
# .github/workflows/ci.yaml ("Verify shared fetch helper hardening (CI-FETCHLIB-01)"), so a
# failure here is attributed to fetch-lib rather than to helm. It first reached CI by delegation
# from hack/test/ci_install_helm_hardening_test.sh, because the lane that wrote this file did not
# own .github/workflows/**; that crutch was circular -- deleting the nested call silently
# unwired this gate -- and GATE-CIWIRING-02 replaced it with the workflow step and removed it.
#
# The assertion below still accepts EITHER route, deliberately: it is a check that the gate is
# reachable, not a check of which file reaches it, and pinning it to one spelling would red this
# gate for a legitimate rewiring. ZERO routes is the failure. The `-Fx` match on a trimmed,
# comment-stripped, `run:`-unwrapped view is what stops a commented-out or neutered invocation
# ("# bash ...", "bash ... || true", "bash ... &") from counting as wiring: GATE-COMMENT-01 and
# GATE-SCOPE-01, both from hack/test/dist_ci_wiring_test.sh, are the house records of a
# substring grep passing on exactly those.
# Two accepted spellings, and only two. `.github/workflows/ci.yaml` runs its gates from the
# repository root, so the relative form is the one a workflow step uses; a sibling gate must
# work from any working directory, so it uses its own resolved ROOT. Both are line-exact after
# trimming, so neither admits a trailing `|| true`, a `&`, or a redirect.
SELF_FORMS=(
  "bash hack/test/ci_fetch_lib_hardening_test.sh"
  'bash "${ROOT}/hack/test/ci_fetch_lib_hardening_test.sh"'
)
wiring_total=0
for candidate in "${ROOT}/.github/workflows/ci.yaml" "${ROOT}/hack/test/ci_install_helm_hardening_test.sh"; do
  [[ -f "${candidate}" ]] || continue
  for form in "${SELF_FORMS[@]}"; do
    n="$(grep -vE '^[[:space:]]*#' "${candidate}" |
      sed -E 's/^[[:space:]]*//; s/[[:space:]]*$//; s/^run:[[:space:]]*//; s/^"(.*)"$/\1/' |
      count_matches -Fx "${form}")"
    require_count "${n}" "self-wiring scan of ${candidate}"
    wiring_total=$((wiring_total + n))
  done
done
((wiring_total > 0)) ||
  fail "this gate is not invoked from .github/workflows/ci.yaml nor from hack/test/ci_install_helm_hardening_test.sh on a bare, uncommented 'bash <this script>' line. An unwired gate protects nothing: the dist_* and sonar_ko_* globs in the lint job do NOT match this filename, so one explicit invocation is the only thing keeping it in CI"
pass "the gate is invoked from a CI-reachable path (${wiring_total} bare invocation(s) found)"

echo "All CI-FETCHLIB-01 / CI-KINDLOOP-01 fetch hardening tests passed."
