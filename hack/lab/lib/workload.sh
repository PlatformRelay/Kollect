#!/usr/bin/env bash
# Lab harness workload library (LAB-H03 / ADR-0707).
# Source from hack/lab/workload.sh — do not execute.
# SPDX-License-Identifier: MIT
# shellcheck shell=bash
#
# Minimal labeled workload helper: deterministic seed, batch manifest render,
# optional light churn plan. Always labels kollect.dev/lab-run=<RUN_ID>.
# Dry-run / fixture modes never call kubectl.

: "${LAB_WORKLOAD_LOG_PREFIX:=[lab-workload]}"

lab_workload_log() { printf '%s %s\n' "${LAB_WORKLOAD_LOG_PREFIX}" "$*"; }
lab_workload_err() { printf '%s FAIL: %s\n' "${LAB_WORKLOAD_LOG_PREFIX}" "$*" >&2; }

lab_workload_usage() {
  cat <<'EOF'
Usage: hack/lab/workload.sh --run-id <RUN_ID> [options]

Minimal labeled workload helper (LAB-H03 / ADR-0707). Renders a small mixed
batch of objects (Deployments, Services, ConfigMaps) with deterministic names
from --seed. Optional --mode=churn emits a light create/update/delete plan.
Not required inside default quick / quick+sinks schedules.

Always sets label kollect.dev/lab-run=<RUN_ID>. Namespaces use
kollect-lab-<RUN_ID>-wl.

Options:
  --run-id <id>           Required. Lab run id (isolation label value).
  --seed <n>              Deterministic seed (default: 1).
  --mode batch|churn      batch = render apply set; churn = light op plan (default: batch).
  --count <n>             Approximate object budget (default: 6; min 3).
  --out-dir <dir>         Directory for rendered YAML / churn plan (required for dry-run).
  --dry-run               Fixture/offline: write manifests only; never call kubectl.
  --fixture               Alias for --dry-run (offline meta-tests).
  -h, --help              Show this help.

Env:
  KOLLECT_LAB_WORKLOAD_FIXTURE  If set to 1, force dry-run/fixture mode.

Exit codes:
  0  OK
  1  usage / invalid args / render failure
EOF
}

# Portable integer hash from seed + salt → non-negative 32-bit-ish int.
lab_workload_hash() {
  local seed="$1"
  local salt="${2:-0}"
  # bash arithmetic: mix seed and salt without external deps.
  local h=$(( (seed * 1103515245 + 12345 + salt * 9973) & 0x7fffffff ))
  printf '%d' "${h}"
}

lab_workload_ns_name() {
  local run_id="$1"
  printf 'kollect-lab-%s-wl' "${run_id}"
}

# Write Namespace + mixed batch objects into out_dir. Deterministic for (run_id, seed, count).
# Usage: lab_workload_render_batch <out_dir> <run_id> <seed> <count>
lab_workload_render_batch() {
  local out_dir="$1"
  local run_id="$2"
  local seed="$3"
  local count="$4"

  if [[ -z "${out_dir}" || -z "${run_id}" ]]; then
    lab_workload_err "render_batch: out_dir and run_id required"
    return 1
  fi
  if ((count < 3)); then
    count=3
  fi

  mkdir -p "${out_dir}"
  local ns
  ns="$(lab_workload_ns_name "${run_id}")"
  local label="kollect.dev/lab-run: ${run_id}"

  # Object mix: roughly Deployment / Service / ConfigMap cycling (minimal shapes).
  # Budget: 1 Namespace + (count) workload objects.
  local ns_file="${out_dir}/00-namespace.yaml"
  cat >"${ns_file}" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
  labels:
    ${label}
    kollect.dev/lab-component: workload
EOF

  local i kind name file idx h
  for ((i = 0; i < count; i++)); do
    h="$(lab_workload_hash "${seed}" "${i}")"
    idx=$((h % 3))
    case "${idx}" in
      0) kind="Deployment" ;;
      1) kind="Service" ;;
      2) kind="ConfigMap" ;;
    esac
    # Include seed in name so different seeds diverge even if kinds collide.
    name="$(printf 'lab-%s-%05d' "${kind,,}" "$((h % 100000))")"
    file="$(printf '%s/%02d-%s.yaml' "${out_dir}" "$((i + 1))" "${name}")"

    case "${kind}" in
      Deployment)
        cat >"${file}" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    ${label}
    app: ${name}
    kollect.dev/lab-seed: "${seed}"
spec:
  replicas: $((1 + h % 2))
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
        ${label}
    spec:
      containers:
        - name: pause
          image: registry.k8s.io/pause:3.9
EOF
        ;;
      Service)
        cat >"${file}" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    ${label}
    app: ${name}
    kollect.dev/lab-seed: "${seed}"
spec:
  selector:
    app: lab-deployment-$(printf '%05d' "$((h % 100000))")
  ports:
    - name: http
      port: $((8000 + h % 1000))
      targetPort: 8080
EOF
        ;;
      ConfigMap)
        cat >"${file}" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    ${label}
    kollect.dev/lab-seed: "${seed}"
data:
  seed: "${seed}"
  index: "${i}"
  payload: "lab-workload-${h}"
EOF
        ;;
    esac
  done

  lab_workload_log "rendered batch run_id=${run_id} seed=${seed} count=${count} ns=${ns} → ${out_dir}"
  return 0
}

# Light churn plan from the same seed: ~60% update, 20% create, 20% delete (of count).
# Writes churn-plan.txt plus any create manifests under out_dir/churn-creates/.
# Usage: lab_workload_render_churn <out_dir> <run_id> <seed> <count>
lab_workload_render_churn() {
  local out_dir="$1"
  local run_id="$2"
  local seed="$3"
  local count="$4"

  if ((count < 3)); then
    count=3
  fi

  # Start from a batch baseline so labels/shapes exist for update/delete targets.
  lab_workload_render_batch "${out_dir}/baseline" "${run_id}" "${seed}" "${count}" || return 1

  mkdir -p "${out_dir}/churn-creates"
  local plan="${out_dir}/churn-plan.txt"
  local ns
  ns="$(lab_workload_ns_name "${run_id}")"
  local label="kollect.dev/lab-run: ${run_id}"

  local n_update n_create n_delete
  n_update=$(((count * 60) / 100))
  n_create=$(((count * 20) / 100))
  n_delete=$(((count * 20) / 100))
  ((n_update < 1)) && n_update=1
  ((n_create < 1)) && n_create=1
  ((n_delete < 1)) && n_delete=1

  {
    printf '# Light churn plan (LAB-H03) run_id=%s seed=%s count=%s\n' "${run_id}" "${seed}" "${count}"
    printf '# ops: create=%s update=%s delete=%s\n' "${n_create}" "${n_update}" "${n_delete}"
    printf '# label: kollect.dev/lab-run=%s\n' "${run_id}"
  } >"${plan}"

  local i h name
  for ((i = 0; i < n_update; i++)); do
    h="$(lab_workload_hash "${seed}" "$((1000 + i))")"
    name="$(printf 'lab-deployment-%05d' "$((h % 100000))")"
    printf 'update Deployment/%s/%s replicas=%s\n' "${ns}" "${name}" "$((2 + h % 3))" >>"${plan}"
  done
  for ((i = 0; i < n_create; i++)); do
    h="$(lab_workload_hash "${seed}" "$((2000 + i))")"
    name="$(printf 'lab-churn-cm-%05d' "$((h % 100000))")"
    printf 'create ConfigMap/%s/%s\n' "${ns}" "${name}" >>"${plan}"
    cat >"${out_dir}/churn-creates/${name}.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name}
  namespace: ${ns}
  labels:
    ${label}
    kollect.dev/lab-seed: "${seed}"
    kollect.dev/lab-churn: create
data:
  churn: "create"
  seed: "${seed}"
  index: "${i}"
EOF
  done
  for ((i = 0; i < n_delete; i++)); do
    h="$(lab_workload_hash "${seed}" "$((3000 + i))")"
    name="$(printf 'lab-configmap-%05d' "$((h % 100000))")"
    printf 'delete ConfigMap/%s/%s\n' "${ns}" "${name}" >>"${plan}"
  done

  lab_workload_log "rendered churn plan create=${n_create} update=${n_update} delete=${n_delete} → ${plan}"
  return 0
}

# Live apply path (not used in meta-tests). Skipped entirely under dry-run/fixture.
lab_workload_apply_dir() {
  local dir="$1"
  if ! command -v kubectl >/dev/null 2>&1; then
    lab_workload_err "kubectl not found; cannot apply (use --dry-run)"
    return 1
  fi
  kubectl apply -f "${dir}"
}

lab_workload_main() {
  local run_id="" seed=1 mode="batch" count=6 out_dir="" dry_run=0
  local arg

  if [[ "${KOLLECT_LAB_WORKLOAD_FIXTURE:-}" == "1" ]]; then
    dry_run=1
  fi

  while [[ $# -gt 0 ]]; do
    arg="$1"
    case "${arg}" in
      --run-id)
        [[ $# -ge 2 ]] || { lab_workload_err "--run-id requires a value"; lab_workload_usage >&2; return 1; }
        run_id="$2"
        shift 2
        ;;
      --run-id=*)
        run_id="${arg#--run-id=}"
        shift
        ;;
      --seed)
        [[ $# -ge 2 ]] || { lab_workload_err "--seed requires a value"; return 1; }
        seed="$2"
        shift 2
        ;;
      --seed=*)
        seed="${arg#--seed=}"
        shift
        ;;
      --mode)
        [[ $# -ge 2 ]] || { lab_workload_err "--mode requires a value"; return 1; }
        mode="$2"
        shift 2
        ;;
      --mode=*)
        mode="${arg#--mode=}"
        shift
        ;;
      --count)
        [[ $# -ge 2 ]] || { lab_workload_err "--count requires a value"; return 1; }
        count="$2"
        shift 2
        ;;
      --count=*)
        count="${arg#--count=}"
        shift
        ;;
      --out-dir)
        [[ $# -ge 2 ]] || { lab_workload_err "--out-dir requires a value"; return 1; }
        out_dir="$2"
        shift 2
        ;;
      --out-dir=*)
        out_dir="${arg#--out-dir=}"
        shift
        ;;
      --dry-run|--fixture)
        dry_run=1
        shift
        ;;
      -h|--help)
        lab_workload_usage
        return 0
        ;;
      *)
        lab_workload_err "unknown argument: ${arg}"
        lab_workload_usage >&2
        return 1
        ;;
    esac
  done

  if [[ -z "${run_id}" ]]; then
    lab_workload_err "--run-id is required"
    lab_workload_usage >&2
    return 1
  fi
  if [[ "${run_id}" == *"/"* || "${run_id}" == *".."* ]]; then
    lab_workload_err "invalid --run-id (no path components): ${run_id}"
    return 1
  fi
  if ! [[ "${seed}" =~ ^[0-9]+$ ]]; then
    lab_workload_err "--seed must be a non-negative integer"
    return 1
  fi
  if ! [[ "${count}" =~ ^[0-9]+$ ]] || ((count < 1)); then
    lab_workload_err "--count must be a positive integer"
    return 1
  fi
  case "${mode}" in
    batch|churn) ;;
    *)
      lab_workload_err "invalid --mode '${mode}' (want batch|churn)"
      return 1
      ;;
  esac

  if [[ "${dry_run}" -eq 1 && -z "${out_dir}" ]]; then
    lab_workload_err "--dry-run/--fixture requires --out-dir"
    return 1
  fi
  if [[ -z "${out_dir}" ]]; then
    # Live path default: still require an explicit out-dir for render-then-apply.
    lab_workload_err "--out-dir is required"
    return 1
  fi

  mkdir -p "${out_dir}"

  if [[ "${dry_run}" -eq 1 ]]; then
    lab_workload_log "dry-run / fixture mode (no kubectl); writing under ${out_dir}"
  fi

  case "${mode}" in
    batch)
      lab_workload_render_batch "${out_dir}" "${run_id}" "${seed}" "${count}" || return 1
      ;;
    churn)
      lab_workload_render_churn "${out_dir}" "${run_id}" "${seed}" "${count}" || return 1
      ;;
  esac

  if [[ "${dry_run}" -eq 0 ]]; then
    lab_workload_log "applying rendered manifests under ${out_dir}"
    if [[ "${mode}" == "batch" ]]; then
      lab_workload_apply_dir "${out_dir}" || return 1
    else
      lab_workload_apply_dir "${out_dir}/baseline" || return 1
      if [[ -d "${out_dir}/churn-creates" ]]; then
        lab_workload_apply_dir "${out_dir}/churn-creates" || return 1
      fi
      lab_workload_log "churn delete/update ops are planned in ${out_dir}/churn-plan.txt (apply deletes manually / runner)"
    fi
  fi

  lab_workload_log "workload OK mode=${mode} run_id=${run_id} seed=${seed} label=kollect.dev/lab-run=${run_id}"
  return 0
}
