#!/usr/bin/env bash
# CFC-4 (openspec change cross-file-consistency-gates): metric names agree between code, docs and
# the chart's alert rules.
#
# The registered metrics live in internal/metrics; docs/operator-manual/metrics.md and
# charts/kollect/templates/prometheusrule.yaml name them by hand, and metrics_catalog_test.go
# mirrors them by hand in registeredMetricNames. A rename in code leaves an alert querying a
# series that no longer exists (it never fires), and a new metric ships undocumented.
#
# What it locks:
#   * registered set = string literals that are exactly `kollect_[a-z0-9_]+` in the non-test .go
#     files of internal/metrics, EXCEPT metrics_catalog.go (its Catalog is documentation, not
#     registration). Comments are not literals. An empty set fails;
#   * the scan's blind spots fail instead of passing: a `Namespace:`/`Subsystem:` field in
#     internal/metrics (a name split into parts no literal shows), and a non-test .go file
#     outside internal/metrics that imports the Prometheus client (registration elsewhere);
#   * every kollect_* name in metrics.md and in prometheusrule.yaml is registered, or is
#     `<h>_bucket|_sum|_count` where <h> is registered with HistogramOpts. Any other suffix
#     (`_created`, `_bucket` on a counter, ...) fails. Either file yielding no name fails;
#   * every registered metric appears by its exact name in metrics.md;
#   * registeredMetricNames (internal/metrics/metrics_catalog_test.go) equals the registered set.
#
# Usage:
#   hack/test/consistency_metrics_test.sh              # check the repository tree
#   hack/test/consistency_metrics_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/consistency_metrics_test.sh <tree-root>  # check an arbitrary tree root
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

command -v python3 >/dev/null 2>&1 || fail "python3 not found; install it to run this test"

check_metrics() {
  local root="$1"
  python3 - "${root}" <<'PY'
import glob
import os
import re
import sys

root = sys.argv[1]
PKG = "internal/metrics"
CATALOG = f"{PKG}/metrics_catalog.go"
MIRROR = f"{PKG}/metrics_catalog_test.go"
DOCS = "docs/operator-manual/metrics.md"
RULES = "charts/kollect/templates/prometheusrule.yaml"
OTHER_GO_DIRS = ("api", "cmd", "internal", "pkg", "test")
CLIENT_IMPORT = re.compile(r'"github\.com/prometheus/client_golang/prometheus(?:/promauto)?"')
NAME = re.compile(r"kollect_[a-z0-9_]+")
SUFFIXES = ("_bucket", "_sum", "_count")
errors = []


def read(rel):
    try:
        with open(f"{root}/{rel}") as fh:
            return fh.read()
    except FileNotFoundError:
        print(f"FAIL: {rel} is missing", file=sys.stderr)
        sys.exit(1)


def mask_go(src):
    """Return (masked, literals): comments dropped, each string literal replaced by S<i>."""
    out, lits, i, n = [], [], 0, len(src)
    while i < n:
        c = src[i]
        if src.startswith("//", i):
            j = src.find("\n", i)
            i = n if j < 0 else j
        elif src.startswith("/*", i):
            j = src.find("*/", i + 2)
            i = n if j < 0 else j + 2
            out.append(" ")
        elif c in "\"`'":
            j = i + 1
            while j < n and src[j] != c:
                j += 2 if (c != "`" and src[j] == "\\") else 1
            if c != "'":
                out.append(f"S{len(lits)} ")
                lits.append(src[i + 1:j])
            else:
                out.append("R ")
            i = j + 1
        else:
            out.append(c)
            i += 1
    return "".join(out), lits


def block_after(masked, start):
    """Text of the {...} block whose opening brace is at or after start."""
    i = masked.index("{", start)
    depth = 0
    for j in range(i, len(masked)):
        depth += {"{": 1, "}": -1}.get(masked[j], 0)
        if depth == 0:
            return masked[i:j + 1]
    return masked[i:]


# --- registered set: non-test internal/metrics, metrics_catalog.go excluded ---
sources = sorted(p for p in glob.glob(f"{root}/{PKG}/*.go")
                 if not p.endswith("_test.go") and not p.endswith("/metrics_catalog.go"))
registered, histograms, consts, parsed = set(), set(), {}, []
for path in sources:
    with open(path) as fh:
        masked, lits = mask_go(fh.read())
    rel = os.path.relpath(path, root)
    parsed.append((rel, masked, lits))
    registered |= {s for s in lits if NAME.fullmatch(s)}
    for m in re.finditer(r"\b([A-Za-z_]\w*)\s*=\s*S(\d+)\b", masked):
        consts[m.group(1)] = lits[int(m.group(2))]
if not registered:
    print(f"FAIL: {PKG} yields no registered kollect_* name (scanned {len(sources)} non-test files, "
          f"{os.path.basename(CATALOG)} excluded) -- an empty scan never passes", file=sys.stderr)
    sys.exit(1)

for rel, masked, lits in parsed:
    m = re.search(r"\b(Namespace|Subsystem)\s*:", masked)
    if m:
        errors.append(f"{rel}: sets {m.group(1)} -- the metric name is split into parts the "
                      "literal scan cannot see; give Name the full kollect_* literal")
    for m in re.finditer(r"\bHistogramOpts\s*\{", masked):
        nm = re.search(r"\bName\s*:\s*(S(\d+)|[A-Za-z_]\w*)", block_after(masked, m.start()))
        val = None
        if nm:
            val = lits[int(nm.group(2))] if nm.group(2) else consts.get(nm.group(1))
        if not val or not NAME.fullmatch(val):
            errors.append(f"{rel}: a HistogramOpts Name does not resolve to a kollect_* literal "
                          f"({nm.group(1) if nm else 'no Name field'})")
        else:
            histograms.add(val)

# --- registration outside internal/metrics ---
for d in OTHER_GO_DIRS:
    for dirpath, dirnames, files in os.walk(f"{root}/{d}"):
        dirnames[:] = [x for x in dirnames if x not in ("testdata", "vendor")]
        rel_dir = os.path.relpath(dirpath, root)
        if rel_dir == PKG or rel_dir.startswith(PKG + "/"):
            continue
        for f in sorted(files):
            if f.endswith(".go") and not f.endswith("_test.go"):
                with open(os.path.join(dirpath, f)) as fh:
                    if CLIENT_IMPORT.search(fh.read()):
                        errors.append(f"{rel_dir}/{f}: imports the Prometheus client outside {PKG} -- "
                                      f"metrics registered there are invisible to this check; register them in {PKG}")

# --- catalog-only names, for the message ---
catalog_names = set(NAME.findall(read(CATALOG)))


def check_names(rel):
    names = set(NAME.findall(read(rel)))
    if not names:
        errors.append(f"{rel} yields no kollect_* name -- an empty scan never passes")
    for n in sorted(names):
        if n in registered:
            continue
        base = next((n[:-len(s)] for s in SUFFIXES if n.endswith(s)), None)
        if base in histograms:
            continue
        why = ""
        if base in registered:
            why = f" ({base} is registered but not with HistogramOpts, so {n} is not one of its series)"
        elif n in catalog_names:
            why = f" (only {CATALOG} names it; the catalog is documentation, not registration)"
        errors.append(f"{rel}: {n} is not a registered metric{why}")
    return names


docs_names = check_names(DOCS)
check_names(RULES)
for n in sorted(registered - docs_names):
    errors.append(f"registered metric {n} is missing from {DOCS}")

# --- the hand-kept mirror ---
mmask, mlits = mask_go(read(MIRROR))
mm = re.search(r"\bregisteredMetricNames\s*=\s*\[\]string\s*\{", mmask)
if not mm:
    errors.append(f"{MIRROR}: registeredMetricNames = []string{{...}} not found")
else:
    mirror = {mlits[int(i)] for i in re.findall(r"S(\d+)", block_after(mmask, mm.start()))}
    for n in sorted(registered - mirror):
        errors.append(f"{MIRROR}: registeredMetricNames lacks {n}, which {PKG} registers")
    for n in sorted(mirror - registered):
        errors.append(f"{MIRROR}: registeredMetricNames lists {n}, which {PKG} does not register")

for e in errors:
    print(f"FAIL: {e}", file=sys.stderr)
if not errors:
    print(f"ok - {len(registered)} registered ({len(histograms)} histograms), "
          f"{len(docs_names)} names in {DOCS}, mirror equal")
sys.exit(1 if errors else 0)
PY
}

if [[ "${MODE}" == "check" ]]; then
  if check_metrics "${ROOT}"; then
    pass "metric names agree: internal/metrics, docs/operator-manual/metrics.md, prometheusrule.yaml, registeredMetricNames"
    exit 0
  fi
  fail "metric names disagree between code, docs, alert rules and the mirror list (listed above)"
fi

# ---------------------------------------------------------------------------
# Self-test (CFC-6): copy the files the gate reads, break one invariant per copy, and assert
# the gate reds WITH THE MESSAGE of the assertion the mutant targets -- a non-zero exit alone
# is not evidence. One unmutated copy must pass.
# ---------------------------------------------------------------------------
SCRATCH=""
trap 'rm -rf ${SCRATCH} 2>/dev/null' EXIT

make_copy() {
  local dst="$1"
  mkdir -p "${dst}/internal/metrics" "${dst}/docs/operator-manual" "${dst}/charts/kollect/templates"
  cp "${ROOT}"/internal/metrics/*.go "${dst}/internal/metrics/"
  cp "${ROOT}/docs/operator-manual/metrics.md" "${dst}/docs/operator-manual/"
  cp "${ROOT}/charts/kollect/templates/prometheusrule.yaml" "${dst}/charts/kollect/templates/"
}

# Called directly (never as $(new_copy)), so SCRATCH survives in this shell for the EXIT trap.
COPY=""
new_copy() {
  COPY="$(mktemp -d)"
  SCRATCH="${SCRATCH} ${COPY}"
  make_copy "${COPY}"
}

# mutant_rejected <label> <sentinel> <probe-regex> <probe-file> <command...>
# The command runs inside the copy; the probe proves the mutation landed before the gate runs.
# The probe ABSENT asserts that <probe-file> no longer exists.
mutant_rejected() {
  local label="$1" sentinel="$2" probe="$3" probe_file="$4"
  shift 4
  local out rc
  new_copy
  (cd "${COPY}" && "$@") ||
    fail "self-test: the mutation step for '${label}' failed -- nothing was tested"
  if [[ "${probe}" == "ABSENT" ]]; then
    [[ ! -e "${COPY}/${probe_file}" ]] ||
      fail "self-test: the '${label}' mutant did not land (${probe_file} still exists) -- nothing was tested"
  else
    grep -Eq -- "${probe}" "${COPY}/${probe_file}" ||
      fail "self-test: the '${label}' mutant did not land (no '${probe}' in ${probe_file}) -- nothing was tested"
  fi
  out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Fq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected '${sentinel}', got: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

control_passes() {
  local label="$1" probe="$2" probe_file="$3"
  shift 3
  local out
  new_copy
  (cd "${COPY}" && "$@") || fail "self-test: the control step for '${label}' failed -- nothing was tested"
  grep -Eq -- "${probe}" "${COPY}/${probe_file}" ||
    fail "self-test: the '${label}' control did not land (no '${probe}' in ${probe_file})"
  out="$(bash "${SELF}" "${COPY}" 2>&1)" ||
    fail "self-test: the gate red on the '${label}' control -- it must pass: $(grep FAIL <<<"${out}" | head -2 | tr '\n' ' ')"
  pass "self-test: ${label} control passes"
}

# No-op control: an unmutated copy passes. Also proves the gate reads the root it is given.
new_copy
if ! out="$(bash "${SELF}" "${COPY}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree (or the gate) first: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
fi
pass "self-test: no-op control passes on an unmutated copy"

M="internal/metrics/metrics.go"
CAT="internal/metrics/metrics_catalog.go"
MIR="internal/metrics/metrics_catalog_test.go"
DOC="docs/operator-manual/metrics.md"
RUL="charts/kollect/templates/prometheusrule.yaml"

# Renamed in code, the catalog, the mirror and the docs; only the alert still queries the old name.
mutant_rejected "renamed metric still queried by a rule" \
  "${RUL}: kollect_workqueue_depth is not a registered metric" \
  '"kollect_workqueue_inflight"' "${M}" \
  sed -i -E 's/kollect_workqueue_depth/kollect_workqueue_inflight/g' "${M}" "${CAT}" "${MIR}" "${DOC}"

mutant_rejected "registered metric missing from the docs" \
  "registered metric kollect_informer_objects is missing from ${DOC}" \
  'kollect_informer_objects' "${M}" \
  sed -i -E '/kollect_informer_objects/d' "${DOC}"

# A name the catalog and the docs carry but no code registers (mirror untouched).
mutant_rejected "catalog-only name" \
  "${DOC}: kollect_ghost_total is not a registered metric (only ${CAT} names it" \
  'kollect_ghost_total' "${CAT}" \
  bash -c 'sed -i -E "0,/^var Catalog = \[\]CatalogEntry\{$/s//var Catalog = []CatalogEntry{\n\t{Name: \"kollect_ghost_total\", Type: MetricTypeCounter},/" '"${CAT}"' &&
    printf "\n| \`kollect_ghost_total\` | counter | — | ghost |\n" >> '"${DOC}"

# A metric added to metrics.go (and the docs) but not to registeredMetricNames.
mutant_rejected "mirror list not updated (missing name)" \
  "${MIR}: registeredMetricNames lacks kollect_brand_new_total, which internal/metrics registers" \
  'kollect_brand_new_total' "${M}" \
  bash -c 'printf "\nvar brandNew = prometheus.NewCounter(prometheus.CounterOpts{Name: \"kollect_brand_new_total\", Help: \"x\"})\n" >> '"${M}"' &&
    printf "\n| \`kollect_brand_new_total\` | counter | — | new |\n" >> '"${DOC}"

mutant_rejected "mirror list not updated (stale name)" \
  "${MIR}: registeredMetricNames lists kollect_phantom_total, which internal/metrics does not register" \
  '"kollect_phantom_total"' "${MIR}" \
  sed -i -E 's/^var registeredMetricNames = \[\]string\{$/&\n\t"kollect_phantom_total",/' "${MIR}"

mutant_rejected "mirror list removed" \
  "${MIR}: registeredMetricNames = []string{...} not found" \
  'registeredMirror' "${MIR}" \
  sed -i -E 's/registeredMetricNames/registeredMirror/g' "${MIR}"

# _bucket on a registered counter/gauge is not a series of it.
mutant_rejected "_bucket on a non-histogram" \
  "${RUL}: kollect_workqueue_depth_bucket is not a registered metric (kollect_workqueue_depth is registered but not with HistogramOpts" \
  'kollect_workqueue_depth_bucket' "${RUL}" \
  sed -i -E 's/max\(kollect_workqueue_depth\)/max(kollect_workqueue_depth_bucket)/' "${RUL}"

mutant_rejected "suffix outside _bucket/_sum/_count" \
  "${DOC}: kollect_export_duration_seconds_created is not a registered metric" \
  'kollect_export_duration_seconds_created' "${DOC}" \
  sed -i -E 's/^kollect_collect_items_total$/kollect_export_duration_seconds_created/' "${DOC}"

# A histogram whose HistogramOpts is turned into GaugeOpts: its _bucket series are gone.
mutant_rejected "histogram no longer a histogram" \
  "${RUL}: kollect_export_duration_seconds_bucket is not a registered metric (kollect_export_duration_seconds is registered but not with HistogramOpts" \
  'NewGaugeVec\(prometheus\.GaugeOpts\{ *Name: *"kollect_export_duration_seconds"' "${M}" \
  bash -c 'python3 - '"${M}"' <<'"'EOF'"'
import re, sys
p = sys.argv[1]
s = open(p).read()
s2 = re.sub(r"prometheus\.NewHistogramVec\(\s*prometheus\.HistogramOpts\{\s*Name:\s*(\"kollect_export_duration_seconds\")",
            r"prometheus.NewGaugeVec(prometheus.GaugeOpts{ Name: \1", s, count=1)
open(p, "w").write(s2)
EOF'

# A commented-out registration is not registration.
mutant_rejected "name only in a comment" \
  "${DOC}: kollect_commented_total is not a registered metric" \
  '// .*"kollect_commented_total"' "${M}" \
  bash -c 'printf "\n// Name: \"kollect_commented_total\",\n/* Name: \"kollect_commented_total\", */\n" >> '"${M}"' &&
    printf "\n| \`kollect_commented_total\` | counter | — | x |\n" >> '"${DOC}"

# A literal in a _test.go file is not registration.
mutant_rejected "name only in a test file" \
  "${DOC}: kollect_test_only_total is not a registered metric" \
  'kollect_test_only_total' internal/metrics/extra_test.go \
  bash -c 'printf "package metrics\n\nvar testOnly = \"kollect_test_only_total\"\n" > internal/metrics/extra_test.go &&
    printf "\n| \`kollect_test_only_total\` | counter | — | x |\n" >> '"${DOC}"

mutant_rejected "Namespace/Subsystem split name" \
  "${M}: sets Namespace -- the metric name is split into parts" \
  'Namespace: "kollect"' "${M}" \
  sed -i -E '0,/^(\t+)Name: "kollect_reconcile_total",$/s//\1Namespace: "kollect",\n&/' "${M}"

mutant_rejected "HistogramOpts with an unresolvable Name" \
  "a HistogramOpts Name does not resolve to a kollect_* literal (exportName)" \
  'Name: +exportName,' "${M}" \
  sed -i -E 's/^(\t+)Name: +"kollect_export_duration_seconds",$/\1Name: exportName,/' "${M}"

mutant_rejected "registration outside internal/metrics" \
  "internal/othermetrics/x.go: imports the Prometheus client outside internal/metrics" \
  'client_golang/prometheus"' internal/othermetrics/x.go \
  bash -c 'mkdir -p internal/othermetrics && printf "package othermetrics\n\nimport \"github.com/prometheus/client_golang/prometheus\"\n\nvar _ = prometheus.NewCounter\n" > internal/othermetrics/x.go'

# Every kollect_ literal in the sources is gone: the scan is empty.
mutant_rejected "empty source scan" \
  "internal/metrics yields no registered kollect_* name" \
  '"kellect_reconcile_total"' "${M}" \
  sed -i -E 's/"kollect_/"kellect_/g' internal/metrics/metrics.go internal/metrics/aggregation.go internal/metrics/aggregation_labeled.go

mutant_rejected "empty rules scan" \
  "${RUL} yields no kollect_* name" \
  'kellect_workqueue_depth' "${RUL}" \
  sed -i -E 's/kollect_/kellect_/g' "${RUL}"

mutant_rejected "missing docs page" \
  "${DOC} is missing" ABSENT "${DOC}" \
  rm "${DOC}"

mutant_rejected "missing rules template" \
  "${RUL} is missing" ABSENT "${RUL}" \
  rm "${RUL}"

# Green controls: every histogram series suffix is accepted, and a Name given by a const resolves.
control_passes "histogram _bucket/_sum/_count accepted" 'kollect_reconcile_duration_seconds_count' "${DOC}" \
  bash -c 'printf "\n\`kollect_reconcile_duration_seconds_bucket\` \`kollect_reconcile_duration_seconds_sum\` \`kollect_reconcile_duration_seconds_count\`\n" >> '"${DOC}"

control_passes "HistogramOpts Name from a const" 'Name: +exportName,' "${M}" \
  bash -c 'sed -i -E "s/^(\t+)Name: +\"kollect_export_duration_seconds\",$/\1Name: exportName,/" '"${M}"' &&
    printf "\nconst exportName = \"kollect_export_duration_seconds\"\n" >> '"${M}"

echo "All metrics consistency self-tests passed."
