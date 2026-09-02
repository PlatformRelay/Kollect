#!/usr/bin/env bash
# GATE-SIGPIPE-01 — `producer | grep -q PATTERN` is unsafe under `set -o pipefail`.
#
# THE MECHANISM
#   `grep -q` exits at its FIRST match and closes the read end of the pipe. If the
#   producer is still writing at that moment it takes SIGPIPE and exits 141, and with
#   `set -o pipefail` the PIPELINE reports 141 — i.e. non-zero — even though the match
#   was found. Every use of the pipeline's status then reads backwards:
#
#     ! producer | grep -q BAD      # absence assertion -> passes SILENTLY when BAD IS present
#     producer   | grep -q GOOD     # presence assertion -> fails LOUDLY when GOOD IS present
#
#   The producer only takes SIGPIPE if it is still writing when grep exits, which means
#   it only happens once the producer's output exceeds one pipe buffer (64 KiB on Linux).
#   A small fixture therefore behaves CORRECTLY and a real tree does not — which is why a
#   gate's own self-test structurally cannot catch this: self-test fixtures are small by
#   design. This file's fixtures are deliberately grown past the pipe buffer.
#
# THE FIX
#   Count with `grep -c` (which reads to EOF, so the producer never sees SIGPIPE) and
#   compare the count; or turn `pipefail` off for that one command. `grep -c` is preferred
#   because it keeps the guarantee local to the expression instead of mutating shell state.
#
# WHAT THIS GATE DOES
#   1. Proves the trap is live on this machine (grows a fixture until the naive shape
#      actually inverts). Non-vacuity guard: if it cannot be triggered, it says so rather
#      than pretending to have tested something.
#   2. Behavioural regression for hack/demo/hero/lib.sh's export predicate against a
#      LARGE producer — the one that runs against a real cloned inventory repo.
#   3. Static sweep: refuses NEW instances of the shape anywhere in the tree's shell.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LIB="${ROOT}/hack/demo/hero/lib.sh"

fail() {
  printf 'gate-sigpipe-01: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

note() {
  printf '# %s\n' "$*"
}

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

# ---------------------------------------------------------------------------
# 1. Is the trap live here? Grow a fixture until the naive shape inverts.
# ---------------------------------------------------------------------------
# Cap is a safety net, not a target: ~3k paths already exceed 64 KiB of `find` output
# under a mktemp -d prefix, so the loop normally settles on the second batch.
readonly FIXTURE_BATCH=2000
readonly FIXTURE_CAP=20000

naive_find_has_files() {
  # The pre-fix shape, kept here on purpose as the thing under test.
  find "$1" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) \
    ! -path '*/.git/*' | grep -q .
}

BIG="${TMP}/big-clone"
mkdir -p "${BIG}"
seeded=0
sigpipe_live=0
while [[ "${seeded}" -lt "${FIXTURE_CAP}" ]]; do
  for ((i = seeded; i < seeded + FIXTURE_BATCH; i++)); do
    printf 'apiVersion: v1\nkind: ConfigMap\n' >"${BIG}/inv${i}.yaml"
  done
  seeded=$((seeded + FIXTURE_BATCH))
  rc=0
  naive_find_has_files "${BIG}" || rc=$?
  if [[ "${rc}" -ne 0 ]]; then
    sigpipe_live=1
    break
  fi
done

BIG_BYTES="$(find "${BIG}" -type f -name '*.yaml' | wc -c)"
if [[ "${sigpipe_live}" -eq 1 ]]; then
  pass "trap reproduced: ${seeded} paths (${BIG_BYTES} bytes) make 'find | grep -q .' report rc=${rc} while the files EXIST"
else
  # Not a failure — a platform where the producer always wins the race cannot exhibit
  # the defect. Say so loudly so a green run is never mistaken for a tested one.
  note "SIGPIPE could not be triggered at ${seeded} paths (${BIG_BYTES} bytes) — step 2 below still runs, but it is not exercising the inversion on this platform."
fi

# ---------------------------------------------------------------------------
# 2. Behavioural regression: hack/demo/hero/lib.sh export predicate.
# ---------------------------------------------------------------------------
# _hero_export_has_inventory_files decides whether the hero demo's Git export is
# non-empty. It runs against a real cloned inventory repo, which is exactly the
# large-producer case, and a false "empty" there fails the demo for a reason that
# does not exist. Sourced in a subshell: lib.sh declares readonly globals.
[[ -f "${LIB}" ]] || fail "missing ${LIB}"

probe_lib_predicate() { # dir -> prints "present" | "absent:<rc>"
  (
    set -euo pipefail
    # shellcheck source=../demo/hero/lib.sh disable=SC1091
    source "${LIB}" >/dev/null 2>&1
    rc=0
    _hero_export_has_inventory_files "$1" || rc=$?
    if [[ "${rc}" -eq 0 ]]; then printf 'present\n'; else printf 'absent:%s\n' "${rc}"; fi
  )
}

got="$(probe_lib_predicate "${BIG}")"
[[ "${got}" == "present" ]] ||
  fail "_hero_export_has_inventory_files reported '${got}' for a ${seeded}-file export that IS non-empty (SIGPIPE inversion under pipefail)"
pass "_hero_export_has_inventory_files: large non-empty export reported present"

mkdir -p "${TMP}/empty-clone"
got="$(probe_lib_predicate "${TMP}/empty-clone")"
[[ "${got}" != "present" ]] ||
  fail "_hero_export_has_inventory_files reported 'present' for an EMPTY export — the predicate is vacuous"
pass "_hero_export_has_inventory_files: empty export reported absent"

got="$(probe_lib_predicate "${TMP}/does-not-exist")"
[[ "${got}" != "present" ]] ||
  fail "_hero_export_has_inventory_files reported 'present' for a missing directory"
pass "_hero_export_has_inventory_files: missing directory reported absent"

# ---------------------------------------------------------------------------
# 3. Static sweep — no NEW instances of the shape.
# ---------------------------------------------------------------------------
# Scope, and why it is drawn here:
#   consumer  = a stage that stops reading early: `grep` with a `q` flag, or `head`.
#   producer  = a command whose output size is NOT visible in the source line because it
#               walks a filesystem, a repo, a cluster or a network. `echo`/`printf`/`sed`/
#               `awk`/`grep` over an in-repo file are excluded: their output is bounded by
#               something a reviewer can see, and including them costs ~40 false positives
#               for no signal. They are not SAFE, they are out of this gate's scope.
# Logical lines are reassembled first (`\`, `|`, `&&`, `||` continuations), because every
# instance this gate was written for spans two physical lines.
readonly SHORT_CIRCUIT_CONSUMER='^(grep([[:space:]]+-[a-zA-Z-]+)*[[:space:]]+-[a-zA-Z]*q|head([[:space:]]|$))'
readonly UNBOUNDED_PRODUCER='^(find|git|ls|curl|wget|kubectl|kind|helm|docker|podman|gh|glab|oras|cosign|crane|rg|tar|kustomize)([[:space:]]|$)'

# ALLOW-LIST — pre-existing instances, one reason per entry. Matched as
# "<repo-relative path>::<substring of the normalized logical line>", NOT by line number,
# so a sibling lane reformatting a file does not red this gate. A stale entry (nothing
# matches it any more) is reported as a warning, not a failure, so that another lane
# FIXING its instance cannot red this gate either.
#
# Apart from this gate's own deliberate reproducer, every entry below is known debt filed
# by GATE-SIGPIPE-01's repo sweep and owned by a different area of the tree; each needs its
# own change with its own regression test. Removing an entry (rather than fixing the line
# it names) is how this debt gets silently re-accepted — don't.
#
# NOT covered here, and therefore not allow-listed: the same defect with an `echo "$VAR"`
# producer. hack/e2e/multitenant.sh's cross-tenant leakage assertions are the sharp case —
# `if echo "${body_a}" | grep -q "${TENANT_B}/tenant-app"; then fail` passes SILENTLY once
# the inventory body exceeds a pipe buffer, i.e. the isolation assertion stops asserting
# exactly when there is enough data for a leak to hide in. See the follow-up story.
ALLOWED=(
  # This gate's own reproducer. naive_find_has_files IS the defect, kept verbatim so step 1
  # can prove the trap is still live on the machine running the gate.
  "hack/test/hyg_sigpipe_pipefail_test.sh::! -path '*/.git/*' | grep -q ."
  # Presence assertion over the pipeline CLI's own output tree. Inverts LOUDLY (reports
  # "no YAML written" once the CLI writes >64 KiB of paths) — a false red on a required-ish
  # e2e job. Owner: hack/kind/**.
  "hack/kind/e2e/pipeline-cli-smoke.sh::if ! find"
  # Same file: `find | head -1` to pick a sample path. SIGPIPE makes the assignment fail
  # under `set -e` rather than return a wrong path. Owner: hack/kind/**.
  "hack/kind/e2e/pipeline-cli-smoke.sh::written="
  # `git ls-tree` over an upstream operator directory piped into `grep -q .`. Presence
  # assertion; inverts loudly once the listing exceeds a pipe buffer. Owner: hack/.
  "hack/operatorhub-pr.sh::git ls-tree"
  # `kind get clusters | grep -qx` — bounded in practice (a handful of cluster names), but
  # the same shape. Owner: hack/kind/**.
  "hack/kind/common.sh::kind get clusters"
  "hack/demo/kind-wide-scope/demo.sh::kind get clusters"
  # `kubectl logs --tail=400 | grep -Eq` — 400 log lines routinely exceed 64 KiB, so this
  # readiness probe can report "not started" for a controller that HAS started.
  # Owner: hack/kind/**.
  "hack/kind/common.sh::kubectl logs"
  # `kubectl get ... | grep -Fq` / `| grep -Eq` finalizer + event assertions. The
  # `-o json` one pipes ALL events in the namespace. Owner: hack/e2e/**.
  "hack/e2e/finalizer-cleanup-assert.sh::grep -Fq"
  "hack/e2e/finalizer-cleanup-assert.sh::--field-selector"
  "hack/e2e/finalizer-cleanup-assert.sh::-o json"
  # `curl .../inventory | grep -q '\"itemCount\"'` readiness poll — an inventory body larger
  # than a pipe buffer makes the poll never see a ready operator. Owner: hack/kind/**.
  "hack/kind/e2e/smoke.sh::curl -sf http://127.0.0.1:18082/inventory"
  # `curl | head -c 4000` diagnostics dumps, status discarded with `|| true`. Cosmetic
  # only — worst case the dump is truncated differently. Owner: hack/e2e/**.
  "hack/e2e/multitenant.sh::head -c 4000"
  # `gh api ... | head -1` behind `|| true`: SIGPIPE silently substitutes the fallback for
  # the real commit message. Cosmetic. Owner: hack/demo/kind-wide-scope/**.
  "hack/demo/kind-wide-scope/lib/reveal.sh::gh api"
  # `git status --porcelain | grep -q 'testdata/fuzz/'` in CI. A large dirty tree makes the
  # new-fuzz-corpus check miss. Owner: .github/workflows/** (a different lane this session).
  ".github/workflows/ci.yaml::git status --porcelain"
)

# Mark the `|` characters that are real pipeline separators with \x01, leaving every other
# `|` alone. A naive split on `|` mis-reads two very common shapes:
#   grep -Eq 'a|b'            -> a literal `|` inside a quoted pattern is not a pipe
#   x="$(find . | head -1)"   -> a `|` inside "$( … )" IS a pipe, even though it sits
#                                inside double quotes
# so this walks the line with a small context stack (command / double-quoted /
# single-quoted) instead. `||` in command context is an OR, not two pipes.
mark_pipes() {
  awk '
    {
      line = $0; out = ""; n = length(line)
      # stack[1] is always "cmd"; "$(" pushes "cmd", quotes push "dq"/"sq".
      top = 1; stack[1] = "cmd"
      for (i = 1; i <= n; i++) {
        c = substr(line, i, 1)
        nxt = (i < n) ? substr(line, i + 1, 1) : ""
        if (c == "\\" && stack[top] != "sq") { out = out c nxt; i++; continue }
        if (stack[top] == "sq") {
          if (c == "'"'"'") top--
          out = out c; continue
        }
        if (c == "$" && nxt == "(") { top++; stack[top] = "cmd"; out = out "$("; i++; continue }
        if (stack[top] == "dq") {
          if (c == "\"") top--
          out = out c; continue
        }
        # command context
        if (c == "'"'"'") { top++; stack[top] = "sq"; out = out c; continue }
        if (c == "\"")    { top++; stack[top] = "dq"; out = out c; continue }
        if (c == ")" && top > 1) { top--; out = out c; continue }
        if (c == "|") {
          if (nxt == "|") { out = out "||"; i++; continue }   # OR, not a pipe
          out = out "\001"; continue
        }
        out = out c
      }
      print out
    }
  '
}

# Reassemble continued lines; emit "<start-line><TAB><logical line>".
join_logical_lines() {
  awk '
    {
      line = $0
      sub(/^[ \t]+/, "", line)
      sub(/[ \t]+$/, "", line)
      if (buf == "") { start = NR; buf = line } else { buf = buf " " line }
      if (buf ~ /(\\|\||&&)$/) next
      printf "%d\t%s\n", start, buf
      buf = ""
    }
    END { if (buf != "") printf "%d\t%s\n", start, buf }
  ' "$1"
}

# Drop shell keywords, negation, command-substitution and assignment prefixes so the
# first surviving token is the actual command being run.
readonly LEADING_NOISE='^([!({]|[[:space:]]|if|elif|while|until|then|do|else|local|export|readonly|declare|time|command|eval|run:|-|\$\(|"\$\(|`|[A-Za-z_][A-Za-z0-9_]*=["'"'"']?(\$\()?)'

strip_leading_keywords() {
  local seg="$1" prev=""
  while [[ "${seg}" != "${prev}" ]]; do
    prev="${seg}"
    [[ "${seg}" =~ ${LEADING_NOISE} ]] || break
    seg="${seg#"${BASH_REMATCH[1]}"}"
    seg="${seg#"${seg%%[![:space:]]*}"}"
  done
  printf '%s\n' "${seg}"
}

scan_file() { # path -> prints "<line>\t<logical line>" for each offending pipeline
  local path="$1" lineno text norm i j seg first rec
  # join + mark once per FILE, not once per line: one awk per line turns a ~90-file sweep
  # into ~18k processes and takes minutes.
  while IFS= read -r rec; do
    lineno="${rec%%$'\t'*}"
    norm="${rec#*$'\t'}"
    [[ "${norm}" == *$'\001'* ]] || continue
    text="${norm//$'\001'/|}"
    local -a segs=()
    IFS=$'\001' read -r -a segs <<<"${norm}"
    for ((i = 1; i < ${#segs[@]}; i++)); do
      seg="${segs[i]#"${segs[i]%%[![:space:]]*}"}"
      [[ "${seg}" =~ ${SHORT_CIRCUIT_CONSUMER} ]] || continue
      for ((j = 0; j < i; j++)); do
        first="$(strip_leading_keywords "${segs[j]#"${segs[j]%%[![:space:]]*}"}")"
        if [[ "${first}" =~ ${UNBOUNDED_PRODUCER} ]]; then
          printf '%s\t%s\n' "${lineno}" "${text}"
          continue 3 # one report per logical line, then on to the next line
        fi
      done
    done
  done < <(join_logical_lines "${path}" | mark_pipes)
}

mapfile -t SCAN_FILES < <(
  find "${ROOT}" \
    \( -name .git -o -name node_modules -o -name vendor -o -name bin \) -prune -o \
    -type f \( -name '*.sh' -o -name '*.bash' \) -print
  find "${ROOT}/.github/workflows" -type f \( -name '*.yaml' -o -name '*.yml' \) -print
)
[[ "${#SCAN_FILES[@]}" -gt 0 ]] || fail "sweep found no shell files to scan — the scan is vacuous"

allow_hit=()
violations=()
for f in "${SCAN_FILES[@]}"; do
  rel="${f#"${ROOT}/"}"
  while IFS=$'\t' read -r lineno text; do
    [[ -n "${lineno:-}" ]] || continue
    matched=""
    for entry in "${ALLOWED[@]}"; do
      apath="${entry%%::*}"
      aneedle="${entry#*::}"
      [[ "${rel}" == "${apath}" ]] || continue
      [[ "${text}" == *"${aneedle}"* ]] || continue
      matched="${entry}"
      break
    done
    if [[ -n "${matched}" ]]; then
      allow_hit+=("${matched}")
    else
      violations+=("${rel}:${lineno}: ${text}")
    fi
  done < <(scan_file "${f}")
done

for entry in "${ALLOWED[@]}"; do
  hit=0
  for h in "${allow_hit[@]+"${allow_hit[@]}"}"; do
    [[ "${h}" == "${entry}" ]] && hit=1 && break
  done
  [[ "${hit}" -eq 1 ]] ||
    note "stale allow-list entry (nothing matches it any more — remove it): ${entry}"
done

if [[ "${#violations[@]}" -gt 0 ]]; then
  printf 'gate-sigpipe-01: short-circuiting consumer fed by an unbounded producer under pipefail:\n' >&2
  printf '  %s\n' "${violations[@]}" >&2
  printf '\n' >&2
  printf 'Under `set -o pipefail` the producer takes SIGPIPE when grep -q / head exits first,\n' >&2
  printf 'so the pipeline reports failure while the match EXISTS. Count with `grep -c` (reads to\n' >&2
  printf 'EOF) and compare the count, or scope `set +o pipefail` to that single command.\n' >&2
  printf 'If an instance is genuinely bounded, add it to ALLOWED in %s with a reason.\n' "${BASH_SOURCE[0]}" >&2
  exit 1
fi
pass "no new '<unbounded producer> | grep -q / head' pipelines (${#SCAN_FILES[@]} files scanned, ${#ALLOWED[@]} pre-existing instances allow-listed)"

printf 'gate-sigpipe-01: ok\n'
