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
#   IT IS A RACE, NOT A SIZE THRESHOLD. The producer takes SIGPIPE only if it is still
#   writing when grep exits, and whether it is depends on scheduling, not on crossing a
#   line. Bigger output makes losing the race overwhelmingly likely — but the inversion has
#   been reproduced in this repo at ~21 KB of `find` output, a third of a 64 KiB pipe
#   buffer. Do NOT read "this producer emits less than a pipe buffer" as "this is safe";
#   the only safe consumer is one that reads to EOF.
#   What the size does change is REPRODUCIBILITY: small output usually wins the race, so a
#   small fixture usually behaves correctly while a real tree does not. That is why a
#   gate's own self-test structurally cannot catch this — self-test fixtures are small by
#   design — and why this file's fixtures are deliberately grown until the race is lost.
#
# THE FIX
#   Use a consumer that reads to EOF and compare the count: `wc -l`, or `grep -c`. Or turn
#   `pipefail` off for that one command. Counting is preferred because it keeps the
#   guarantee local to the expression instead of mutating shell state, and `wc -l` is
#   preferred over `grep -c` because it exits 0 on a zero count and so needs no `|| true` —
#   and `|| true` reintroduces the same defect by turning a failed producer into "0".
#
# WHAT THIS GATE DOES
#   1. Proves the trap is live on this machine (grows a fixture until the naive shape
#      actually inverts). Non-vacuity guard: if it cannot be triggered, it says so rather
#      than pretending to have tested something.
#   2. Behavioural regression for hack/demo/hero/lib.sh's export predicate against a
#      LARGE producer — the one that runs against a real cloned inventory repo.
#   3. Parser self-test, then a static sweep that refuses NEW instances anywhere in the
#      tree's shell.
#
# LIMITS — what this gate does NOT see, stated so a green run is read correctly:
#   * Producers whose output size is visible in the source (`echo "$VAR"`, `printf`, `sed`,
#     `awk`, `grep` over an in-repo file) are out of scope; see the note above ALLOWED.
#   * Heredoc bodies are scanned as if they were code, and a logical line whose quotes do
#     not close cannot be parsed at all. The latter is REPORTED (see "unparseable" below)
#     rather than dropped; the gate never claims to have scanned a line it could not read.
#   * It is a text sweep: it cannot see a pipeline assembled at runtime (`eval`, `$CMD`).
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
# Looser form, used only to decide whether an UNPARSEABLE line is worth reporting.
readonly MENTIONS_SHORT_CIRCUIT='(grep([[:space:]]+-[a-zA-Z-]+)*[[:space:]]+-[a-zA-Z]*q|[[:space:]]head([[:space:]]|$))'
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
# producer, e.g. hack/e2e/multitenant.sh's cross-tenant leakage assertions
# (`if echo "${body_a}" | grep -q "${TENANT_B}/tenant-app"; then fail`).
#
# Those are LATENT, not live, and the reason is worth writing down because it is the only
# thing keeping them honest: internal/inventory/server.go serves the body with
# `json.NewEncoder(w).Encode(summary)` and NO SetIndent, so the response is COMPACT
# SINGLE-LINE JSON. grep cannot decide a match until it has buffered a whole line, so on a
# one-line body it drains stdin to EOF and the producer never blocks — measured rc=0 with
# PIPESTATUS=(0 0) and the leak correctly reported, at every body size from 16 B to 200 KB.
# The same bytes split across multiple lines give PIPESTATUS=(141 0), i.e. the inversion.
# So the assertion is one `SetIndent`, one move to NDJSON, or one multi-line producer away
# from going silent — a hygiene follow-up, NOT a live security hole. There is also an
# independent backstop: multitenant.sh separately asserts each tenant's item count is 1.
ALLOWED=(
  # This gate's own reproducer. naive_find_has_files IS the defect, kept verbatim so step 1
  # can prove the trap is still live on the machine running the gate.
  "hack/test/hyg_sigpipe_pipefail_test.sh::! -path '*/.git/*' | grep -q ."
  # This gate's own parser self-test fixture (step 3a): every positive case there is a
  # deliberate instance of the defect, written out so the sweep can prove it still sees it.
  # One entry covers them all — they share the marker.
  "hack/test/hyg_sigpipe_pipefail_test.sh::GATE-SIGPIPE-01-SELFTEST"
  # Presence assertion over the pipeline CLI's own output tree. Inverts LOUDLY (reports
  # "no YAML written" once the CLI writes enough paths to lose the race) — a false red on an
  # e2e job. Owner: hack/kind/**.
  "hack/kind/e2e/pipeline-cli-smoke.sh::if ! find"
  # Same file: `find | head -1` to pick a sample path. SIGPIPE makes the assignment fail
  # under `set -e` rather than return a wrong path. Owner: hack/kind/**.
  "hack/kind/e2e/pipeline-cli-smoke.sh::written="
  # `git ls-tree` over an upstream operator directory piped into `grep -q .`. Presence
  # assertion; inverts loudly on a large enough listing. Owner: hack/.
  "hack/operatorhub-pr.sh::git ls-tree"
  # `kind get clusters | grep -qx` — bounded in practice (a handful of cluster names), but
  # the same shape. Owner: hack/kind/**.
  "hack/kind/common.sh::kind get clusters"
  "hack/demo/kind-wide-scope/demo.sh::kind get clusters"
  # `kubectl logs --tail=400 | grep -Eq` — 400 controller log lines are multi-line and far
  # readiness probe can report "not started" for a controller that HAS started.
  # Owner: hack/kind/**.
  "hack/kind/common.sh::kubectl logs"
  # `kubectl get ... | grep -Fq` / `| grep -Eq` finalizer + event assertions. The
  # `-o json` one pipes ALL events in the namespace. Owner: hack/e2e/**.
  "hack/e2e/finalizer-cleanup-assert.sh::grep -Fq"
  "hack/e2e/finalizer-cleanup-assert.sh::--field-selector"
  "hack/e2e/finalizer-cleanup-assert.sh::-o json"
  # `curl .../inventory | grep -q '\"itemCount\"'` readiness poll. LATENT, not live, for the
  # same reason as the multitenant note above: the operator serves compact single-line JSON
  # (internal/inventory/server.go, Encode with no SetIndent), so grep buffers the one line
  # to EOF and curl never blocks. Goes live the day the body becomes multi-line. Same shape
  # regardless, and the poll would then never see a ready operator. Owner: hack/kind/**.
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
# `|` alone. A naive split on `|` mis-reads three very common shapes:
#   grep -Eq 'a|b'            -> a literal `|` inside a quoted pattern is not a pipe
#   x="$(find . | head -1)"   -> a `|` inside "$( … )" IS a pipe, even though it sits
#                                inside double quotes
#   # find . | grep -q .      -> a `|` inside a comment is not code at all
# so this walks the line with a small context stack (command / double-quoted /
# single-quoted) instead. `||` in command context is an OR, not two pipes.
#
# Emits "<state>\t<marked line>", where <state> is `unbalanced` when the stack did not
# return to command context by end of line — i.e. THIS LINE COULD NOT BE PARSED and any
# instance on it would be invisible. The caller reports those rather than dropping them
# silently; see the "unparseable" note below and the LIMITS section in the header.
mark_pipes() {
  awk '
    {
      line = $0; out = ""; n = length(line)
      # stack[1] is always "cmd"; "$(" pushes "cmd", quotes push "dq"/"sq".
      top = 1; stack[1] = "cmd"
      for (i = 1; i <= n; i++) {
        c = substr(line, i, 1)
        prv = (i > 1) ? substr(line, i - 1, 1) : ""
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
        # A `#` starting a word begins a comment: copy the rest verbatim and stop parsing.
        # Without this, an apostrophe in prose ("do not do this") opens a single-quote
        # context that never closes, and the rest of the line silently stops being scanned.
        if (c == "#" && (i == 1 || prv ~ /[ \t;&(]/)) { out = out substr(line, i); break }
        if (c == "'"'"'") { top++; stack[top] = "sq"; out = out c; continue }
        if (c == "\"")    { top++; stack[top] = "dq"; out = out c; continue }
        if (c == ")" && top > 1) { top--; out = out c; continue }
        if (c == "|") {
          if (nxt == "|") { out = out "||"; i++; continue }   # OR, not a pipe
          out = out "\001"; continue
        }
        out = out c
      }
      printf "%s\t%s\n", (top == 1 ? "ok" : "unbalanced"), out
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

# Drop everything in front of the command actually being run, so the first surviving token
# is that command's name.
#
# This strips WHOLE TOKENS. An earlier version matched a regex alternation of keywords with
# no boundary and chewed the head off real command names: `do` ate `docker` -> `cker`,
# `time` ate `timeout` -> `out`, `else` ate `elsewhere`, `local` ate `localhost`. `docker`
# is in UNBOUNDED_PRODUCER, so the gate declared it in scope and then structurally could
# never fire on it. Anything added here must match to a token boundary.
#
# Shell keywords and `set`-style builtins that can precede a command.
readonly NOISE_TOKENS=' if elif else fi then do done while until case esac local export readonly declare typeset time command builtin eval exec source . run: '
# Wrappers that run another command: the real producer is what FOLLOWS them.
# `timeout` additionally swallows its duration argument.
readonly WRAPPER_TOKENS=' sudo env nice ionice stdbuf nohup timeout '

strip_leading_noise() {
  local seg="$1" prev="" tok
  while [[ "${seg}" != "${prev}" ]]; do
    prev="${seg}"
    seg="${seg#"${seg%%[![:space:]]*}"}" # ltrim

    # Function definition header: `name() {`, `name () {`, `function name {`. This is the
    # repo's dominant helper idiom -- one-line `foo() { cmd | grep -q x; }` helpers are
    # everywhere under hack/ -- and omitting it let the defect through in the common case.
    if [[ "${seg}" =~ ^(function[[:space:]]+)?[A-Za-z_][A-Za-z0-9_-]*[[:space:]]*\(\)[[:space:]]*\{? ]]; then
      seg="${seg#"${BASH_REMATCH[0]}"}"
      continue
    fi
    if [[ "${seg}" =~ ^function[[:space:]]+[A-Za-z_][A-Za-z0-9_-]*[[:space:]]*\{ ]]; then
      seg="${seg#"${BASH_REMATCH[0]}"}"
      continue
    fi
    # Grouping / negation / separators / substitution openers, and a YAML list dash.
    if [[ "${seg}" =~ ^([!{}();:\&]|\$\(|\"\$\(|\`|\|\||-[[:space:]])+ ]]; then
      seg="${seg#"${BASH_REMATCH[0]}"}"
      continue
    fi
    # Assignment prefix: VAR=, VAR=", VAR="$(, VAR=$(
    if [[ "${seg}" =~ ^[A-Za-z_][A-Za-z0-9_]*=([\"\']?(\$\()?)? ]]; then
      seg="${seg#"${BASH_REMATCH[0]}"}"
      continue
    fi
    # A whole leading token that is a keyword or a transparent wrapper.
    if [[ "${seg}" =~ ^([A-Za-z_.:][A-Za-z0-9_.:-]*)([[:space:]]|$) ]]; then
      tok="${BASH_REMATCH[1]}"
      if [[ "${NOISE_TOKENS}" == *" ${tok} "* ]]; then
        seg="${seg#"${tok}"}"
        continue
      fi
      if [[ "${WRAPPER_TOKENS}" == *" ${tok} "* ]]; then
        seg="${seg#"${tok}"}"
        seg="${seg#"${seg%%[![:space:]]*}"}"
        # Drop the wrapper's own flags, and timeout's duration (`timeout 30 kubectl …`).
        while [[ "${seg}" =~ ^(-[^[:space:]]*|[0-9]+(\.[0-9]+)?[smhd]?|[A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*)([[:space:]]|$) ]]; do
          seg="${seg#"${BASH_REMATCH[1]}"}"
          seg="${seg#"${seg%%[![:space:]]*}"}"
        done
        continue
      fi
    fi
    break
  done
  printf '%s\n' "${seg}"
}

# path -> prints "<verdict>\t<line>\t<logical line>" per finding.
#   verdict "hit"         : an unbounded producer feeds a short-circuiting consumer
#   verdict "unparseable" : the quote/substitution stack did not close, so this line was
#                           NOT scanned and could be hiding an instance
scan_file() {
  local path="$1" state lineno text norm i j seg first rec
  # join + mark once per FILE, not once per line: one awk per line turns a ~190-file sweep
  # into ~24k processes and takes minutes.
  while IFS= read -r rec; do
    state="${rec%%$'\t'*}"
    rec="${rec#*$'\t'}"
    norm="${rec#*$'\t'}"
    lineno="${rec%%$'\t'*}"
    text="${norm//$'\001'/|}"
    if [[ "${state}" == "unbalanced" ]]; then
      # Only worth reporting when the line could actually hide an instance; an unclosed
      # quote in a prose message cannot.
      if [[ "${text}" =~ ${MENTIONS_SHORT_CIRCUIT} ]]; then
        printf 'unparseable\t%s\t%s\n' "${lineno}" "${text}"
      fi
      continue
    fi
    [[ "${norm}" == *$'\001'* ]] || continue
    local -a segs=()
    IFS=$'\001' read -r -a segs <<<"${norm}"
    for ((i = 1; i < ${#segs[@]}; i++)); do
      seg="${segs[i]#"${segs[i]%%[![:space:]]*}"}"
      [[ "${seg}" =~ ${SHORT_CIRCUIT_CONSUMER} ]] || continue
      for ((j = 0; j < i; j++)); do
        first="$(strip_leading_noise "${segs[j]#"${segs[j]%%[![:space:]]*}"}")"
        if [[ "${first}" =~ ${UNBOUNDED_PRODUCER} ]]; then
          printf 'hit\t%s\t%s\n' "${lineno}" "${text}"
          continue 3 # one report per logical line, then on to the next line
        fi
      done
    done
  done < <(join_logical_lines "${path}" | mark_pipes)
}

# ---------------------------------------------------------------------------
# 3a. Parser self-test — the sweep must actually SEE the repo's own idioms.
# ---------------------------------------------------------------------------
# A sweep that cannot parse the code it sweeps reports "clean" and means "blind". Two real
# escapes were found in review and are locked here:
#   * a one-line `name() { cmd | grep -q x; }` helper — the dominant idiom under hack/ —
#     was not recognised as a function header, so the producer was never inspected;
#   * keyword stripping matched without a token boundary, so `do` ate `docker` -> `cker`
#     and `time` ate `timeout` -> `out`, silencing the gate on producers it claims to cover.
# Every case below is valid bash (asserted with `bash -n`) and tagged; positives must be
# reported, negatives must not — the negatives are what stop a future fix from degenerating
# into "flag every pipeline".
PARSER_CASES="${TMP}/parser-cases.sh"
cat >"${PARSER_CASES}" <<'SELFTEST'
#!/usr/bin/env bash
set -euo pipefail
# --- positives: must be reported ---
operator_ready() { kubectl logs -n kollect-system deploy/kollect --tail=400 | grep -q 'Starting Controller'; } # GATE-SIGPIPE-01-SELFTEST case:fn-oneline-kubectl
has_yaml() { find "$1" -type f -name '*.yaml' | grep -q .; } # GATE-SIGPIPE-01-SELFTEST case:fn-oneline-find
cluster_exists() { kind get clusters | grep -qx "$1"; } # GATE-SIGPIPE-01-SELFTEST case:fn-oneline-kind
function has_json { find . -name '*.json' | head -1; } # GATE-SIGPIPE-01-SELFTEST case:fn-keyword-form
if docker logs ctr | grep -q 'ready'; then :; fi # GATE-SIGPIPE-01-SELFTEST case:docker-not-eaten-by-do
if timeout 30 kubectl logs -f deploy/x | grep -q 'ready'; then :; fi # GATE-SIGPIPE-01-SELFTEST case:timeout-not-eaten-by-time
sample="$(find . -type f | head -1)" # GATE-SIGPIPE-01-SELFTEST case:assignment-substitution
if ! find /tmp -type f \
  -name '*.log' | grep -q .; then :; fi # GATE-SIGPIPE-01-SELFTEST case:two-physical-lines
# --- negatives: must NOT be reported ---
echo "${body:-}" | grep -q leak # GATE-SIGPIPE-01-SELFTEST case:neg-echo-producer-out-of-scope
awk '/a/,/b/' /etc/hostname | grep -Eq x # GATE-SIGPIPE-01-SELFTEST case:neg-awk-bounded-producer
grep -Eq 'find|git|kubectl' /etc/hostname # GATE-SIGPIPE-01-SELFTEST case:neg-quoted-pipe-is-not-a-pipeline
find . -type f | grep -c . # GATE-SIGPIPE-01-SELFTEST case:neg-grep-c-reads-to-eof
find . -type f | wc -l # GATE-SIGPIPE-01-SELFTEST case:neg-wc-l-reads-to-eof
dockerize logs | grep -q ready # GATE-SIGPIPE-01-SELFTEST case:neg-producer-matches-whole-token-only
timeouts_report | grep -q ready # GATE-SIGPIPE-01-SELFTEST case:neg-wrapper-matches-whole-token-only
# find . -type f | grep -q . # GATE-SIGPIPE-01-SELFTEST case:neg-commented-out
SELFTEST

bash -n "${PARSER_CASES}" ||
  fail "parser self-test fixture is not valid bash — the cases must be real code, not prose"

readonly PARSER_POSITIVE=(
  fn-oneline-kubectl fn-oneline-find fn-oneline-kind fn-keyword-form
  docker-not-eaten-by-do timeout-not-eaten-by-time
  assignment-substitution two-physical-lines
)
readonly PARSER_NEGATIVE=(
  neg-echo-producer-out-of-scope neg-awk-bounded-producer
  neg-quoted-pipe-is-not-a-pipeline neg-grep-c-reads-to-eof neg-wc-l-reads-to-eof
  neg-producer-matches-whole-token-only neg-wrapper-matches-whole-token-only
  neg-commented-out
)

flagged_tags=""
while IFS=$'\t' read -r verdict _ text; do
  [[ "${verdict}" == "hit" ]] || continue
  [[ "${text}" =~ case:([A-Za-z0-9-]+) ]] &&
    flagged_tags="${flagged_tags} ${BASH_REMATCH[1]}"
done < <(scan_file "${PARSER_CASES}")

for tag in "${PARSER_POSITIVE[@]}"; do
  [[ "${flagged_tags}" == *" ${tag}"* ]] ||
    fail "parser self-test: '${tag}' is a real instance of the defect and the sweep did NOT report it — the sweep is blind to that shape"
done
for tag in "${PARSER_NEGATIVE[@]}"; do
  [[ "${flagged_tags}" != *" ${tag}"* ]] ||
    fail "parser self-test: '${tag}' is not an instance and the sweep reported it — the sweep has degenerated into flagging every pipeline"
done
pass "parser self-test: ${#PARSER_POSITIVE[@]} escape shapes reported, ${#PARSER_NEGATIVE[@]} non-instances left alone"

# ---------------------------------------------------------------------------
# 3b. The sweep itself.
# ---------------------------------------------------------------------------
mapfile -t SCAN_FILES < <(
  find "${ROOT}" \
    \( -name .git -o -name node_modules -o -name vendor -o -name bin \) -prune -o \
    -type f \( -name '*.sh' -o -name '*.bash' \) -print
  find "${ROOT}/.github/workflows" -type f \( -name '*.yaml' -o -name '*.yml' \) -print
)
[[ "${#SCAN_FILES[@]}" -gt 0 ]] || fail "sweep found no shell files to scan — the scan is vacuous"

allow_hit=()
violations=()
unparseable=()
for f in "${SCAN_FILES[@]}"; do
  rel="${f#"${ROOT}/"}"
  while IFS=$'\t' read -r verdict lineno text; do
    [[ -n "${lineno:-}" ]] || continue
    if [[ "${verdict}" == "unparseable" ]]; then
      unparseable+=("${rel}:${lineno}: ${text}")
      continue
    fi
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
# Lines the parser could not close a quote on were NOT scanned. Say so — a silent drop is
# the same failure mode this gate exists to forbid. These are notes, not failures: they are
# usually a multi-line prose string, and treating them as violations would make the gate
# unmaintainable. Anything listed here needs a human to read the line.
if [[ "${#unparseable[@]}" -gt 0 ]]; then
  note "${#unparseable[@]} logical line(s) could not be parsed (unclosed quote) AND mention a short-circuiting consumer, so they were NOT scanned — read them by hand:"
  printf '#   %.160s\n' "${unparseable[@]}"
fi

pass "no new '<unbounded producer> | grep -q / head' pipelines (${#SCAN_FILES[@]} files scanned, ${#ALLOWED[@]} pre-existing instances allow-listed, ${#unparseable[@]} unparseable)"

printf 'gate-sigpipe-01: ok\n'
