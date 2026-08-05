#!/usr/bin/env bash
# LAB-DOC-04: local backend/emulator fidelity matrix must list every registered
# sink type once, mark emulator/lab substitutes as PASS_WITH_LIMITATION (not
# managed-cloud proof), and stay wired in MkDocs + docs verify.
# Offline only — no live kubectl.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PAGE="${ROOT}/docs/operator-manual/lab-backend-fidelity.md"
NAV="${ROOT}/mkdocs.yml"
VERIFY="${ROOT}/hack/docs/verify.sh"
INDEX="${ROOT}/docs/operator-manual/index.md"
RUNBOOK="${ROOT}/docs/operator-manual/local-lab-runbook.md"
EVIDENCE="${ROOT}/docs/operator-manual/lab-evidence-bundle.md"
REGISTRY="${ROOT}/internal/sink/registry.go"

fail() {
  printf 'lab-doc-04 fidelity: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${PAGE}" ]] || fail "${PAGE} is missing"
[[ -f "${REGISTRY}" ]] || fail "${REGISTRY} is missing"

# Registry types from NewRegistry (must appear exactly once in the primary matrix table).
types=(local git gitlab s3 gcs postgres mongodb kafka nats bigquery)
PRIMARY="$(awk '/^## Primary fidelity matrix$/{p=1;next} /^## /{if(p){exit}} p' "${PAGE}")"
[[ -n "${PRIMARY}" ]] || fail "missing ## Primary fidelity matrix section"

for t in "${types[@]}"; do
  count="$(printf '%s\n' "${PRIMARY}" | grep -E "^\\|[[:space:]]*\`${t}\`[[:space:]]*\\|" | wc -l | tr -d ' ')"
  [[ "${count}" -eq 1 ]] || fail "registry type ${t} must appear exactly once in primary matrix (found ${count})"
done
pass "each NewRegistry sink type appears exactly once in primary matrix"

# Fidelity / limitation contract
grep -qF 'PASS_WITH_LIMITATION' "${PAGE}" || fail "page must require PASS_WITH_LIMITATION for emulator/lab substitutes"
grep -Eqi 'not.*(managed|SaaS|cloud).*(IAM|quota|parity|proof)|never.*(managed|SaaS|cloud).*(proof|parity)|emulator.*(not|never).*(IAM|quota|managed)' "${PAGE}" ||
  fail "page must state emulator/lab success is not managed-cloud IAM/quota/parity proof"
grep -Eqi 'MinIO.*(not|never|≠|!=).*GCS|MinIO.*(not|never).*Google.*(IAM|API)|S3-compatible.*(not|never).*GCS.*(IAM|API)|GCS.*(not|never).*Google IAM' "${PAGE}" ||
  fail "page must state MinIO/S3-compatible paths are not Google IAM/API proof"
grep -Eqi 'Redpanda.*(not|never|≠|!=).*managed.*Kafka|Redpanda.*(not|never).*Kafka.*(control.plane|SaaS|managed)' "${PAGE}" ||
  fail "page must state Redpanda is not managed Kafka control-plane proof"
grep -Eqi 'Forgejo.*(not|never|≠|!=).*(GitHub|GitLab)|Forgejo.*(not|never).*(SaaS|managed)' "${PAGE}" ||
  fail "page must state Forgejo is not GitHub/GitLab SaaS proof"
grep -Eqi 'BigQuery emulator.*(not|never).*(IAM|billing|GCP)|emulator.*(not|never).*(billing|IAM)' "${PAGE}" ||
  fail "page must state BigQuery emulator is not GCP IAM/billing proof"
pass "fidelity / not-managed-cloud caveats present"

# Lab substitutes + schedule honesty
for token in minio forgejo redpanda github gitlab; do
  grep -Eqi "${token}" "${PAGE}" || fail "page must mention lab substitute ${token}"
done
grep -Eqi 'quick\+sinks' "${PAGE}" || fail "page must reference quick+sinks schedule membership"
grep -Eqi 'H07|deferred.*backend.profile|backend.profile.*deferred' "${PAGE}" ||
  fail "page must note H07 backend profiles deferred (ADR-0707)"
pass "lab substitutes and schedule/H07 honesty present"

# Lab-local secrets / no real credentials required
grep -Eqi 'lab-local|no real credential|without.*(real|production).*credential|example.*(secret|endpoint).*lab' "${PAGE}" ||
  fail "page must state examples are lab-local and do not require real credentials"
pass "lab-local secret/endpoint guidance present"

# Nav + verify wiring
grep -qF 'operator-manual/lab-backend-fidelity.md' "${NAV}" ||
  fail "mkdocs.yml must list lab-backend-fidelity.md"
grep -qF 'docs_lab_doc_04_fidelity_test.sh' "${VERIFY}" ||
  fail "hack/docs/verify.sh must invoke docs_lab_doc_04_fidelity_test.sh"
pass "mkdocs nav and verify.sh wiring present"

# Cross-links
grep -Eqi 'lab-backend-fidelity' "${INDEX}" || fail "operator-manual/index.md must link lab-backend-fidelity"
if ! grep -Eqi 'lab-backend-fidelity' "${RUNBOOK}" && ! grep -Eqi 'lab-backend-fidelity' "${EVIDENCE}"; then
  fail "local-lab-runbook.md or lab-evidence-bundle.md must cross-link lab-backend-fidelity"
fi
grep -Eqi 'lab-evidence-bundle|local-lab-runbook|ADR-0707|0707-lab-harness' "${PAGE}" ||
  fail "fidelity page must cross-link evidence/runbook/ADR-0707"
pass "cross-links present"

printf 'All LAB-DOC-04 fidelity docs tests passed.\n'
