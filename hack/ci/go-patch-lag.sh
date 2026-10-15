#!/usr/bin/env bash
# CFC-5 (openspec change cross-file-consistency-gates): is go.mod's Go behind the latest patch of
# its minor?
#
# Reads the `go` directive of go.mod and the go.dev release listing, and compares the directive
# with the newest STABLE release of the same minor. A newer minor existing is reported as a note,
# never as a failure. The lookup fails closed: an unreachable or unparsable listing, a minor the
# listing does not carry, or a go.mod newer than the listing is a red run, never a green one.
#
# `&include=all` is required: the default listing carries only the two newest minors, so a go.mod
# two minors behind would drop out of it and "behind" would read as "unknown".
#
# Exit codes: 0 current, 1 behind, 2 cannot determine (lookup failed or go.mod unreadable).
#
# Usage:
#   hack/ci/go-patch-lag.sh [--gomod <go.mod>] [--listing <file> | --url <url>] [--retries <n>]
#   --gomod    go.mod to read (default: the repository's)
#   --listing  read the listing from a file instead of fetching it (tests)
#   --url      listing URL (default: https://go.dev/dl/?mode=json&include=all)
#   --retries  fetch retries (default 3; tests pass 0)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GOMOD="${ROOT}/go.mod"
URL="https://go.dev/dl/?mode=json&include=all"
LISTING=""
RETRIES=3

while [[ $# -gt 0 ]]; do
  case "$1" in
    --gomod)
      GOMOD="${2:?--gomod needs a path}"
      shift 2
      ;;
    --listing)
      LISTING="${2:?--listing needs a path}"
      shift 2
      ;;
    --url)
      URL="${2:?--url needs a URL}"
      shift 2
      ;;
    --retries)
      RETRIES="${2:?--retries needs a number}"
      [[ "${RETRIES}" =~ ^[0-9]+$ ]] || { echo "go-patch-lag: --retries needs a number" >&2; exit 2; }
      shift 2
      ;;
    *)
      echo "go-patch-lag: unknown argument '$1'" >&2
      exit 2
      ;;
  esac
done

minor_tag="?"
undetermined() { echo "go-patch-lag: FAIL: cannot determine the latest go${minor_tag} patch: $*" >&2; exit 2; }

command -v jq >/dev/null 2>&1 || undetermined "jq not found"

# --- go.mod's directive -------------------------------------------------------------------------
directive=""
if [[ -f "${GOMOD}" ]]; then
  directive="$(sed -nE 's/^go[[:space:]]+([^[:space:]]+)[[:space:]]*$/\1/p' "${GOMOD}")"
fi
# Canonical numbers of at most four digits: bash arithmetic reads 08 as bad octal (an arithmetic
# error inside `if` is just "false") and wraps past 2^63 -- either way a false green.
num='(0|[1-9][0-9]{0,3})'
if [[ ! "${directive}" =~ ^${num}\.${num}\.${num}$ ]]; then
  echo "go-patch-lag: FAIL: cannot read a go <major>.<minor>.<patch> directive from ${GOMOD} (found '${directive}')" >&2
  exit 2
fi
minor_tag="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

# --- the listing --------------------------------------------------------------------------------
if [[ -z "${LISTING}" ]]; then
  LISTING="$(mktemp)"
  trap 'rm -f "${LISTING:?}"' EXIT
  # Bounded well inside the workflow step's 5-minute timeout, so a stalled go.dev ends with this
  # script's own message, not a killed step: <= 200 s of retries plus one 45 s attempt.
  # hack/test/go_patch_lag_test.sh fails when --retry-max-time + --max-time outgrow that timeout.
  curl -fsSL --proto '=https' --tlsv1.2 --connect-timeout 10 --max-time 45 \
    --retry "${RETRIES}" --retry-all-errors --retry-max-time 200 -o "${LISTING}" "${URL}" ||
    undetermined "fetching ${URL} failed"
fi

# Stable patch numbers of go.mod's minor, one per line; jq errors on anything but a JSON array.
# shellcheck disable=SC2016 # $p is a jq variable
jq_program='if type != "array" then error("not an array") else . end
  | .[] | select(type == "object") | select(.stable == true)
  | .version | select(type == "string") | select(startswith($p)) | ltrimstr($p)
  | select(test("^(0|[1-9][0-9]{0,3})$"))'
patches="$(jq -r --arg p "go${minor_tag}." "${jq_program}" "${LISTING}" 2>/dev/null)" ||
  undetermined "the listing is not a JSON array of releases"
latest="$(sort -n <<<"${patches}" | tail -n 1)"
[[ -n "${latest}" ]] || undetermined "the listing has no stable go${minor_tag}.N release"

# The newest stable release overall, for the newer-minor note only.
newest="$(jq -r '[.[] | select(type == "object") | select(.stable == true) | .version
  | select(type == "string") | capture("^go(?<a>[0-9]+)\\.(?<b>[0-9]+)\\.(?<c>[0-9]+)$")
  | [(.a | tonumber), (.b | tonumber), (.c | tonumber)]] | max // empty
  | "go\(.[0]).\(.[1]).\(.[2])"' "${LISTING}" 2>/dev/null || true)"

if ((10#${patch} > 10#${latest})); then
  undetermined "go.mod's go ${directive} is newer than go${minor_tag}.${latest}, the newest listed -- the listing is stale or go.mod names an unreleased Go"
fi

behind() {
  echo "go-patch-lag: FAIL: go.mod's go ${directive} is behind go${minor_tag}.${latest}, the latest go${minor_tag} patch -- bump go.mod (and the golang images, CFC-1)" >&2
  exit 1
}
if ((10#${patch} < 10#${latest})); then
  behind
fi
echo "go-patch-lag: ok: go.mod's go ${directive} is the latest go${minor_tag} patch"
if [[ -n "${newest}" && "${newest}" != "go${directive}" ]]; then
  echo "go-patch-lag: note: a newer minor exists (${newest}); not a failure on its own"
fi
