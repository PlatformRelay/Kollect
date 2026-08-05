#!/usr/bin/env bash
# LAB-H02: resumable runner + schedule registry + serial-backend (offline only).
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNNER="${ROOT}/hack/lab/run.sh"
SERIAL_LIB="${ROOT}/hack/lab/lib/serial-backend.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab runner resume meta: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${RUNNER}" ]] || fail "missing ${RUNNER}"
[[ -f "${SERIAL_LIB}" ]] || fail "missing ${SERIAL_LIB}"
[[ -f "${ROOT}/hack/lab/schedules/quick.json" || -f "${ROOT}/hack/lab/schedules/quick.yaml" ]] ||
  fail "missing schedules/quick.{json,yaml}"
[[ -f "${ROOT}/hack/lab/schedules/quick+sinks.json" || -f "${ROOT}/hack/lab/schedules/quick+sinks.yaml" ]] ||
  fail "missing schedules/quick+sinks.{json,yaml}"

# Poison PATH: any real kubectl/helm invocation is a hard test failure.
cat >"${TMP}/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "lab runner resume meta: unexpected live kubectl: $*" >&2
exit 99
EOF
cat >"${TMP}/helm" <<'EOF'
#!/usr/bin/env bash
echo "lab runner resume meta: unexpected live helm: $*" >&2
exit 99
EOF
chmod +x "${TMP}/kubectl" "${TMP}/helm"
export PATH="${TMP}:${PATH}"

# shellcheck disable=SC1091
source "${SERIAL_LIB}"

# ---------------------------------------------------------------------------
# Serial backend: must refuse next backend while previous is still up
# ---------------------------------------------------------------------------
STATE="${TMP}/serial.state"
lab_serial_backend_reset "${STATE}" || fail "reset failed"
lab_serial_backend_assert_clear "${STATE}" || fail "fresh state should be clear"

lab_serial_backend_begin "${STATE}" "postgres" || fail "begin postgres"
lab_serial_backend_mark_up "${STATE}" "postgres" || fail "mark postgres up"
if lab_serial_backend_begin "${STATE}" "minio" 2>/dev/null; then
  fail "begin minio must fail while postgres is still up"
fi
lab_serial_backend_teardown "${STATE}" "postgres" || fail "teardown postgres"
lab_serial_backend_assert_clear "${STATE}" || fail "after teardown state should be clear"
lab_serial_backend_begin "${STATE}" "minio" || fail "begin minio after teardown"
pass "serial-backend enforces tear-down before next backend"

# ---------------------------------------------------------------------------
# Refuse unimplemented schedules (explicit BLOCKED / refuse — not silent pass)
# ---------------------------------------------------------------------------
rc=0
out="$(
  env -u KUBECONFIG PATH="${PATH}" \
    bash "${RUNNER}" --dry-run --schedule full-lab-day --run-id refuse-full \
      --artifacts-root "${TMP}/art" 2>&1
)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "full-lab-day must refuse (non-zero): ${out}"
printf '%s\n' "${out}" | grep -Eqi 'BLOCKED|refuse|not implemented|unimplemented' ||
  fail "full-lab-day refuse should say BLOCKED/refuse: ${out}"
pass "full-lab-day refused"

rc=0
out="$(
  env -u KUBECONFIG PATH="${PATH}" \
    bash "${RUNNER}" --dry-run --schedule soak --run-id refuse-soak \
      --artifacts-root "${TMP}/art" 2>&1
)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "soak must refuse (non-zero): ${out}"
printf '%s\n' "${out}" | grep -Eqi 'BLOCKED|refuse|not implemented|unimplemented' ||
  fail "soak refuse should say BLOCKED/refuse: ${out}"
pass "soak refused"

# ---------------------------------------------------------------------------
# Dry-run quick: expands schedule, writes results.json, no live cluster tools
# ---------------------------------------------------------------------------
RUN_ID="meta-quick-1"
ART="${TMP}/art"
mkdir -p "${ART}"
if ! out="$(
  env -u KUBECONFIG PATH="${PATH}" \
    KOLLECT_LAB_PREFLIGHT_FIXTURE=clean \
    bash "${RUNNER}" \
      --dry-run \
      --schedule quick \
      --run-id "${RUN_ID}" \
      --seed 42 \
      --tier auto \
      --artifacts-root "${ART}" \
      --exec-log "${TMP}/exec-quick.log" \
      2>&1
)"; then
  fail "quick dry-run failed: ${out}"
fi

RESULTS="${ART}/${RUN_ID}/results.json"
[[ -f "${RESULTS}" ]] || fail "missing results.json at ${RESULTS}; out=${out}"

# Every scenario row must have id + verdict; non-PASS need non-empty reason
jq -e '
  (.schedule == "quick")
  and (.run_id == "'"${RUN_ID}"'")
  and (.scenarios | type == "array")
  and (.scenarios | length >= 1)
  and all(.scenarios[];
    (.id | type == "string" and length > 0)
    and (.verdict | type == "string" and length > 0)
    and (
      .verdict == "PASS"
      or ((.reason | type == "string") and (.reason | length > 0))
    )
  )
' "${RESULTS}" >/dev/null || fail "results.json shape/reasons invalid: $(cat "${RESULTS}")"

# quick must include core IDs; Wave-2b sinks are schedule-excluded (SKIPPED + reason), never PASS
for id in DR-0.1 DR-1.1 DR-2.1 DR-3.1 DR-4.3; do
  jq -e --arg id "${id}" '
    [.scenarios[] | select(.id == $id)]
    | length == 1
    and (.[0].verdict == "PASS" or .[0].verdict == "PASS_WITH_LIMITATION")
  ' "${RESULTS}" >/dev/null || fail "quick results missing runnable ${id}"
done
for id in DR-2b.3 DR-2b.11; do
  jq -e --arg id "${id}" '
    [.scenarios[] | select(.id == $id)]
    | length == 1
    and .[0].verdict == "SKIPPED"
    and ((.[0].reason | length) > 0)
    and ((.[0].reason | test("schedule-exclusion|quick\\+sinks")) )
  ' "${RESULTS}" >/dev/null ||
    fail "quick must SKIP sink ${id} with schedule-exclusion reason"
done
# Exec log must not list sink scenarios for quick
if grep -E '^DR-2b\.' "${TMP}/exec-quick.log" >/dev/null 2>&1; then
  fail "quick must not execute Wave-2b scenarios: $(cat "${TMP}/exec-quick.log")"
fi
pass "quick dry-run expands schedule and emits reasoned results"

# ---------------------------------------------------------------------------
# quick+sinks includes serial backend scenarios
# ---------------------------------------------------------------------------
RUN_ID2="meta-sinks-1"
if ! out="$(
  env -u KUBECONFIG PATH="${PATH}" \
    KOLLECT_LAB_PREFLIGHT_FIXTURE=clean \
    bash "${RUNNER}" \
      --dry-run \
      --schedule quick+sinks \
      --run-id "${RUN_ID2}" \
      --artifacts-root "${ART}" \
      --exec-log "${TMP}/exec-sinks.log" \
      2>&1
)"; then
  fail "quick+sinks dry-run failed: ${out}"
fi
RESULTS2="${ART}/${RUN_ID2}/results.json"
for id in DR-2b.3 DR-2b.4 DR-2b.5 DR-2b.11 DR-2b.12; do
  jq -e --arg id "${id}" '[.scenarios[].id] | index($id) != null' "${RESULTS2}" >/dev/null ||
    fail "quick+sinks missing ${id}"
done
pass "quick+sinks includes serial backend IDs"

# ---------------------------------------------------------------------------
# Resume: skip scenarios already PASS in results.json (do not re-exec)
# ---------------------------------------------------------------------------
RUN_ID3="meta-resume-1"
# Seed a partial results.json as if DR-0.1 and DR-0.2 already PASS'd
mkdir -p "${ART}/${RUN_ID3}"
cat >"${ART}/${RUN_ID3}/results.json" <<EOF
{
  "run_id": "${RUN_ID3}",
  "schedule": "quick",
  "seed": 7,
  "scenarios": [
    {"id": "DR-0.1", "verdict": "PASS", "reason": ""},
    {"id": "DR-0.2", "verdict": "PASS", "reason": ""}
  ]
}
EOF

: >"${TMP}/exec-resume.log"
if ! out="$(
  env -u KUBECONFIG PATH="${PATH}" \
    KOLLECT_LAB_PREFLIGHT_FIXTURE=clean \
    bash "${RUNNER}" \
      --dry-run \
      --schedule quick \
      --run-id "${RUN_ID3}" \
      --resume \
      --seed 7 \
      --artifacts-root "${ART}" \
      --exec-log "${TMP}/exec-resume.log" \
      2>&1
)"; then
  fail "resume dry-run failed: ${out}"
fi

# Already-PASS IDs must not appear in exec log
if grep -E '^(DR-0\.1|DR-0\.2)$' "${TMP}/exec-resume.log" >/dev/null 2>&1; then
  fail "resume re-executed PASS scenarios: $(cat "${TMP}/exec-resume.log")"
fi
# But a later scenario should have been executed
grep -Eq '^DR-' "${TMP}/exec-resume.log" ||
  fail "resume should still execute remaining scenarios: $(cat "${TMP}/exec-resume.log")"

# Final results still have DR-0.1 as PASS (preserved) and more rows
jq -e '
  ([.scenarios[] | select(.id == "DR-0.1") | .verdict] | index("PASS") != null)
  and (.scenarios | length > 2)
' "${ART}/${RUN_ID3}/results.json" >/dev/null ||
  fail "resume results should preserve PASS and continue: $(cat "${ART}/${RUN_ID3}/results.json")"
pass "resume skips already-PASS scenarios"

# ---------------------------------------------------------------------------
# Excluded / skipped rows never look like empty green (reason required)
# ---------------------------------------------------------------------------
jq -e '
  all(.scenarios[] | select(.verdict != "PASS");
    (.reason | type == "string") and (.reason | length > 0)
  )
' "${ART}/${RUN_ID2}/results.json" >/dev/null ||
  fail "non-PASS rows must carry machine-emitted reasons"
pass "skip/blocked/limit reasons are non-empty"

# ---------------------------------------------------------------------------
# Flags surface in --help / README
# ---------------------------------------------------------------------------
help_out="$(bash "${RUNNER}" --help 2>&1 || true)"
for flag in --schedule --run-id --resume --seed --keep-lab --tier --dry-run; do
  printf '%s\n' "${help_out}" | grep -Fq -- "${flag}" ||
    grep -Fq -- "${flag}" "${ROOT}/hack/lab/README.md" ||
    fail "flag ${flag} missing from --help and README"
done
pass "runner flags documented"

# Poisoned kubectl/helm never successfully invoked.
pass "no live kubectl/helm invocations"

echo "All lab runner resume meta tests passed."
