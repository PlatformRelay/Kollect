#!/usr/bin/env bash
# Shared hardened download for hack/install-*.sh and .github/actions/kind-e2e-setup
# (CI-FETCHLIB-01). Source it, then call:
#
#   fetch_to <url> <destination-path> [description]
#
# Returns 0 with <destination-path> written, or non-zero having removed any partial file and
# printed a diagnosis naming the host and curl's exit code. Never writes to stdout, so a caller
# may safely read the fetched file into a variable afterwards.
#
# shellcheck shell=bash
#
# ------------------------------------------------------------------------------------------------
# WHY THIS FILE EXISTS
#
# On 2026-09-01 the REQUIRED `kind-smoke` check went red on PR #351 -- a diff containing one bash
# test file -- after 46 seconds, with
#
#     curl: (7) Failed to connect to get.helm.sh port 443 after 1029 ms
#
# The cause was a bare `curl -fsSL` in hack/install-helm.sh: no retry, no connect bound, no
# protocol pin. CI-HELMDL-01 (PR #354) fixed that ONE script and deliberately kept its retry
# helper local, because factoring it out would have touched files that lane did not own. Its
# closing comment named the debt precisely: seven sibling installers shared the same defect, and
# a shared helper was the right home for the flag list. This is that helper. The flags are now
# written ONCE, so no two download sites in this repo can drift apart the way get.helm.sh and
# kind.sigs.k8s.io did inside a single composite step.
#
# ------------------------------------------------------------------------------------------------
# THE FLAGS, AND WHY EACH ONE IS HERE
#
# --proto '=https' --proto-redir '=https'
#     Pin the scheme on both the initial request AND the redirect chain. Several origins used
#     here are redirectors. Without the second flag a 301 to http:// downgrades the transfer to
#     plaintext, and a CHECKSUM fetched over plaintext verifies nothing at all: whoever can
#     rewrite the tarball can rewrite the digest to match it. The two flags are independent --
#     dropping either leaves the other's failure mode wide open.
#
# --tlsv1.2
#     A TLS floor. Modest on its own, but it is part of the flag set the repo already committed
#     to, and locking it here is what keeps "consistent hardening" a checked claim.
#
# -f  (--fail)
#     Without it, curl writes an HTTP error body to the output file and exits 0. The caller then
#     checksums an HTML error page and reports "checksum mismatch", sending the reader hunting a
#     corrupt release when the real fault is a wrong URL.
#
# --retry 5 --retry-delay 5 --retry-max-time 120
#     curl's own retry layer, which covers a flaky origin that ANSWERS: 408, 429, 5xx, timeouts.
#     --retry-max-time is not optional garnish -- curl RESETS --max-time on each of its own
#     retries, so without it `--retry 5 --max-time 120` is a twelve-minute worst case for a
#     single invocation.
#
# --connect-timeout 30 --max-time 120
#     --connect-timeout alone bounds only the CONNECT; a body that stalls after its first byte
#     would hang until the job's own timeout kills it with no diagnostic. 120s on an 18MB tarball
#     is a 150 KB/s floor -- unreachable on a working link, so the bound cannot itself become the
#     flake class it exists to remove.
#
# ------------------------------------------------------------------------------------------------
# WHY AN OUTER BASH LOOP *AND* curl's --retry -- neither is redundant
#
# curl's --retry covers "transient" conditions, and a failed connect is NOT one of them unless
# --retry-connrefused or --retry-all-errors is also passed. `curl: (7)` is exactly a failed
# connect, so --retry alone would not have saved PR #351. The outer loop covers the
# DNS/connect/TLS/dropped-connection class; --retry covers the answering-but-unhealthy origin
# class. Two layers, disjoint coverage, and hack/test/ci_fetch_lib_hardening_test.sh proves the
# split with two separate cases against a real local origin -- one 503-then-200 (recovered with
# zero shell-level sleeps, i.e. inside curl) and one dropped connection (recovered by this loop).
#
# --retry-all-errors is deliberately NOT used instead of the loop. It would also retry HTTP error
# responses that carry real information -- a 404 on a pinned release URL means the version is
# wrong, and retrying it five times converts a two-second diagnosis into a slow one.
#
# ------------------------------------------------------------------------------------------------
# WHAT THIS HELPER DOES *NOT* WRAP, and why
#
# The rule is: retry the TRANSPORT, never a decision about content. A retry is defensible only
# when the failure carries no information -- a dead TCP connect says nothing about the origin's
# contents. Once bytes have arrived in full, anything wrong with them (an empty body, an
# interception page, a digest that does not match) is EVIDENCE, and retrying evidence trades a
# hard, diagnosable failure for an intermittent one. A security check that sometimes passes is
# strictly worse than one that always fails, because it gets re-run until it goes green. So
# fetch_to returns as soon as the transport succeeds, and every caller verifies exactly once,
# outside this file.
#
# ------------------------------------------------------------------------------------------------
# FAILING ACCURATELY (CI-KINDLOOP-01)
#
# The three-attempt loop in .github/actions/kind-e2e-setup used to fall THROUGH when all attempts
# failed: `set -euo pipefail` still failed the job at the next command, so it was never a false
# green, but the operator was handed `chmod: No such file or directory` instead of "could not
# reach kind.sigs.k8s.io". fetch_to returns non-zero from the fetch itself, names the host, and
# includes curl's last exit code so that unreachable (7) is distinguishable from TLS failure
# (35/60) or a truncated transfer (18) without re-running the job. It also does not sleep after
# the final attempt -- the old loop burned a pointless 10 seconds before failing anyway.

KOLLECT_FETCH_ATTEMPTS=3
KOLLECT_FETCH_RETRY_DELAY=10

fetch_to() {
  local url="${1:?fetch_to: url required}"
  local dest="${2:?fetch_to: destination path required}"
  local what="${3:-${url##*/}}"
  local attempt status=0 host

  host="${url#*://}"
  host="${host%%/*}"

  for ((attempt = 1; attempt <= KOLLECT_FETCH_ATTEMPTS; attempt++)); do
    status=0
    # `|| status=$?` rather than `if curl ...`: the exit code is part of the diagnosis, and
    # under `set -e` in the sourcing script a bare failing curl would abort before this
    # function could report anything useful.
    curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
      -fsSL --retry 5 --retry-delay 5 --retry-max-time 120 \
      --connect-timeout 30 --max-time 120 \
      -o "${dest}" "${url}" || status=$?
    if ((status == 0)); then
      return 0
    fi
    # curl creates the output file before it knows the request failed, and a truncated transfer
    # leaves a partial one. Either would reach the caller's sha256sum/tar as though it were a
    # complete download, so the failure path must leave NOTHING at the destination.
    rm -f "${dest}"
    if ((attempt < KOLLECT_FETCH_ATTEMPTS)); then
      echo "${what}: download attempt ${attempt}/${KOLLECT_FETCH_ATTEMPTS} from ${host} failed (curl exit ${status}); retrying in ${KOLLECT_FETCH_RETRY_DELAY}s..." >&2
      sleep "${KOLLECT_FETCH_RETRY_DELAY}"
    fi
  done

  echo "failed to download ${what} from ${url}" >&2
  echo "  ${KOLLECT_FETCH_ATTEMPTS} attempts to ${host} all failed (last curl exit ${status})" >&2
  return 1
}
