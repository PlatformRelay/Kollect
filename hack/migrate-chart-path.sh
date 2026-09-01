#!/usr/bin/env bash
# DIST-AH-03: move the Kollect Helm chart from ghcr.io/platformrelay/kollect to
# ghcr.io/platformrelay/charts/kollect (ADR-0709), by hand, with every step checked.
#
# WHY THIS IS A SCRIPT AND NOT A CI JOB
# -------------------------------------
# ADR-0709 records the migration as prose. The code half of it (release workflow, docs,
# install coordinate) is ordinary reviewable change and lives in CI. The REGISTRY half
# cannot: it needs a GitHub token carrying `write:packages` -- a scope no workflow token
# and no agent session has -- plus cosign/crane/oras on PATH. It is therefore performed
# once, by a maintainer, at a keyboard. This file is that procedure, made executable and
# self-checking, so the one-shot run is not driven from a bullet list.
#
# DR-FIND-07, restated because it is the reason this migration exists and the reason it is
# dangerous: ONE OCI repository, ghcr.io/platformrelay/kollect, currently holds two kinds
# of artifact. Bare semver tags (0.18.0) are Helm charts. `v`-prefixed tags (v0.18.0) are
# the multi-arch controller image. Both resolve to a valid sha256 digest, so a reference to
# the wrong one passes every format check, every lint, and every "does it exist" probe --
# and has already shipped one defect (an OLM bundle that pinned the CHART digest as the
# controller image, surfacing only as a CreateContainerError at install time).
#
# The chart moves. The CONTROLLER IMAGE DOES NOT. Its digests are pinned immutably in OLM
# bundles already merged into community-operators and community-operators-prod, and its
# cosign signatures are bound to the current path. Everything below copies FROM the old
# path TO the new one and never the other way.
#
# SAFETY PROPERTIES (gated by hack/test/dist_chart_path_migration_test.sh):
#   * nothing here removes anything, anywhere -- not a tag, not a package version, not a
#     signature. Every GitHub API call is a plain GET. There is no path through this file
#     that reaches a destructive verb.
#   * charts 0.9.0-0.13.0 are never republished (see EXCLUDED_VERSIONS).
#   * the V1 gate runs before the bulk copy, and the bulk copy refuses to run without it.
#   * the visibility check runs before the metadata push.
#
# USAGE: run with no arguments first. See --help.
set -euo pipefail

# --------------------------------------------------------------------------------------
# Constants
# --------------------------------------------------------------------------------------
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

REGISTRY="ghcr.io"
OWNER="platformrelay"

# Source: the shared image+chart repository. Destination: the chart-only nested path.
# ADR-0709: the last path segment must stay `kollect` -- `helm push` appends the chart
# name, and Artifact Hub's documented URL format is oci://registry/namespace/chart-name.
SRC_REPO="${REGISTRY}/${OWNER}/kollect"
DST_REPO="${REGISTRY}/${OWNER}/charts/kollect"

# GHCR package name for the destination, and its URL-encoded form. GitHub's packages API
# takes the package name as a single path segment, so the nested path's slash has to be
# percent-encoded or the request 404s on a route that does not exist.
DST_PACKAGE_NAME="charts/kollect"
DST_PACKAGE_NAME_ENC="charts%2Fkollect"

# The verify shape published in docs/RELEASE.md and docs/security/security-architecture.md.
# Used VERBATIM. A looser regexp here would certify signatures that a user following the
# published command would reject, which is the opposite of what this gate is for.
OIDC_ISSUER="https://token.actions.githubusercontent.com"
IDENTITY_REGEXP='^https://github.com/platformrelay/kollect/.+'

# The V1 gate chart. One chart is copied and fully verified before any other is touched,
# so that a signature-portability failure costs one copy rather than six.
V1_GATE_VERSION="0.14.0"

# The rest of the installable history. 0.14.0 is the first chart that defaults image.tag
# to v<appVersion>; everything from there on is genuinely installable.
BULK_VERSIONS=(0.15.0 0.16.0 0.17.0 0.18.0 0.19.0)

# HARD-CODED EXCLUSION -- DO NOT WIDEN THE RANGE.
#
# ADR-0709: same exclusion, TWO DIFFERENT REASONS. Do not collapse them into one sentence.
#
#   0.9.0, 0.10.0, 0.11.0, 0.13.0 -- these four ARE charts, and they shipped
#     `image.tag: latest` in values.yaml. That tag has never been pushed to GHCR: the
#     controller image only ever lands on v-prefixed tags. So they CANNOT INSTALL -- the
#     manager pod sits in ImagePullBackOff for anyone who tries. Publishing a `latest` tag
#     to satisfy them would re-create the exact DR-FIND-07 collision the release workflow
#     guards against, so that is not a fix either.
#
#   0.12.0 -- NOT a chart at all. That one release pushed the controller IMAGE to the
#     chart's bare tag (this is DR-FIND-07's original instance, and artifacthub-repo.yml
#     records it in the same words). Copying it would put an image into a repository whose
#     entire purpose is to contain only charts. Its GHCR package version is the protected
#     1087932920 named below.
#
# The distinction is operator-facing and load-bearing: a maintainer who reads a blanket
# "hardcodes image.tag: latest", opens 0.12.0's values.yaml, finds no such line and
# concludes the note is stale is precisely the reader this exclusion exists to stop.
#
# Republishing any of the five would resurrect uninstallable listings AND restore the
# recurring Artifact Hub scan-failure mail that ADR-0709 exists to end. If you are reading
# this because the version list looks arbitrary and you were about to tidy it into a range:
# this is the tidy version. The gap is the point. The list is also enforced at RUNTIME by
# assert_not_excluded(), so hand-editing BULK_VERSIONS to resume a partial run cannot
# reach a copy either.
EXCLUDED_VERSIONS=(0.9.0 0.10.0 0.11.0 0.12.0 0.13.0)

# Per-version, because the two reasons above are not interchangeable.
exclusion_reason() {
  case "$1" in
    0.12.0)
      printf '%s' "NEVER republished: this release pushed the controller IMAGE to the chart's bare tag, so 0.12.0 is not a chart at all -- the original DR-FIND-07 instance. Its GHCR package version is the protected one."
      ;;
    *)
      printf '%s' "NEVER republished: hardcodes image.tag=latest, a tag never pushed, so the chart cannot install; taken out of the registry on purpose"
      ;;
  esac
}

# OUT OF SCOPE, NAMED SO IT STAYS OUT OF SCOPE. GHCR package version 1087932920 carries the
# v0.12.0 CONTROLLER IMAGE and its cosign signature. It sits in the same GHCR package as
# the charts being copied and, because 0.12.0 also appears in the exclusion list above, it
# is the single most likely thing for a future reader to mistake for a stale chart worth
# tidying away. It is not. Nothing in this file reads it, writes it, or names it to any
# registry or API client.
PROTECTED_PACKAGE_VERSION_ID="1087932920"

# Artifact Hub's public repository search endpoint. No API key: verified 2026-09-01,
# HTTP 200, returning repository_id, url, verified_publisher, last_tracking_ts and
# last_tracking_errors for the `kollect` repository.
AH_SEARCH_URL="https://artifacthub.io/api/v1/repositories/search?name=kollect&kind=0&limit=5"

# ADR-0709 identifies the Artifact Hub repository by repository_id, and so does this script.
# The endpoint is a SEARCH: taking element 0 would silently substitute any future sibling
# whose name merely contains "kollect", and even an exact name match is only as stable as
# the name. repository_id is the row's identity -- the same value that must survive the URL
# repoint, and the same one recorded in artifacthub-repo.yml. Exact name is kept as a
# fallback so the mode still reports something useful if the id is ever reissued.
AH_REPOSITORY_ID="cb3be9a6-8e3b-4419-9de5-1184fe349c29"

# Floor for --interval on LIVE reads. Artifact Hub tracks roughly every 30 minutes.
AH_MIN_INTERVAL=60

# --------------------------------------------------------------------------------------
# Mode / options
# --------------------------------------------------------------------------------------
# DRY RUN IS THE DEFAULT, and that is a deliberate choice rather than a convention.
#
# Every mutating step in this script writes to a PUBLIC registry. Publication is not
# symmetric with its undo: withdrawing a mistakenly published package path requires a GHCR
# package-version deletion, which this script refuses to perform under any circumstance
# (see the safety properties above). So the cost of an accidental live run is a permanent,
# publicly visible artifact that this tool cannot clean up, while the cost of an accidental
# rehearsal is a screen of text. Those are not close. A bare `bash hack/migrate-chart-path.sh`
# therefore rehearses; writing requires typing --apply, which is exactly the moment a
# maintainer should be thinking about what they are about to publish.
#
# --dry-run is still accepted explicitly, because a script whose safe mode has no name is a
# script whose safe mode cannot be asked for in a runbook.
APPLY=0
MODE="migrate"
SINCE_TS=""
POLL_INTERVAL=300
POLL_TIMEOUT=3600
SKIP_SCOPE_PROBE=0

# Set by phase1_v1_gate on success. phase2_copy_remaining refuses to run without it, so
# the ordering survives someone invoking a phase directly while resuming a partial run.
V1_GATE_PASSED=0

# Test seam for --verify-ac1: a colon-separated list of JSON files, consumed in order in
# place of live HTTP reads. It exists because the sampling rule below -- "two reads whose
# tracking timestamp genuinely advanced" -- is the entire reason that mode exists, and a
# rule that cannot be exercised offline can only be asserted by grepping for its own source
# text, which proves nothing. It is also useful by hand: paste two captured responses and
# see what verdict they would have produced.
MIGRATE_AH_FIXTURES="${MIGRATE_AH_FIXTURES:-}"
AH_FIXTURES=()
AH_FIXTURE_IDX=0

# --------------------------------------------------------------------------------------
# Output helpers
# --------------------------------------------------------------------------------------
info() { printf '%s\n' "$*"; }
warn() { printf 'WARNING: %s\n' "$*" >&2; }

die() {
  printf 'migrate-chart-path: %s\n' "$*" >&2
  exit 1
}

# Usage errors exit 64 (sysexits.h EX_USAGE), NOT 1. Exit 1 is the documented FAIL verdict
# of --verify-ac1, and a mistyped `--since` that exited 1 would be indistinguishable from
# "Artifact Hub still reports tracking errors" -- a wrong answer to the question the
# operator is actually asking, delivered with the same exit code as a right one.
usage_error() {
  printf 'migrate-chart-path: %s\n\n' "$*" >&2
  usage >&2
  exit 64
}

# Read the value of an option that takes one, refusing to run off the end of the argument
# list. `shift` on an empty stack returns non-zero, which under `set -e` aborted the script
# with exit 1 and no output at all -- again indistinguishable from a genuine FAIL.
require_value() {
  local flag="$1" value="${2-}"
  [[ -n "${value}" ]] || usage_error "${flag} requires a value"
  case "${value}" in
    --*) usage_error "${flag} requires a value, but was followed by the flag '${value}'" ;;
  esac
  printf '%s' "${value}"
}

require_uint() {
  local flag="$1" value
  value="$(require_value "${flag}" "${2-}")"
  [[ "${value}" =~ ^[0-9]+$ ]] || usage_error "${flag} expects a non-negative integer, got '${value}'"
  printf '%s' "${value}"
}

hdr() {
  printf '\n=== %s ===\n' "$*"
}

# Echo a mutating command, then run it only under --apply. Read-only probes are NOT routed
# through this: they run in both modes on purpose, so a rehearsal reports the registry's
# actual state instead of a guess.
run_mutating() {
  printf '  $ %s\n' "$*"
  if (( APPLY )); then
    "$@"
  fi
}

usage() {
  cat <<'USAGE'
migrate-chart-path.sh -- move the Kollect Helm chart to ghcr.io/platformrelay/charts/kollect

  ADR-0709. Copies the installable chart history from the shared image+chart repository to
  a chart-only path, verifying signature portability before it commits to the move, then
  hands off the one step that has no API: the Artifact Hub repository URL edit.

MODES
  (default)          Run the migration phases 0-4, then print the handoff.
  --plan             Print the version plan (what is copied, what is excluded and why).
                     Local only: no credentials, no network.
  --handoff          Print the closing instructions and warnings again. Local only.
  --verify-ac1       After the URL repoint: check Artifact Hub's public tracking state and
                     report PASS / FAIL / INCONCLUSIVE. Needs no credentials.
  --help             This text.

MIGRATION OPTIONS
  --dry-run          THE DEFAULT: dry run prints every command it would run and mutates
                     nothing. Publication to a public registry cannot be undone by this
                     script, so writing has to be asked for explicitly.
  --apply            Actually perform the copies, checks and metadata push.
  --skip-scope-probe Skip the GHCR write-scope preflight (use only if the probe itself is
                     broken; you are then relying on the phases to fail late instead).

--verify-ac1 OPTIONS
  --since <unix-ts>  Baseline: only a tracking run NEWER than this can support a PASS.
                     Without it the mode reports the timestamp to use and stops.
  --interval <sec>   Seconds between polls while waiting for the tracking run to advance
                     (default 300).
  --timeout <sec>    Give up waiting after this many seconds (default 3600).

ENVIRONMENT
  GHCR_TOKEN / GITHUB_TOKEN / CR_PAT
                     GitHub token. Needs write:packages (read:packages alone is NOT
                     enough) and read:packages for the visibility check.
  GHCR_USER          Username sent with the token (default: $GITHUB_ACTOR or $USER).

EXIT CODES
  0   success (or PASS)
  1   failure (or FAIL), including "some charts could not be copied"
  2   INCONCLUSIVE (--verify-ac1 only)
  64  usage error -- a bad or missing flag value, never a verdict about the migration
USAGE
}

# --------------------------------------------------------------------------------------
# Argument parsing
# --------------------------------------------------------------------------------------
while (( $# )); do
  case "$1" in
    --help | -h) usage; exit 0 ;;
    --plan) MODE="plan" ;;
    --handoff) MODE="handoff" ;;
    --verify-ac1) MODE="verify-ac1" ;;
    --dry-run) APPLY=0 ;;
    --apply) APPLY=1 ;;
    --skip-scope-probe) SKIP_SCOPE_PROBE=1 ;;
    --since) SINCE_TS="$(require_uint --since "${2-}")"; shift ;;
    --interval) POLL_INTERVAL="$(require_uint --interval "${2-}")"; shift ;;
    --timeout) POLL_TIMEOUT="$(require_uint --timeout "${2-}")"; shift ;;
    # An unrecognised flag is an error, never a no-op. `--dryrun` is a plausible typo for
    # `--dry-run`, and silently ignoring it would leave the operator's intent and the
    # script's behaviour disagreeing at the exact moment that matters.
    *) usage_error "unrecognised argument: $1" ;;
  esac
  shift
done

# --------------------------------------------------------------------------------------
# Plan (local, credential-free)
# --------------------------------------------------------------------------------------
print_plan() {
  info "plan: source      ${SRC_REPO}   (charts at bare tags, controller image at v-tags)"
  info "plan: destination ${DST_REPO}   (charts only)"
  info "plan: copies are by digest; the controller image is never a destination"
  info ""
  info "plan: copy ${V1_GATE_VERSION}  [V1 gate: signature portability proven here before anything else moves]  ${SRC_REPO}:${V1_GATE_VERSION} -> ${DST_REPO}:${V1_GATE_VERSION}"
  local v
  for v in "${BULK_VERSIONS[@]}"; do
    info "plan: copy ${v}  [bulk]  ${SRC_REPO}:${v} -> ${DST_REPO}:${v}"
  done
  info ""
  for v in "${EXCLUDED_VERSIONS[@]}"; do
    info "plan: excluded ${v}  $(exclusion_reason "${v}")"
  done
  info ""
  info "plan: out of scope -- GHCR package version ${PROTECTED_PACKAGE_VERSION_ID} (v0.12.0 controller image + its cosign signature) is not read or written by this script"
}

# --------------------------------------------------------------------------------------
# Phase 0 -- preconditions
# --------------------------------------------------------------------------------------
# "Check, don't assume." Each of these has a specific late failure mode it is here to
# convert into an early one: a missing tool fails halfway through the bulk copy; a
# read-only token fails on the first push, after the V1 gate has already reported success
# on a copy that never happened; a token that can write to the OLD path but not the new
# namespace fails in a way that looks like a registry outage.
require_tools() {
  local missing=()
  local t
  # cosign: `cosign copy` moves the artifact AND its signatures/attestations in one step
  #   and preserves digests. crane alone would carry the chart but not the .sig tag.
  # crane: independent digest read, used to prove the copy was a copy and not a re-push.
  # oras: the artifacthub.io metadata push (Phase 4).
  # jq:    cosign's certificate identity is only reliably available from --output json;
  #        grepping the human-readable summary for it is how you get a gate that passes on
  #        the wrong identity.
  # curl:  Artifact Hub and GitHub packages API reads.
  for t in cosign crane oras jq curl; do
    command -v "${t}" >/dev/null 2>&1 || missing+=("${t}")
  done
  if (( ${#missing[@]} )); then
    die "missing required tool(s): ${missing[*]}
  cosign: https://docs.sigstore.dev/cosign/installation/   (or: go install github.com/sigstore/cosign/v2/cmd/cosign@latest)
  crane:  https://github.com/google/go-containerregistry/releases  (part of go-containerregistry)
  oras:   https://oras.land/docs/installation
  jq, curl: your package manager"
  fi
}

registry_token() {
  printf '%s' "${GHCR_TOKEN:-${GITHUB_TOKEN:-${CR_PAT:-}}}"
}

registry_user() {
  # GHCR ignores the username when the password is a PAT, but the field must be non-empty
  # or the basic-auth header is malformed and the token endpoint answers 401 with no
  # explanation of why.
  printf '%s' "${GHCR_USER:-${GITHUB_ACTOR:-${USER:-ghcr}}}"
}

# --------------------------------------------------------------------------------------
# Credential handling
# --------------------------------------------------------------------------------------
# The token NEVER appears in any process's argv. Anything on a command line is world-
# readable from /proc/<pid>/cmdline for the lifetime of the call, and this script runs on a
# maintainer's own machine where that is a real audience. Registry logins use
# --password-stdin; curl reads its credential from a config file on stdin via `-K -`.
#
# The token is also format-checked before it is ever interpolated. Every GitHub token form
# (ghp_/gho_/ghs_/github_pat_ classic and fine-grained) draws from [A-Za-z0-9_]; the check
# below additionally tolerates '.' and '-' so that a non-GitHub registry credential is not
# rejected out of hand. What it does NOT tolerate is a quote, a backslash, whitespace or a
# newline -- none of which appear in any token, and any of which turns the value into an
# injection into curl's config parser rather than a credential.
#
# That is not hypothetical. curl config files take one directive per line, so a token
# containing a newline followed by `output = /path` makes curl write the response body to
# an attacker-named path. It is operator-self-inflicted (the value comes from their own
# environment), which is why the answer is to stop rather than to escape it into working:
# a value that is not a token should never become a working credential.
assert_token_shape() {
  local token="$1"
  [[ "${token}" =~ ^[A-Za-z0-9_.-]+$ ]] ||
    die "the registry token contains characters no GitHub token contains (expected only A-Za-z0-9_.- ).
  This is almost always a copy-paste accident: a trailing newline, a wrapping quote, or a
  whole 'export GHCR_TOKEN=...' line captured into the variable. Re-export just the token."
}

# Emit a curl config file on stdout. Values are written unquoted, which curl reads to
# end-of-line and which assert_token_shape has already proven contains no whitespace.
curl_basic_auth_config() {
  printf 'user = %s:%s\n' "$1" "$2"
}

curl_bearer_config() {
  printf 'header = "Authorization: Bearer %s"\n' "$1"
}

# Decode a base64url segment (JWT payload). Written out rather than shelled to a helper
# because the only thing on PATH that does this reliably is jq, and jq cannot base64url.
b64url_decode() {
  local d="${1//-/+}"
  d="${d//_//}"
  case $(( ${#d} % 4 )) in
    2) d="${d}==" ;;
    3) d="${d}=" ;;
    *) : ;;
  esac
  printf '%s' "${d}" | base64 -d 2>/dev/null
}

# Prove the token can WRITE to the destination namespace, without writing anything.
#
# GHCR issues a scoped JWT from its token endpoint; the granted actions are inside the
# token, so asking for `pull,push` on the destination repository and reading back what was
# actually granted is a genuine capability check with no registry side effect. This matters
# because `read:packages` and `write:packages` are indistinguishable from the outside until
# the first push fails -- and by then the V1 gate has already been "passed".
probe_write_scope() {
  local token user resp jwt payload actions scope
  token="$(registry_token)"
  user="$(registry_user)"
  scope="repository:${OWNER}/${DST_PACKAGE_NAME}:pull,push"

  # `-K -`: the credential is read from stdin as a curl config file, so it never reaches
  # this process's argv. This probe runs in dry run too, which is exactly why it matters.
  resp="$(curl_basic_auth_config "${user}" "${token}" |
    curl -fsSL -K - "https://${REGISTRY}/token?service=${REGISTRY}&scope=${scope}" 2>/dev/null)" ||
    die "could not reach ${REGISTRY}'s token endpoint with the supplied credential.
  The token is either invalid, expired, or not accepted for ${OWNER}.
  Re-issue a classic PAT with write:packages at https://github.com/settings/tokens
  and export it as GHCR_TOKEN. (Fine-grained PATs cannot currently write GHCR packages.)
  If you believe the credential is good and this probe is at fault, re-run with --skip-scope-probe."

  jwt="$(jq -r '.token // empty' <<<"${resp}")"
  [[ -n "${jwt}" ]] || die "${REGISTRY} token endpoint returned no token; re-run with --skip-scope-probe if this probe is at fault"

  payload="$(b64url_decode "$(cut -d. -f2 <<<"${jwt}")")"
  actions="$(jq -r --arg n "${OWNER}/${DST_PACKAGE_NAME}" '.access[]? | select(.name == $n) | .actions[]?' <<<"${payload}" 2>/dev/null | sort -u | tr '\n' ',')"

  case ",${actions}" in
    *,push,*) : ;;
    *)
      die "the supplied token cannot PUSH to ${DST_REPO} (granted actions: ${actions:-none}).
  read:packages alone is NOT enough -- it grants pull only, and every phase below would
  fail on its first write. Re-issue the token with the write:packages scope:
    https://github.com/settings/tokens  ->  classic token  ->  check write:packages
  then: export GHCR_TOKEN=<token>
  Note that a brand-new nested path grants push on a package that does not exist yet;
  if this still reports 'pull' only, the scope is the problem, not the path."
      ;;
  esac
  info "  token grants: ${actions%,} on ${OWNER}/${DST_PACKAGE_NAME}"
}

registry_login() {
  local token user
  token="$(registry_token)"
  user="$(registry_user)"
  # NOT routed through run_mutating, which echoes its arguments: that would print the token
  # to stdout and into whatever terminal capture or CI log the operator happens to be in.
  # --password-stdin for the same reason at the process-table level.
  #
  # These are local credential-store writes only; no registry state changes. They are still
  # performed under --apply only, so a rehearsal leaves the operator's docker config exactly
  # as it found it. That costs nothing in rehearsal: the SOURCE repository is public, so the
  # read-only digest probes report the truth without a login.
  printf '  $ printf %%s "$GHCR_TOKEN" | cosign login %s --username %s --password-stdin\n' "${REGISTRY}" "${user}"
  printf '  $ printf %%s "$GHCR_TOKEN" | crane auth login %s --username %s --password-stdin\n' "${REGISTRY}" "${user}"
  printf '  $ printf %%s "$GHCR_TOKEN" | oras login %s --username %s --password-stdin\n' "${REGISTRY}" "${user}"
  if (( APPLY )); then
    printf '%s' "${token}" | cosign login "${REGISTRY}" --username "${user}" --password-stdin
    printf '%s' "${token}" | crane auth login "${REGISTRY}" --username "${user}" --password-stdin
    printf '%s' "${token}" | oras login "${REGISTRY}" --username "${user}" --password-stdin
  fi
}

phase0_preconditions() {
  hdr "Phase 0 -- preconditions"
  require_tools
  info "  tools: cosign, crane, oras, jq, curl all on PATH"

  [[ -n "$(registry_token)" ]] ||
    die "no registry credential found.
  Export a GitHub token as GHCR_TOKEN (GITHUB_TOKEN and CR_PAT are also read).
  It must carry write:packages. read:packages alone is NOT enough: it grants pull only,
  and every copy in this migration is a push. Create one at
    https://github.com/settings/tokens  ->  'Generate new token (classic)'  ->  write:packages"
  assert_token_shape "$(registry_token)"

  if (( SKIP_SCOPE_PROBE )); then
    warn "skipping the GHCR write-scope probe on request; a read-only token will now fail late, mid-copy, instead of here"
  else
    probe_write_scope
  fi

  # The metadata push in Phase 4 needs this file, and finding out in Phase 4 wastes the run.
  [[ -f "${REPO_ROOT}/artifacthub-repo.yml" ]] ||
    die "${REPO_ROOT}/artifacthub-repo.yml not found; Phase 4 pushes it verbatim from the repo root"
  info "  artifacthub-repo.yml present"

  if (( APPLY )); then
    info "  mode: --apply (this run WILL write to ${DST_REPO})"
  else
    info "  mode: dry run (default) -- nothing will be written; re-run with --apply to perform the migration"
  fi
  registry_login
}

# --------------------------------------------------------------------------------------
# Copy machinery
# --------------------------------------------------------------------------------------
# cosign's signature for a subject with digest sha256:<hex> lives at the tag
# `sha256-<hex>.sig` in the same repository. Its presence at the destination is what
# distinguishes "the chart was copied" from "the chart and its provenance were copied".
sig_tag_for() {
  local digest="$1"
  printf 'sha256-%s.sig' "${digest#sha256:}"
}

# Read-only. Exit status carries the meaning, because "the tag is not there" and "GHCR did
# not answer" are completely different facts and must not be collapsed:
#
#   0 -- resolved; the digest is on stdout
#   1 -- definitively absent (the registry said so: MANIFEST_UNKNOWN / NAME_UNKNOWN / 404)
#   2 -- indeterminate; the raw error is on stderr
#
# The earlier version was `crane digest "$1" 2>/dev/null || true`, which turned a rate-limit
# or a network blip on any one of a dozen back-to-back GHCR calls into "no such tag" ->
# warn -> skip -> exit 0. That is a chart silently missing from the new path, discovered
# only after the Artifact Hub URL repoint, which is the one step in this migration that is
# awkward to walk back.
digest_of() {
  local ref="$1" out rc
  out="$(crane digest "${ref}" 2>&1)" && rc=0 || rc=$?
  if (( rc == 0 )); then
    printf '%s\n' "${out}"
    return 0
  fi
  case "${out}" in
    *MANIFEST_UNKNOWN* | *NAME_UNKNOWN* | *MANIFEST_BLOB_UNKNOWN* | *"manifest unknown"* | *"name unknown"* | *"404 Not Found"* | *"status code 404"*)
      return 1
      ;;
  esac
  printf 'crane digest %s failed, and not with a "no such tag" error:\n%s\n' "${ref}" "${out}" >&2
  return 2
}

# Wrapper that turns exit status 2 into a stop. Callers that legitimately tolerate an
# absent tag check for the empty string; nobody has to remember to handle the third case.
digest_or_absent() {
  local ref="$1" out rc
  out="$(digest_of "${ref}")" && rc=0 || rc=$?
  case "${rc}" in
    0) printf '%s\n' "${out}" ;;
    1) : ;;
    *)
      die "could not determine whether ${ref} exists (see the crane error above).
  This is NOT the same as 'the tag is not there', and it must not be treated as one: a
  transient registry failure silently swallowed here becomes a chart missing from the new
  path, found only after the Artifact Hub repoint. Re-run once the registry answers."
      ;;
  esac
}

# RUNTIME enforcement of the never-republish list. EXCLUDED_VERSIONS being printed is not
# enforcement: the only thing that previously stopped 0.13.0 reaching a copy was a CI count
# assertion, and a maintainer hand-editing the version list to resume a partial run does
# not go through CI. This makes the invariant hold in the process that does the writing,
# for exactly the reason phase2_copy_remaining's V1_GATE_PASSED guard exists.
assert_not_excluded() {
  local version="$1" x
  for x in "${EXCLUDED_VERSIONS[@]}"; do
    if [[ "${version}" == "${x}" ]]; then
      die "refusing to copy ${version}: it is on the never-republish list.
  ${version}: $(exclusion_reason "${version}")
  This guard fires regardless of how ${version} got into the version list -- including a
  hand edit made while resuming a partial run. If you believe the exclusion is wrong, that
  is an ADR-0709 decision, not a version-list edit."
    fi
  done
}

# Verifications 2 and 3, factored out because they must run on EVERY path that concludes a
# version is good -- including the "it was already at the destination" resume path.
#
# Both are read-only, so there is no cost to running them again on a resume, and there is a
# very large cost to not running them: `cosign copy` happens BEFORE verification, so a run
# that copied and then failed to verify leaves the destination holding exactly the chart +
# signature that the resume path would otherwise read as proof of a passed gate. The gate
# would then print "cosign verify passes ... certificate identity is the original
# release-time one" having computed neither. A gate that can be laundered by re-running is
# not a gate, and re-running is the documented remediation for Phase 3.
verify_signature_at_destination() {
  local version="$1" dst="$2" dst_digest="$3"
  local verify_out identity expected_suffix

  # Verification 2 of 3: the published verify command, verbatim, against the NEW path.
  if ! verify_out="$(cosign verify \
    --certificate-oidc-issuer "${OIDC_ISSUER}" \
    --certificate-identity-regexp "${IDENTITY_REGEXP}" \
    --output json "${dst}@${dst_digest}" 2>/dev/null)"; then
    die "${version}: cosign verify FAILED against ${dst}@${dst_digest}.
  Command used (the shape published in docs/RELEASE.md):
    cosign verify \\
      --certificate-oidc-issuer ${OIDC_ISSUER} \\
      --certificate-identity-regexp '${IDENTITY_REGEXP}' \\
      ${dst}@${dst_digest}
  See the fallback printed by the V1 gate."
  fi

  # Verification 3 of 3: WHOSE identity. A passing verify is not enough on its own -- a
  # signature minted at migration time would also verify, while presenting an identity no
  # published verify command accepts. The certificate subject must be the ORIGINAL
  # release-time workflow identity, e.g. for the V1 gate chart:
  #   https://github.com/platformrelay/kollect/.github/workflows/release.yaml@refs/tags/v0.14.0
  identity="$(jq -r '.[0].optional.Subject // empty' <<<"${verify_out}")"
  expected_suffix="/.github/workflows/release.yaml@refs/tags/v${version}"
  [[ -n "${identity}" ]] ||
    die "${version}: cosign verify returned no certificate Subject; cannot confirm the signature is the original release-time one"
  case "${identity}" in
    *"${expected_suffix}") : ;;
    *)
      die "${version}: the signature verifies, but its certificate identity is
    ${identity}
  and not the original release-time workflow identity ending in
    ${expected_suffix}
  A signature minted now -- for instance by running \`cosign sign\` on a laptop -- carries
  the maintainer's own OIDC identity. It would satisfy a naive check and would NOT satisfy
  the published verify command, so users would be told to run a command that fails."
      ;;
  esac
  info "    certificate identity: ${identity}"
  # Appended at the ONLY place the identity check is evaluated, so membership is proof that
  # it ran in this process. Phase 1 keys its gate on this array rather than on
  # VERIFIED_VERSIONS, which any future skip/resume branch could append to for a weaker
  # reason -- which is precisely how the laundering hole got in the first time.
  IDENTITY_VERIFIED_VERSIONS+=("${version}")
}

# Copy one chart and verify it landed as the SAME artifact, signed by the SAME identity.
# Returns 0 on success. Callers decide what a failure means.
copy_and_verify_chart() {
  local version="$1"
  local src="${SRC_REPO}:${version}"
  local dst="${DST_REPO}:${version}"
  local src_digest dst_digest sig_tag sig_digest

  # First thing, before anything reads or writes a registry.
  assert_not_excluded "${version}"

  info ""
  info "  ${version}: ${src} -> ${dst}"

  src_digest="$(digest_or_absent "${src}")"
  if [[ -z "${src_digest}" ]]; then
    # Not fatal, and not silent. A tag absent at the SOURCE cannot be fixed by re-running,
    # so stopping the whole migration for it would strand the remaining charts; but a
    # silently skipped version is how history quietly goes missing on the hub.
    warn "${version}: no such tag at ${src} -- skipped (nothing to copy). If this version is supposed to exist, stop and find out why before repointing Artifact Hub."
    SKIPPED_VERSIONS+=("${version}")
    return 0
  fi
  info "    source digest: ${src_digest}"
  sig_tag="$(sig_tag_for "${src_digest}")"

  dst_digest="$(digest_or_absent "${dst}")"
  if [[ -n "${dst_digest}" ]]; then
    if [[ "${dst_digest}" != "${src_digest}" ]]; then
      # Never overwrite a destination that holds something else. Whatever is there was put
      # there by something this script does not know about, and clobbering it is exactly
      # the class of action this script does not take.
      die "${dst} already exists with digest ${dst_digest}, which differs from the source digest ${src_digest}.
  Refusing to overwrite an artifact this script did not put there. Investigate by hand:
    crane manifest ${dst}
  and re-run once the destination is either absent or identical."
    fi
    # Assigned first, then tested. `[[ -n "$(...)" ]]` puts the command substitution inside
    # a condition, where `set -e` does not apply -- so digest_or_absent's die exited its
    # subshell, the empty result read as "no signature at the destination", and the script
    # carried on past a message that reads as fatal everywhere else in this file.
    sig_digest="$(digest_or_absent "${DST_REPO}:${sig_tag}")"
    if [[ -n "${sig_digest}" ]]; then
      # Idempotent resume: chart and signature both already present and identical, so the
      # copy is a no-op. The VERIFICATION is not: it is re-run in full here, because the
      # only thing this branch has established is that bytes are present, and a previous
      # run that copied and then failed verification leaves exactly these bytes. See
      # verify_signature_at_destination's comment.
      info "    already present at destination with a matching digest -- copy skipped, verifying"
      verify_signature_at_destination "${version}" "${dst}" "${dst_digest}"
      info "    ${version}: verified (no copy needed)"
      VERIFIED_VERSIONS+=("${version}")
      return 0
    fi
    info "    chart present but its signature is not; re-running the copy to carry it"
  fi

  # `cosign copy` moves the subject together with its signatures and attestations and does
  # not re-push content, so digests are preserved.
  #
  # On --force: the branch above establishes that the destination SUBJECT tag is either
  # absent or byte-identical by digest, so the flag cannot clobber a divergent chart. It is
  # weaker than that for the tags cosign also writes -- `sha256-<hex>.sig` and any `.att` --
  # which are not digest-compared above, so --force could in principle overwrite a
  # divergent signature tag. That residual is accepted rather than ignored: those tags are
  # derived from the same source subject, and whatever lands is re-verified immediately
  # below by cosign verify plus the identity check, which is a stronger statement about
  # them than a digest comparison would have been.
  run_mutating cosign copy --force "${src}" "${dst}"

  if ! (( APPLY )); then
    info "    (dry run) skipping post-copy verification: the destination has not been written"
    return 0
  fi

  # Verification 1 of 3: identical digest. A re-push -- rebuilding or re-uploading the
  # layers rather than copying them -- would produce a different digest, which would break
  # every reference that pins one and silently invalidate the existing signature.
  dst_digest="$(digest_or_absent "${dst}")"
  [[ -n "${dst_digest}" ]] || die "${version}: destination ${dst} does not resolve after the copy"
  [[ "${dst_digest}" == "${src_digest}" ]] ||
    die "${version}: DIGEST CHANGED across the copy.
  source:      ${src_digest}
  destination: ${dst_digest}
  That is a re-push, not a copy. The existing signature cannot cover the new digest.
  Stop here; see the fallback printed by the V1 gate."
  info "    digest preserved: ${dst_digest}"

  verify_signature_at_destination "${version}" "${dst}" "${dst_digest}"
  info "    ${version}: verified"
  VERIFIED_VERSIONS+=("${version}")
}

# --------------------------------------------------------------------------------------
# Phase 1 -- the V1 gate
# --------------------------------------------------------------------------------------
print_v1_fallback() {
  info ""
  info "  FALLBACK (ADR-0709), if signatures cannot be carried by cosign copy:"
  info "    1. Copy the chart and its signature tag as two plain registry copies:"
  info "         crane copy ${SRC_REPO}:${V1_GATE_VERSION} ${DST_REPO}:${V1_GATE_VERSION}"
  info "         crane copy ${SRC_REPO}:<sha256-HEX.sig> ${DST_REPO}:<sha256-HEX.sig>"
  info "       where <sha256-HEX> is the source digest with the 'sha256:' prefix replaced by 'sha256-'."
  info "    2. If the signatures genuinely cannot be carried, a RE-SIGN is required, and it"
  info "       MUST run inside GitHub Actions with 'id-token: write'. An interactive"
  info "       'cosign sign' from a laptop mints the maintainer's personal OIDC identity,"
  info "       which does NOT match ${IDENTITY_REGEXP}"
  info "       -- users following docs/RELEASE.md would get a verification failure."
  info "    3. The alternative ADR-0709 leaves open is to start the new path at 0.19.0 and"
  info "       accept the loss of the 0.14.0-0.18.0 listing. That is a decision, not a"
  info "       fallback: make it deliberately."
}

phase1_v1_gate() {
  hdr "Phase 1 -- V1 gate (${V1_GATE_VERSION} only)"
  info "  ADR-0709 leaves 'whether copied signatures still verify' explicitly untested."
  info "  This phase is that test. Nothing else is copied until it passes."

  # copy_and_verify_chart dies on any of the three checks, which is the required
  # stop-the-whole-script behaviour; the trap prints the fallback on the way out so the
  # operator does not have to go and find it in the ADR.
  trap 'print_v1_fallback' EXIT
  copy_and_verify_chart "${V1_GATE_VERSION}"
  trap - EXIT

  if (( APPLY )); then
    # copy_and_verify_chart has several paths that end in "this version is fine", and two of
    # them do so WITHOUT evaluating the signature: a tag missing at the source is a skip
    # (right for the bulk copy, very wrong here), and the resume path used to treat bytes
    # already sitting at the destination as proof. Neither means the gate ran.
    #
    # So the gate keys on IDENTITY_VERIFIED_VERSIONS, which is appended only inside
    # verify_signature_at_destination. Membership is therefore proof that the certificate
    # identity was evaluated in THIS process -- the property the printed "V1 gate PASSED"
    # line claims, and the one the bulk copy is authorised by.
    local seen=0 v
    for v in "${IDENTITY_VERIFIED_VERSIONS[@]:-}"; do
      if [[ "${v}" == "${V1_GATE_VERSION}" ]]; then
        seen=1
      fi
    done
    (( seen )) ||
      die "the V1 gate chart ${V1_GATE_VERSION} was NOT verified in this run: the certificate
  identity check never executed. Most likely ${SRC_REPO}:${V1_GATE_VERSION} does not resolve
  and the chart was skipped.
  Nothing has been proven about signature portability, so the bulk copy must not run.
  Find out why before going any further."
    V1_GATE_PASSED=1
    info ""
    info "  V1 gate PASSED: digest preserved, cosign verify passes against the new path,"
    info "  and the certificate identity is the original release-time one."
  else
    # In a rehearsal the gate CANNOT have passed: nothing was written, so nothing was
    # verified. The flag is set anyway, purely so the remaining phases can be rehearsed --
    # and the print below says so in as many words, because claiming a gate passed when it
    # was never evaluated would be the single most misleading thing this script could
    # print. The guard exists to protect an --apply run, and under --apply it is only ever
    # set after all three checks have actually succeeded.
    V1_GATE_PASSED=1
    info ""
    info "  (dry run) V1 gate NOT evaluated -- no copy was performed, so there is nothing to verify."
    info "  Under --apply this phase stops the script unless all three checks pass."
  fi
}

# --------------------------------------------------------------------------------------
# Phase 2 -- the rest of the installable history
# --------------------------------------------------------------------------------------
phase2_copy_remaining() {
  hdr "Phase 2 -- copy ${BULK_VERSIONS[0]}-${BULK_VERSIONS[-1]}"
  # Runtime ordering guard. The dispatcher already calls the phases in order, but a
  # maintainer resuming a partial migration may well call a function directly, and the
  # cost of the bulk copy running on an unproven assumption is five more artifacts to
  # reason about instead of one.
  (( V1_GATE_PASSED )) ||
    die "refusing to run the bulk copy: the V1 gate (Phase 1) has not passed in this run.
  The bulk copy is authorised by the V1 gate's result and by nothing else."

  local v
  info "  excluded and never republished:"
  for v in "${EXCLUDED_VERSIONS[@]}"; do
    info "    ${v}  $(exclusion_reason "${v}")"
  done

  for v in "${BULK_VERSIONS[@]}"; do
    copy_and_verify_chart "${v}"
  done
}

# --------------------------------------------------------------------------------------
# Phase 3 -- visibility
# --------------------------------------------------------------------------------------
# GHCR creates a NEW package PRIVATE by default. This is the likeliest silent dead end in
# the whole migration: every command above succeeds, `crane digest` resolves, `cosign
# verify` passes -- and Artifact Hub, which reads anonymously, sees nothing at all. The
# operator's evidence all says "worked"; the hub says "no packages found".
#
# CHECK AND STOP, RATHER THAN FLIP. Two reasons, in order of weight:
#
#   1. There is no API to flip it. GitHub's Packages REST API exposes get / list-versions /
#      restore and package deletion -- there is no endpoint that SETS container package
#      visibility. Changing it is a web-UI action. A script that claimed to flip it would
#      be a script that silently did nothing, which is worse than one that says so.
#   2. Even if there were, making a package public is a disclosure action. It is the one
#      step in this migration whose blast radius is "the internet can now read this", and
#      it belongs to a human who has looked at what is in the package. This script is
#      designed to be safe to re-run; an automatic publish-to-world is not.
#
# So: read the visibility, report it, and stop with the exact click-path if it is private.
phase3_visibility() {
  hdr "Phase 3 -- destination package visibility"
  local token resp visibility http

  token="$(registry_token)"

  # Owner may be an organisation or a user account; the packages API routes differ and a
  # wrong guess 404s in a way indistinguishable from "package does not exist".
  local endpoint
  for endpoint in "orgs/${OWNER}" "users/${OWNER}"; do
    # `-K -` again: the Authorization header is supplied through curl's config file on
    # stdin, so the token stays out of argv here as well.
    resp="$(curl_bearer_config "${token}" |
      curl -sS -w '\n%{http_code}' -K - \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        "https://api.github.com/${endpoint}/packages/container/${DST_PACKAGE_NAME_ENC}" 2>/dev/null || true)"
    http="$(tail -n1 <<<"${resp}")"
    [[ "${http}" == "200" ]] && break
  done

  if [[ "${http}" != "200" ]]; then
    # 404 and 401/403 mean completely different things and must not be collapsed into one
    # reassuring sentence. 404 in a rehearsal is expected (nothing has been pushed yet);
    # 401/403 is a token problem that will still be a token problem under --apply, and
    # reporting it as "does not exist yet" would hide it until the live run.
    case "${http}" in
      404)
        if ! (( APPLY )); then
          info "  (dry run) package ${DST_PACKAGE_NAME} does not exist yet (HTTP 404); it is created by the first push under --apply."
          info "  GHCR will create it PRIVATE. This phase will then stop and tell you how to publish it."
          return 0
        fi
        die "package ${DST_PACKAGE_NAME} does not exist (HTTP 404) even though Phases 1-2 reported copies.
  Go back and look at what those phases actually did before continuing."
        ;;
      401 | 403)
        die "the GitHub packages API rejected the token (HTTP ${http}) for ${DST_PACKAGE_NAME}.
  The visibility check needs read:packages. Without it this phase cannot tell you whether
  the new package is private, which is the one failure mode in this migration that is
  completely silent. Fix the token scope rather than skipping this phase."
        ;;
      *)
        die "could not read the visibility of package ${DST_PACKAGE_NAME} (HTTP ${http})."
        ;;
    esac
  fi

  visibility="$(sed '$d' <<<"${resp}" | jq -r '.visibility // "unknown"')"
  info "  ${DST_PACKAGE_NAME}: visibility = ${visibility}"

  if [[ "${visibility}" != "public" ]]; then
    die "the new package is ${visibility}, and Artifact Hub reads GHCR ANONYMOUSLY.
  Repointing the repository URL now would produce a hub entry that finds nothing, with
  every command in this migration having reported success. Publish it first:

    https://github.com/orgs/${OWNER}/packages/container/${DST_PACKAGE_NAME}/settings
      -> Package settings -> Change package visibility -> Public -> confirm

  (For a user account, replace 'orgs/${OWNER}' with 'users/${OWNER}' in that URL.)

  There is no REST endpoint that sets container package visibility, which is why this
  script stops here rather than doing it for you. Once it reads Public, re-run this
  script: Phases 1 and 2 will skip the charts that are already in place."
  fi
  info "  package is public; Artifact Hub will be able to read it anonymously"
}

# --------------------------------------------------------------------------------------
# Phase 4 -- Artifact Hub repository metadata
# --------------------------------------------------------------------------------------
# ADR-0709 flags this as the step that is easy to miss because it sits in a different job
# step from the chart push. Without artifacthub-repo.yml at <chart path>:artifacthub.io,
# Verified Publisher breaks: the hub matches the pushed repositoryID against the registered
# one, and an absent file is an unmatched ID.
#
# The file is pushed verbatim from the repo root. Nothing here parses it or depends on its
# contents -- that is another lane's file, and a second copy of its schema living in this
# script would be a second thing to keep in sync.
phase4_metadata() {
  hdr "Phase 4 -- Artifact Hub repository metadata"
  local media_config="application/vnd.cncf.artifacthub.config.v1+yaml"
  local media_layer="application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml"

  # The one content fact worth checking before the push, stated rather than assumed. On
  # `main` today artifacthub-repo.yml still carries an `ignore` regex written for the OLD
  # path. ADR-0709 says that is harmless in the new repository -- it would go on suppressing
  # 0.9.0-0.13.0, which is now suppression ON PURPOSE rather than leftover cleanup -- but
  # "harmless" is a judgement the operator should make with the fact in front of them, not
  # one this script should make silently on their behalf. Every other ordering hazard in
  # this file is enforced; this one cannot be, because the file belongs to another lane and
  # both states are legitimate. So it is surfaced, loudly, and never blocks.
  if grep -Eq '^ignore:' "${REPO_ROOT}/artifacthub-repo.yml"; then
    info "  note: artifacthub-repo.yml carries an 'ignore' list. In the new chart-only path it"
    info "        keeps 0.9.0-0.13.0 delisted, which ADR-0709 calls deliberate rather than"
    info "        leftover. Nothing to do -- but if the docs lane has since restated it, push"
    info "        the version you mean to publish."
  fi

  info "  pushing artifacthub-repo.yml to ${DST_REPO}:artifacthub.io"
  # Same shape as the release workflow's push, so the two cannot drift into producing
  # different artifacts for the same coordinate.
  (
    cd "${REPO_ROOT}"
    run_mutating oras push \
      "${DST_REPO}:artifacthub.io" \
      --config "/dev/null:${media_config}" \
      "artifacthub-repo.yml:${media_layer}"
  )
}

# --------------------------------------------------------------------------------------
# Handoff
# --------------------------------------------------------------------------------------
print_handoff() {
  hdr "Handoff -- the step with no API"
  cat <<'HANDOFF'
  Everything scriptable is done. The last step is a control-panel action on Artifact Hub
  and there is no API equivalent for it.

  DO THIS
    1. Sign in to https://artifacthub.io as the account that owns the `kollect` repository
       (user alias: konih).
    2. Control Panel -> Repositories -> kollect -> Edit.
    3. Change the URL, IN PLACE, from
         oci://ghcr.io/platformrelay/kollect
       to
         oci://ghcr.io/platformrelay/charts/kollect
       Leave the repository NAME (`kollect`) exactly as it is.
    4. Save. Then run:  bash hack/migrate-chart-path.sh --verify-ac1 --since <baseline-ts>

  WARNING 1 -- NEVER delete and re-create the Artifact Hub repository.
    Edit the URL in place. Artifact Hub's Manager.Update keys on the repository NAME, so an
    in-place URL edit keeps the same row: repository_id, accumulated stars and Verified
    Publisher status all survive. Deleting the repository and adding a new one with the
    same name and the new URL looks equivalent and is not -- it mints a new repository_id,
    drops the stars, and drops Verified Publisher, which is not trivially re-earned (it
    requires the pushed repositoryID and the owner email to match the account again, and
    the repositoryID in artifacthub-repo.yml would then be the OLD one).

  WARNING 2 -- do NOT cut a release until the URL repoint above is done.
    Once the release-workflow lane has landed, a release publishes the chart to the NEW
    path only. Until Artifact Hub is repointed it is still tracking the OLD path, so that
    release would be invisible on the hub -- and, because the old path keeps its v-prefixed
    image tags, it would add one more permanent tracking error to the list this whole
    migration exists to clear. The window between "workflow lane lands" and "URL repointed"
    is a release freeze. If a release is genuinely urgent, do the repoint first.
HANDOFF
}

# --------------------------------------------------------------------------------------
# --verify-ac1: did the repoint actually clear the tracking errors?
# --------------------------------------------------------------------------------------
# Artifact Hub exposes last_tracking_ts and last_tracking_errors on a PUBLIC, unauthenticated
# endpoint, so the acceptance criterion is checkable with no API key.
#
# THE SAMPLING RULE, which is the reason this mode exists rather than a one-line curl:
# last_tracking_errors is a SAMPLE, not a census. Observed directly on 2026-09-01: the
# registry held ELEVEN v-prefixed image tags (v0.9.0 through v0.19.0), every one of them a
# load-as-chart candidate that should error -- and the endpoint reported SIX errors. The
# field is whatever that one tracking run happened to hit. So a single empty read proves
# nothing, and reading the same tracking run twice is the easiest way to manufacture a
# green result by accident.
#
# ADR-0709 states the criterion precisely: TWO reads, BOTH taken after the repoint, BOTH
# empty. Every weaker reading of that has a way to be wrong:
#   * two reads of the same tracking run  -> one sample, not two            -> INCONCLUSIVE
#   * one post-repoint run, clean          -> whatever that run happened to hit
#                                                                           -> INCONCLUSIVE
#   * a pre-repoint run carrying errors    -> expected; not evidence against the repoint
# A pre-repoint read is therefore not a FAIL, but it is not a credit towards the two
# either: this mode keeps polling until it has seen two genuinely post-baseline runs, and
# reports INCONCLUSIVE rather than PASS until it has. On the one criterion this whole
# migration is measured by, under-claiming is the only acceptable direction to be wrong.
#
# Takes the read number (1-based) rather than keeping an internal cursor: every caller
# reaches this through a command substitution, which is a subshell, so a cursor incremented
# in here would be discarded and every read would return the same fixture.
ah_read() {
  local n="$1"
  if (( ${#AH_FIXTURES[@]} )); then
    local idx=$(( n - 1 ))
    (( idx < ${#AH_FIXTURES[@]} )) || idx=$(( ${#AH_FIXTURES[@]} - 1 ))
    cat "${AH_FIXTURES[idx]}"
    return 0
  fi
  curl -fsSL -H 'Accept: application/json' "${AH_SEARCH_URL}"
}

# Prints `<ts>\t<url>\t<verified_publisher>\t<error-count>\t<errors>` for the kollect
# repository, selected by repository_id with an exact name match as fallback.
ah_sample() {
  local raw
  raw="$(ah_read "$1")" || die "could not read ${AH_SEARCH_URL}"
  # last_tracking_errors is, today, a single newline-delimited STRING (not an array), so it
  # is flattened onto one line here: the caller reads this with `read`, and an embedded
  # newline would silently truncate every field after it.
  #
  # Both shapes are accepted. An array would be a perfectly reasonable future shape for
  # that field, and `split` on an array raises a jq type error -- which would leave this
  # script exiting 5 with a jq stack trace, a code outside its documented 0/1/2 and a
  # result the operator cannot interpret. Being liberal here costs one line.
  jq -r --arg id "${AH_REPOSITORY_ID}" '
    ((map(select(.repository_id == $id)) | .[0])
      // (map(select(.name == "kollect")) | .[0])) as $r
    | if $r == null then "MISSING" else
        (($r.last_tracking_errors // "")
          | if type == "array" then . else split("\n") end
          | map(select((. | tostring | length) > 0))) as $e
        | [ ($r.last_tracking_ts // 0 | tostring),
            ($r.url // ""),
            ($r.verified_publisher // false | tostring),
            ($e | length | tostring),
            ($e | join(" ~ "))
          ] | @tsv
      end' <<<"${raw}"
}

verify_ac1() {
  hdr "--verify-ac1 -- Artifact Hub tracking state"

  if [[ -n "${MIGRATE_AH_FIXTURES}" ]]; then
    IFS=':' read -r -a AH_FIXTURES <<<"${MIGRATE_AH_FIXTURES}"
    warn "reading fixtures instead of ${AH_SEARCH_URL} (MIGRATE_AH_FIXTURES is set)"
  fi

  # A zero interval against a live third-party API is an unbounded hot loop for as long as
  # --timeout allows. It is only ever wanted with fixtures, where reads are local file
  # copies, so clamp it everywhere else rather than letting a copy-pasted flag hammer
  # artifacthub.io. Artifact Hub tracks on the order of every 30 minutes; polling faster
  # than the floor below cannot observe anything the previous poll did not.
  if (( POLL_INTERVAL < AH_MIN_INTERVAL )) && [[ -z "${MIGRATE_AH_FIXTURES}" ]]; then
    warn "--interval ${POLL_INTERVAL} raised to ${AH_MIN_INTERVAL}s: artifacthub.io is a public third-party API and tracking runs are ~30 minutes apart, so a tighter poll only adds load"
    POLL_INTERVAL="${AH_MIN_INTERVAL}"
  fi

  # Distinct tracking runs observed, oldest first. Distinct means "different
  # last_tracking_ts": the endpoint reports whatever the last run produced, so polling
  # faster than the tracker returns the SAME run, and a repeated sample is one sample.
  local -a run_ts=() run_url=() run_vp=() run_nerr=() run_errs=()

  record_run() {
    local sample="$1" ts url vp nerr errs
    [[ "${sample}" != "MISSING" ]] ||
      die "Artifact Hub returned no repository with repository_id ${AH_REPOSITORY_ID} (nor one named 'kollect')"
    IFS=$'\t' read -r ts url vp nerr errs <<<"${sample}"
    # Same timestamp as the newest run already recorded => same tracking run => not a new
    # sample. Silently dropping it here is what makes "two runs" mean two runs.
    if (( ${#run_ts[@]} )) && [[ "${ts}" == "${run_ts[-1]}" ]]; then
      return 0
    fi
    run_ts+=("${ts}")
    run_url+=("${url}")
    run_vp+=("${vp}")
    run_nerr+=("${nerr}")
    run_errs+=("${errs}")
  }

  # How many recorded runs postdate the baseline. Only these are evidence about the
  # repoint; anything at or before the baseline was produced before it could have taken
  # effect. `>` and not `>=`: the operator pastes back the timestamp this mode printed, so
  # the baseline IS a run they already saw, and it must not count as one of the two.
  post_baseline_count() {
    local n=0 t
    [[ -n "${SINCE_TS}" ]] || { printf '0'; return 0; }
    for t in "${run_ts[@]:-}"; do
      [[ -n "${t}" ]] || continue
      if (( t > SINCE_TS )); then
        n=$(( n + 1 ))
      fi
    done
    printf '%s' "${n}"
  }

  AH_FIXTURE_IDX=1
  record_run "$(ah_sample "${AH_FIXTURE_IDX}")"

  # ADR-0709 wants TWO tracking runs taken after the repoint, so that is the target. When
  # no baseline was given there is nothing to measure "after" against; keep going until two
  # distinct runs have been seen, purely so the operator is handed a useful baseline.
  local start=${SECONDS}
  while :; do
    if [[ -n "${SINCE_TS}" ]]; then
      (( $(post_baseline_count) < 2 )) || break
    else
      (( ${#run_ts[@]} < 2 )) || break
    fi
    # With fixtures the run is bounded by the fixture list rather than by the clock.
    if (( ${#AH_FIXTURES[@]} )) && (( AH_FIXTURE_IDX >= ${#AH_FIXTURES[@]} )); then
      break
    fi
    if (( SECONDS - start >= POLL_TIMEOUT )); then
      break
    fi
    if (( POLL_INTERVAL > 0 )); then
      info "  waiting ${POLL_INTERVAL}s for the next tracking run..."
      sleep "${POLL_INTERVAL}"
    fi
    AH_FIXTURE_IDX=$(( AH_FIXTURE_IDX + 1 ))
    record_run "$(ah_sample "${AH_FIXTURE_IDX}")"
  done

  local i latest_ts="${run_ts[-1]}"
  for i in "${!run_ts[@]}"; do
    local marker="pre-baseline"
    if [[ -n "${SINCE_TS}" ]] && (( run_ts[i] > SINCE_TS )); then
      marker="POST-repoint"
    elif [[ -z "${SINCE_TS}" ]]; then
      marker="no baseline given"
    elif (( run_ts[i] == SINCE_TS )); then
      marker="IS the baseline run"
    fi
    info "  run $(( i + 1 )): last_tracking_ts=${run_ts[i]}  ($(date -u -d "@${run_ts[i]}" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo 'unparseable'))  errors=${run_nerr[i]}  [${marker}]"
    info "           url=${run_url[i]}  verified_publisher=${run_vp[i]}"
    if [[ -n "${run_errs[i]}" ]]; then
      printf '%s\n' "${run_errs[i]}" | tr '~' '\n' | sed 's/^ *//; s/^/    | /'
    fi
  done
  info ""
  info "  baseline for a later run: --since ${latest_ts}"

  if (( ${#run_ts[@]} < 2 )); then
    info ""
    info "RESULT: INCONCLUSIVE"
    info "  last_tracking_ts never advanced, so every read sampled the SAME tracking run."
    info "  last_tracking_errors is a sample, not a census, and one run is not evidence."
    info "  Wait for the next run and try again (--timeout ${POLL_TIMEOUT} was not long enough)."
    exit 2
  fi

  if [[ -z "${SINCE_TS}" ]]; then
    info ""
    info "RESULT: INCONCLUSIVE"
    info "  No --since baseline was given, so nothing ties these runs to the URL repoint: an"
    info "  empty error list from a run that predates the repoint says nothing about it."
    info "  Re-run with --since ${latest_ts} once you have repointed, or with the timestamp"
    info "  captured before the repoint if you have one."
    exit 2
  fi

  local npost
  npost="$(post_baseline_count)"
  if (( npost < 2 )); then
    info ""
    info "RESULT: INCONCLUSIVE"
    info "  Observed ${npost} tracking run(s) newer than the baseline (${SINCE_TS}); ADR-0709"
    info "  requires TWO reads BOTH taken after the repoint, both empty. One clean"
    info "  post-repoint run is not enough -- last_tracking_errors is whatever that single"
    info "  run happened to hit. Keep polling: re-run with the same --since and a longer"
    info "  --timeout, and this mode will wait for the second one."
    exit 2
  fi

  # Every post-repoint run has to hold up, not just the newest: AC1 is "both reads empty",
  # and a repoint that is only intermittently correct is not a repoint that is correct.
  local verdict="PASS" reasons=()
  for i in "${!run_ts[@]}"; do
    (( run_ts[i] > SINCE_TS )) || continue
    if [[ "${run_url[i]}" != "oci://${DST_REPO}" ]]; then
      verdict="FAIL"
      reasons+=("run at ts=${run_ts[i]} still tracks ${run_url[i]}, not oci://${DST_REPO} -- the URL repoint has not taken effect")
    fi
    if [[ "${run_vp[i]}" != "true" ]]; then
      verdict="FAIL"
      reasons+=("run at ts=${run_ts[i]} reports verified_publisher=${run_vp[i]}, not true -- the repository was probably re-created instead of edited in place, or artifacthub-repo.yml never reached ${DST_REPO}:artifacthub.io")
    fi
    if (( run_nerr[i] > 0 )); then
      verdict="FAIL"
      reasons+=("run at ts=${run_ts[i]} reported ${run_nerr[i]} tracking error(s); the migration has not cleared them")
    fi
  done

  info ""
  info "RESULT: ${verdict}"
  if [[ "${verdict}" == "FAIL" ]]; then
    local r
    for r in "${reasons[@]}"; do
      info "  - ${r}"
    done
    exit 1
  fi
  info "  ${npost} tracking runs newer than the baseline (${SINCE_TS}), all of them clean,"
  info "  all tracking the new coordinate, all still Verified Publisher."
  info "  Re-run this once or twice more over the next few hours: two clean runs meet AC1,"
  info "  several are proof."
}

# --------------------------------------------------------------------------------------
# Migration dispatcher
# --------------------------------------------------------------------------------------
SKIPPED_VERSIONS=()
VERIFIED_VERSIONS=()
IDENTITY_VERIFIED_VERSIONS=()

print_summary() {
  hdr "Summary"
  info "  verified at ${DST_REPO}: ${VERIFIED_VERSIONS[*]:-none}"
  if (( ${#SKIPPED_VERSIONS[@]} )); then
    warn "skipped (no such tag at ${SRC_REPO}): ${SKIPPED_VERSIONS[*]}"
    warn "those versions will be absent from Artifact Hub after the repoint. Confirm that is intended."
  fi
  info "  never republished (by design): ${EXCLUDED_VERSIONS[*]}"
}

run_migration() {
  phase0_preconditions
  phase1_v1_gate
  phase2_copy_remaining
  phase3_visibility
  phase4_metadata
  print_summary
  print_handoff
  if ! (( APPLY )); then
    hdr "This was a dry run"
    info "  Nothing was written. Re-run with --apply to perform the migration."
  fi
  # An incomplete migration must NOT exit 0. A skipped chart is a chart that will be absent
  # from Artifact Hub after the repoint, and the repoint is the awkward step to walk back;
  # a green exit here is how that gets noticed too late. The handoff is printed first on
  # purpose -- the operator still needs to read it -- but the status tells the truth.
  if (( ${#SKIPPED_VERSIONS[@]} )); then
    hdr "INCOMPLETE"
    printf 'migrate-chart-path: %d chart(s) were not copied: %s\n' \
      "${#SKIPPED_VERSIONS[@]}" "${SKIPPED_VERSIONS[*]}" >&2
    printf 'Do NOT repoint the Artifact Hub URL until you know why, or those versions are gone from the hub.\n' >&2
    exit 1
  fi
}

case "${MODE}" in
  plan) print_plan ;;
  handoff) print_handoff ;;
  verify-ac1) verify_ac1 ;;
  migrate) run_migration ;;
  *) die "internal error: unknown mode '${MODE}'" ;;
esac
