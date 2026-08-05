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
  --run-id <id>           Required. DNS1123 label (lowercase alnum / '-' ; no spaces).
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

# DNS1123 label: lowercase alphanumeric, '-' allowed; start/end alnum; 1–63 chars.
# Rejects spaces and uppercase (lab run ids are lowercase by convention).
lab_workload_valid_run_id() {
  local id="${1:-}"
  [[ -n "${id}" ]] || return 1
  [[ "${id}" != *"/"* && "${id}" != *".."* ]] || return 1
  [[ "${id}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || return 1
  ((${#id} <= 63)) || return 1
  return 0
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

lab_workload_kind_slug() {
  case "$1" in
    Deployment) printf 'deployment' ;;
    Service) printf 'service' ;;
    ConfigMap) printf 'configmap' ;;
    *) printf '%s' "${1,,}" ;;
  esac
}

# Write Namespace + mixed batch objects into out_dir. Deterministic for (run_id, seed, count).
# Also writes .lab-workload-inventory (Kind name) for churn targeting.
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

  # Phase 1: plan kinds/names (force index 0 = Deployment so Services can select a real app).
  local -a plan_kinds=() plan_names=() plan_hashes=()
  local -a dep_names=()
  local i kind name h idx slug

  for ((i = 0; i < count; i++)); do
    h="$(lab_workload_hash "${seed}" "${i}")"
    if ((i == 0)); then
      kind="Deployment"
    else
      idx=$((h % 3))
      case "${idx}" in
        0) kind="Deployment" ;;
        1) kind="Service" ;;
        2) kind="ConfigMap" ;;
      esac
    fi
    slug="$(lab_workload_kind_slug "${kind}")"
    name="$(printf 'lab-%s-%05d' "${slug}" "$((h % 100000))")"
    plan_kinds+=("${kind}")
    plan_names+=("${name}")
    plan_hashes+=("${h}")
    if [[ "${kind}" == "Deployment" ]]; then
      dep_names+=("${name}")
    fi
  done

  if ((${#dep_names[@]} == 0)); then
    lab_workload_err "internal: batch plan has no Deployments"
    return 1
  fi

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

  local inventory="${out_dir}/.lab-workload-inventory"
  : >"${inventory}"

  # Phase 2: emit manifests; Service selectors pick a real Deployment app label.
  local file dep_app dep_idx
  for ((i = 0; i < count; i++)); do
    kind="${plan_kinds[i]}"
    name="${plan_names[i]}"
    h="${plan_hashes[i]}"
    file="$(printf '%s/%02d-%s.yaml' "${out_dir}" "$((i + 1))" "${name}")"
    printf '%s %s\n' "${kind}" "${name}" >>"${inventory}"

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
        dep_idx=$((i % ${#dep_names[@]}))
        dep_app="${dep_names[dep_idx]}"
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
    app: ${dep_app}
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

# Load inventory lines "Kind name" from baseline dir into parallel arrays via namerefs.
# Usage: lab_workload_load_inventory <baseline_dir> <kinds_arr_name> <names_arr_name>
lab_workload_load_inventory() {
  local baseline="$1"
  local -n _kinds_ref="$2"
  local -n _names_ref="$3"
  _kinds_ref=()
  _names_ref=()
  local inv="${baseline}/.lab-workload-inventory"
  local line kind name
  if [[ -f "${inv}" ]]; then
    while IFS= read -r line || [[ -n "${line}" ]]; do
      [[ -z "${line}" || "${line}" == \#* ]] && continue
      kind="${line%% *}"
      name="${line#* }"
      [[ -n "${kind}" && -n "${name}" ]] || continue
      _kinds_ref+=("${kind}")
      _names_ref+=("${name}")
    done <"${inv}"
    return 0
  fi
  # Fallback: scan YAML if inventory missing.
  local f
  for f in "${baseline}"/*.yaml "${baseline}"/*.yml; do
    [[ -f "${f}" ]] || continue
    kind="$(awk '/^kind:/{print $2; exit}' "${f}")"
    name="$(awk '/^metadata:/{m=1} m && /^[[:space:]]+name:/{print $2; exit}' "${f}")"
    name="${name#\"}"
    name="${name%\"}"
    [[ "${kind}" == "Namespace" ]] && continue
    [[ -n "${kind}" && -n "${name}" ]] || continue
    _kinds_ref+=("${kind}")
    _names_ref+=("${name}")
  done
}

# Light churn plan from the same seed: ~60% update, 20% create, 20% delete (of count).
# Update/delete targets are real baseline object names (same seed/count batch).
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

  local -a base_kinds=() base_names=()
  lab_workload_load_inventory "${out_dir}/baseline" base_kinds base_names
  if ((${#base_names[@]} == 0)); then
    lab_workload_err "churn: baseline inventory empty"
    return 1
  fi

  local -a update_pool=() delete_pool=()
  local i kind name
  for ((i = 0; i < ${#base_names[@]}; i++)); do
    kind="${base_kinds[i]}"
    name="${base_names[i]}"
    case "${kind}" in
      Deployment)
        update_pool+=("Deployment/${name}")
        ;;
      ConfigMap|Service)
        delete_pool+=("${kind}/${name}")
        ;;
    esac
  done

  # If no ConfigMap/Service for delete, allow deleting a Deployment not chosen for update.
  if ((${#delete_pool[@]} == 0)); then
    for ((i = 0; i < ${#base_names[@]}; i++)); do
      if [[ "${base_kinds[i]}" == "Deployment" ]]; then
        delete_pool+=("Deployment/${base_names[i]}")
      fi
    done
  fi

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

  # Cap to available pools (still emit at least one update/delete when pool non-empty).
  if ((${#update_pool[@]} == 0)); then
    lab_workload_err "churn: no updateable baseline objects"
    return 1
  fi
  if ((${#delete_pool[@]} == 0)); then
    lab_workload_err "churn: no deletable baseline objects"
    return 1
  fi
  ((n_update > ${#update_pool[@]})) && n_update=${#update_pool[@]}
  ((n_delete > ${#delete_pool[@]})) && n_delete=${#delete_pool[@]}

  {
    printf '# Light churn plan (LAB-H03) run_id=%s seed=%s count=%s\n' "${run_id}" "${seed}" "${count}"
    printf '# ops: create=%s update=%s delete=%s\n' "${n_create}" "${n_update}" "${n_delete}"
    printf '# label: kollect.dev/lab-run=%s\n' "${run_id}"
    printf '# update/delete targets are baseline object names (executable)\n'
  } >"${plan}"

  local picked_update=" " picked_delete=" " entry obj_kind obj_name h
  local -a used_for_update=()

  for ((i = 0; i < n_update; i++)); do
    h="$(lab_workload_hash "${seed}" "$((1000 + i))")"
    entry="${update_pool[$((h % ${#update_pool[@]}))]}"
    # Prefer unused entries when possible.
    local j
    for ((j = 0; j < ${#update_pool[@]}; j++)); do
      entry="${update_pool[$(((h + j) % ${#update_pool[@]}))]}"
      if [[ "${picked_update}" != *" ${entry} "* ]]; then
        break
      fi
    done
    picked_update+="${entry} "
    obj_kind="${entry%%/*}"
    obj_name="${entry#*/}"
    used_for_update+=("${obj_name}")
    printf 'update %s/%s/%s replicas=%s\n' "${obj_kind}" "${ns}" "${obj_name}" "$((2 + h % 3))" >>"${plan}"
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
    entry="${delete_pool[$((h % ${#delete_pool[@]}))]}"
    local j
    for ((j = 0; j < ${#delete_pool[@]}; j++)); do
      entry="${delete_pool[$(((h + j) % ${#delete_pool[@]}))]}"
      obj_name="${entry#*/}"
      # Avoid deleting something we just planned to update when alternatives exist.
      if [[ "${picked_delete}" == *" ${entry} "* ]]; then
        continue
      fi
      local conflict=0 u
      for u in "${used_for_update[@]+"${used_for_update[@]}"}"; do
        if [[ "${u}" == "${obj_name}" ]]; then
          conflict=1
          break
        fi
      done
      if [[ "${conflict}" -eq 0 ]]; then
        break
      fi
    done
    picked_delete+="${entry} "
    obj_kind="${entry%%/*}"
    obj_name="${entry#*/}"
    printf 'delete %s/%s/%s\n' "${obj_kind}" "${ns}" "${obj_name}" >>"${plan}"
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
  if ! lab_workload_valid_run_id "${run_id}"; then
    lab_workload_err "invalid --run-id '${run_id}' (want DNS1123 label: [a-z0-9]([-a-z0-9]*[a-z0-9])?, no spaces, ≤63)"
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
