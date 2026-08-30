#!/usr/bin/env bash
# Gate for the docs/GLOSSARY.md AUTO-CRD block and its generator.
#
# CONTRIBUTING.md tells contributors to run `python3 hack/gen-glossary.py`
# after a schema description change. Nothing used to check the result, so the
# committed block could drift from the generator output in either direction:
# a CRD field could go missing, and — worse — hand-written prose inside the
# markers was silently destroyed the moment anyone ran the documented command.
#
# This gate closes both holes, plus the untested-generator gap left by the
# CR-REFERENCE.md link fix (PR #336):
#
#   1. committed AUTO-CRD block == freshly generated block (drift)
#   2. curated overrides survive regeneration (no silent prose loss)
#   3. every link the generator emits resolves to a real docs/ page
#   4. fixture: a CRD that gains a spec field reds, naming that field
#   5. fixture: a generator emitting a dead link target reds, naming it
#
# Every generator run happens in a scratch ROOT built with `cp`. The generator
# derives its write target from `Path(__file__).resolve().parents[1]`, and
# `.resolve()` follows symlinks — so a symlinked copy would resolve back to the
# real worktree and let a fixture overwrite the real docs/GLOSSARY.md. Copy,
# never link. The final check re-hashes the real file to prove that held.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT
readonly GENERATOR="${ROOT}/hack/gen-glossary.py"
readonly GLOSSARY="${ROOT}/docs/GLOSSARY.md"
readonly BEGIN_MARKER='<!-- BEGIN AUTO-CRD -->'
readonly END_MARKER='<!-- END AUTO-CRD -->'

# The curated disambiguation that regeneration used to destroy. It contradicts
# the CRD's own description on purpose: admission rejects `type: http`.
readonly CURATED_HTTP_ROW='Reserved snapshot type that is rejected by admission'

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

hash_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

hash_real_glossary() {
  hash_file "${GLOSSARY}"
}

# Build an isolated ROOT the generator can safely write into.
make_scratch_root() {
  local dest="$1"
  mkdir -p "${dest}/hack" "${dest}/config/crd/bases" "${dest}/docs/crds"
  cp "${GENERATOR}" "${dest}/hack/gen-glossary.py"
  cp "${ROOT}"/config/crd/bases/*.yaml "${dest}/config/crd/bases/"
  cp "${GLOSSARY}" "${dest}/docs/GLOSSARY.md"
  cp "${ROOT}"/docs/crds/*.md "${dest}/docs/crds/"
}

# Print the AUTO-CRD block (markers included) of the file in $1.
extract_block() {
  awk -v begin="${BEGIN_MARKER}" -v end="${END_MARKER}" '
    $0 == begin { inside = 1 }
    inside { print }
    $0 == end { inside = 0 }
  ' "$1"
}

# Assert every relative markdown link inside the AUTO-CRD block of $1 resolves
# to a file under the docs directory $2. Deliberately generic: grepping for the
# one filename that broke in PR #336 would only catch the bug that already
# happened.
check_block_links() {
  local file="$1" docs_dir="$2" missing=0 target
  local -a targets=()
  while IFS= read -r target; do
    targets+=("${target}")
  done < <(extract_block "${file}" |
    grep -oE '\]\([^)]+\)' |
    sed -E 's/^\]\(//; s/\)$//' |
    sed -E 's/#.*$//' |
    grep -vE '^(https?:|mailto:)' |
    grep -v '^$' |
    sort -u)

  [[ ${#targets[@]} -gt 0 ]] || fail "no markdown links found in the AUTO-CRD block of ${file}"

  for target in "${targets[@]}"; do
    if [[ ! -f "${docs_dir}/${target}" ]]; then
      echo "  dead link target: ${target}" >&2
      missing=$((missing + 1))
    fi
  done
  return "${missing}"
}

# Add a spec field named $1 to KollectSnapshotSink in the scratch CRD dir $3.
# An empty $2 adds the field with no description at all.
inject_crd_field() {
  local field="$1" desc="$2" crd_dir="$3"
  FIXTURE_FIELD="${field}" FIXTURE_DESC="${desc}" FIXTURE_CRD_DIR="${crd_dir}" \
    python3 - <<'PY'
import os
import pathlib

import yaml

field = os.environ["FIXTURE_FIELD"]
desc = os.environ["FIXTURE_DESC"]
path = pathlib.Path(os.environ["FIXTURE_CRD_DIR"]) / "kollect.dev_kollectsnapshotsinks.yaml"
doc = yaml.safe_load(path.read_text(encoding="utf-8"))
props = doc["spec"]["versions"][0]["schema"]["openAPIV3Schema"]["properties"]["spec"]["properties"]
props[field] = {"type": "string"}
if desc:
    props[field]["description"] = desc
path.write_text(yaml.safe_dump(doc, sort_keys=True), encoding="utf-8")
PY
}

# Replace the description of KollectSnapshotSink spec field $1 in the scratch
# CRD dir $3 with $2.
set_crd_description() {
  local field="$1" desc="$2" crd_dir="$3"
  FIXTURE_FIELD="${field}" FIXTURE_DESC="${desc}" FIXTURE_CRD_DIR="${crd_dir}" \
    python3 - <<'PY'
import os
import pathlib

import yaml

field = os.environ["FIXTURE_FIELD"]
desc = os.environ["FIXTURE_DESC"]
path = pathlib.Path(os.environ["FIXTURE_CRD_DIR"]) / "kollect.dev_kollectsnapshotsinks.yaml"
doc = yaml.safe_load(path.read_text(encoding="utf-8"))
props = doc["spec"]["versions"][0]["schema"]["openAPIV3Schema"]["properties"]["spec"]["properties"]
props[field]["description"] = desc
path.write_text(yaml.safe_dump(doc, sort_keys=True), encoding="utf-8")
PY
}

[[ -f "${GENERATOR}" ]] || fail "expected ${GENERATOR} to exist"
[[ -f "${GLOSSARY}" ]] || fail "expected ${GLOSSARY} to exist"

real_hash_before="$(hash_real_glossary)"
scratch=""

# The "no fixture escaped" check has to run on every exit path. As a terminal
# statement it was unreachable the moment any earlier check called fail(),
# which is exactly the run in which an escaped fixture is most likely.
cleanup() {
  local status=$?
  if [[ "$(hash_real_glossary)" != "${real_hash_before}" ]]; then
    echo "FAIL: a scratch generator run modified the real ${GLOSSARY}" >&2
    status=1
  fi
  [[ -n "${scratch}" ]] && rm -rf "${scratch}"
  exit "${status}"
}
trap cleanup EXIT

# --- markers -------------------------------------------------------------
# Counted on exact whole lines, and required to be unique. `grep -qF` accepted
# a marker with a trailing space that extract_block's `$0 ==` then skipped,
# yielding an empty block and a misleading "block is stale" failure. A
# duplicated pair was worse: patch_glossary splits on the FIRST BEGIN/END, so a
# second block would survive every regeneration untouched, while extract_block
# concatenated both on both sides of the diff and matched.
count_exact_lines() {
  grep -cxF "$1" "${GLOSSARY}" || true
}

begin_count="$(count_exact_lines "${BEGIN_MARKER}")"
end_count="$(count_exact_lines "${END_MARKER}")"
[[ "${begin_count}" == "1" ]] ||
  fail "expected exactly one line reading '${BEGIN_MARKER}' in ${GLOSSARY}, found ${begin_count}"
[[ "${end_count}" == "1" ]] ||
  fail "expected exactly one line reading '${END_MARKER}' in ${GLOSSARY}, found ${end_count}"
pass "exactly one AUTO-CRD marker pair, each on its own exact line"

scratch="$(mktemp -d)"

# --- 1. committed block matches a fresh generation -----------------------
baseline="${scratch}/baseline"
make_scratch_root "${baseline}"
python3 "${baseline}/hack/gen-glossary.py" >/dev/null

if ! diff -u \
  <(extract_block "${GLOSSARY}") \
  <(extract_block "${baseline}/docs/GLOSSARY.md") >"${scratch}/drift.diff"; then
  cat "${scratch}/drift.diff" >&2
  fail "docs/GLOSSARY.md AUTO-CRD block is stale; run: python3 hack/gen-glossary.py"
fi
pass "committed AUTO-CRD block matches hack/gen-glossary.py output"

# --- 2. regeneration is idempotent ---------------------------------------
python3 "${baseline}/hack/gen-glossary.py" >/dev/null
if ! diff -q "${GLOSSARY}" "${baseline}/docs/GLOSSARY.md" >/dev/null; then
  diff -u "${GLOSSARY}" "${baseline}/docs/GLOSSARY.md" >&2 || true
  fail "hack/gen-glossary.py is not idempotent across two consecutive runs"
fi
pass "regeneration is idempotent (whole file, two consecutive runs)"

# --- 3. curated prose survives regeneration ------------------------------
grep -qF "${CURATED_HTTP_ROW}" "${GLOSSARY}" ||
  fail "the curated \`http\` disambiguation is gone from ${GLOSSARY}"
grep -qF "${CURATED_HTTP_ROW}" "${baseline}/docs/GLOSSARY.md" ||
  fail "hack/gen-glossary.py destroyed the curated \`http\` disambiguation on regeneration"
pass "curated \`http\` disambiguation survives regeneration"

# --- 4. every emitted link target exists ---------------------------------
if ! check_block_links "${baseline}/docs/GLOSSARY.md" "${baseline}/docs"; then
  fail "hack/gen-glossary.py emitted link targets that do not exist"
fi
pass "every link the generator emits resolves to a real docs page"

# --- 5. fixture: a CRD that gains a spec field reds, naming the field ----
# Three positions: a name sorting into the leading rows, one sorting past
# them, and one carrying no description. KollectSnapshotSink has more spec
# fields than the table lists, so the latter two only red if the generator
# names the fields it leaves out of the table.
# The third case has no description at all: the curated lookup and the
# omitted-field list both used to sit behind `if desc:`, so an undocumented new
# field was invisible to the page and to this gate alike.
for fixture_field in aaaFixtureField zzzFixtureField undocumentedFixtureField; do
  fixture="${scratch}/${fixture_field}"
  make_scratch_root "${fixture}"
  fixture_desc="${fixture_field} is a fixture-only field."
  [[ "${fixture_field}" == undocumentedFixtureField ]] && fixture_desc=""
  inject_crd_field "${fixture_field}" "${fixture_desc}" "${fixture}/config/crd/bases"
  python3 "${fixture}/hack/gen-glossary.py" >/dev/null

  if diff -u \
    <(extract_block "${GLOSSARY}") \
    <(extract_block "${fixture}/docs/GLOSSARY.md") >"${scratch}/${fixture_field}.diff"; then
    fail "fixture: adding spec field ${fixture_field} produced no drift; the gate is blind to new CRD fields"
  fi
  grep -qF "${fixture_field}" "${scratch}/${fixture_field}.diff" ||
    fail "fixture: drift for ${fixture_field} does not name the field:
$(cat "${scratch}/${fixture_field}.diff")"
  pass "fixture: a CRD gaining spec field ${fixture_field} reds and names it"
done

# --- 6. fixture: a generator emitting a dead link target reds ------------
# Reproduces the class of defect fixed in PR #336 (links to a deleted page).
deadlink="${scratch}/deadlink"
make_scratch_root "${deadlink}"
sed -i.bak 's|crds/index\.md|CR-REFERENCE.md|g' "${deadlink}/hack/gen-glossary.py"
rm -f "${deadlink}/hack/gen-glossary.py.bak"
grep -qF 'CR-REFERENCE.md' "${deadlink}/hack/gen-glossary.py" ||
  fail "fixture: could not patch the scratch generator to emit a dead link target"
# The per-kind pages stay in place on purpose: the redirected link is the only
# dead target, so the patch above is what reds this fixture. Deleting
# docs/crds/*.md as well would have red it with the sed doing nothing.
python3 "${deadlink}/hack/gen-glossary.py" >/dev/null

if check_block_links "${deadlink}/docs/GLOSSARY.md" "${deadlink}/docs" 2>/dev/null; then
  fail "fixture: a generator emitting CR-REFERENCE.md was not caught by the link check"
fi
pass "fixture: a generator emitting a dead link target reds"

# --- 7. fixture: a curated override for a vanished field reds ------------
# The override map is only safe because a key that stops matching a real CRD
# field is a hard error. Without that, deleting a field from the CRD would make
# its curated text disappear as quietly as the prose this gate exists to save.
# The generator must also refuse *before* writing, so a stale map cannot leave
# a half-updated glossary behind.
stale="${scratch}/staleoverride"
make_scratch_root "${stale}"
sed -i.bak 's|"KollectSnapshotSink", "http"|"KollectSnapshotSink", "kollectNoSuchField"|' \
  "${stale}/hack/gen-glossary.py"
rm -f "${stale}/hack/gen-glossary.py.bak"
grep -qF 'kollectNoSuchField' "${stale}/hack/gen-glossary.py" ||
  fail "fixture: could not patch the scratch generator to hold a stale override key"

stale_hash_before="$(hash_file "${stale}/docs/GLOSSARY.md")"
if python3 "${stale}/hack/gen-glossary.py" >"${scratch}/stale.out" 2>&1; then
  fail "fixture: a curated override naming a nonexistent spec field did not fail the generator"
fi
grep -qF 'KollectSnapshotSink.kollectNoSuchField' "${scratch}/stale.out" ||
  fail "fixture: the stale-override failure does not name the offending key:
$(cat "${scratch}/stale.out")"
[[ "$(hash_file "${stale}/docs/GLOSSARY.md")" == "${stale_hash_before}" ]] ||
  fail "fixture: the generator rewrote the glossary despite a stale curated override"
pass "fixture: a curated override for a vanished field reds before writing"

# --- 8. fixture: an override the schema has caught up with reds ----------
# The other half of override rot. Once the CRD description says what the
# curated text says, the override is dead weight that reads as load-bearing,
# and nothing about the rendered output would ever reveal it.
redundant="${scratch}/redundantoverride"
make_scratch_root "${redundant}"
set_crd_description http \
  "Reserved snapshot type that is rejected by admission; do not confuse it with the optional Inventory HTTP read API." \
  "${redundant}/config/crd/bases"

redundant_hash_before="$(hash_file "${redundant}/docs/GLOSSARY.md")"
if python3 "${redundant}/hack/gen-glossary.py" >"${scratch}/redundant.out" 2>&1; then
  fail "fixture: an override repeating the CRD's own description did not fail the generator"
fi
grep -qF 'KollectSnapshotSink.http' "${scratch}/redundant.out" ||
  fail "fixture: the redundant-override failure does not name the offending key:
$(cat "${scratch}/redundant.out")"
[[ "$(hash_file "${redundant}/docs/GLOSSARY.md")" == "${redundant_hash_before}" ]] ||
  fail "fixture: the generator rewrote the glossary despite a redundant curated override"
pass "fixture: a curated override the schema caught up with reds before writing"

# --- 9. no fixture touched the real glossary -----------------------------
[[ "$(hash_real_glossary)" == "${real_hash_before}" ]] ||
  fail "a scratch generator run modified the real ${GLOSSARY}"
pass "the real docs/GLOSSARY.md was not modified by this gate"

echo "All glossary drift tests passed."
