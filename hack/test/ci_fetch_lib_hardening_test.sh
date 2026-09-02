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
  logical_lines "$1" | grep -E '(^|[;&|]|\$\(|(^|[[:space:]])(if|then|else|do|!))[[:space:]]*curl[[:space:]]' || true
}

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
# Mutation caught: dropping --proto-redir '=https'. The trap is the plaintext listener: WITHOUT
# the flag curl follows the 302 and fetches the body over http, so the assertion that the plain
# server saw zero requests is the one that discriminates. Asserting only "exit non-zero" would
# not -- a downgraded fetch SUCCEEDS.
#
# This matters most for the checksum fetch: a digest retrieved over plaintext verifies nothing,
# because anyone who can rewrite the tarball can rewrite the digest to match it.
new_case redirect-downgrade
run_fetch "${BASE}/redir-http" "${CASE_DIR}/artifact" "redirected artifact"
[[ "$(case_status)" != "0" ]] ||
  fail "https->http redirect: fetch_to exited 0, so it FOLLOWED a redirect down to plaintext. A checksum fetched over http verifies nothing at all"
[[ "$(plain_hits)" == "0" ]] ||
  fail "https->http redirect: the plaintext listener received $(plain_hits) request(s) -- --proto-redir '=https' is missing or ineffective, and the body was fetched over http"
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

echo "All CI-FETCHLIB-01 fetch-helper behavioural tests passed."
