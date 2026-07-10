#!/usr/bin/env bash
# L4 kind smoke for the pipeline CLI (ADR-0801, P-008).
#
# The pipeline CLI (`kollect-pipeline collect`) is standalone: it reads KollectProfile/KollectTarget
# YAML from disk and collects via the kubeconfig, without the operator installed. This scenario
# builds the CLI, seeds a Deployment in a fresh namespace, runs a one-shot collection into a local
# output directory, and asserts at least one inventory file was written.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

_kind_require kubectl
kind_use_context "${CLUSTER_NAME:-kollect-e2e}"

_log() { echo "[pipeline-cli-smoke] $*"; }

readonly SMOKE_NS="pipeline-smoke"

_log "Building the kollect-pipeline CLI (task build:cli)..."
( cd "${REPO_ROOT}" && task build:cli )
readonly CLI_BIN="${REPO_ROOT}/bin/kollect-pipeline"
[[ -x "${CLI_BIN}" ]] || { echo "expected built CLI at ${CLI_BIN}" >&2; exit 1; }

_log "Seeding namespace and Deployment..."
kubectl create namespace "${SMOKE_NS}" --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${SMOKE_NS}" create deployment nginx --image=nginx:1.25 --dry-run=client -o yaml | kubectl apply -f -

# The pipeline CLI lists the Deployment object (not its pods), so it is collectable as soon as the
# object exists — no rollout wait required.

config_dir="$(mktemp -d)"
output_dir="$(mktemp -d)"
trap 'rm -rf "${config_dir}" "${output_dir}"' EXIT

cat > "${config_dir}/profile.yaml" <<'EOF'
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: deploy-images
  namespace: default
spec:
  targetGVK:
    group: apps
    version: v1
    kind: Deployment
  attributes:
    - name: image
      path: "$.spec.template.spec.containers[0].image"
EOF

cat > "${config_dir}/target.yaml" <<EOF
apiVersion: kollect.dev/v1alpha1
kind: KollectTarget
metadata:
  name: deploy-target
  namespace: default
spec:
  profileRef: deploy-images
  includedNamespaces:
    - ${SMOKE_NS}
EOF

ctx="$(kubectl config current-context)"
_log "Running one-shot collection (context=${ctx})..."
"${CLI_BIN}" collect \
  --config "${config_dir}" \
  --output "${output_dir}" \
  --context "${ctx}"

_log "Asserting at least one inventory file was written..."
if ! find "${output_dir}" -type f -name '*.yaml' | grep -q .; then
  echo "no inventory file written under ${output_dir}" >&2
  find "${output_dir}" -type f -print >&2 || true
  exit 1
fi

written="$(find "${output_dir}" -type f -name '*.yaml' | head -1)"
_log "Wrote ${written}:"
cat "${written}"

if ! grep -q 'nginx:1.25' "${written}"; then
  echo "inventory file does not contain the collected image attribute" >&2
  exit 1
fi

_log "Pipeline CLI smoke passed."
