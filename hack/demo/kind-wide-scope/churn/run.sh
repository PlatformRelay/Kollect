#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Declarative churn runner — schedule from churn/steps.yaml (JSON-compatible YAML).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=../lib/ui.sh
source "${DEMO_DIR}/lib/ui.sh"

readonly KUBECTL="${KUBECTL:-kubectl}"
readonly STEPS_FILE="${CHURN_STEPS_FILE:-${SCRIPT_DIR}/steps.yaml}"
readonly PRESET="${CHURN_PRESET:-fast}"

demo_require_gum
demo_intro "Churn — watch inventory drift land in Git and the UI"

_load_preset() {
  python3 - "${STEPS_FILE}" "${PRESET}" <<'PY'
import json, sys

path, preset = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
p = data["presets"].get(preset)
if not p:
    raise SystemExit(f"unknown preset: {preset}")
print(p["initial_wait_sec"])
print(p["step_gap_sec"])
print(p.get("description", ""))
PY
}

_run_step() {
  local idx="$1"
  python3 - "${STEPS_FILE}" "${idx}" <<'PY'
import json, subprocess, sys, time

steps_path, idx = sys.argv[1], int(sys.argv[2])
kubectl = __import__("os").environ.get("KUBECTL", "kubectl")
script_dir = __import__("os").path.dirname(steps_path)
demo_dir = __import__("os").path.dirname(script_dir)
churn_dir = steps_path.rsplit("/", 1)[0] if "/" in steps_path else script_dir

with open(steps_path, encoding="utf-8") as f:
    steps = json.load(f)["steps"]
step = steps[idx]
label = step["label"]
print(label)

if "delete" in step:
    subprocess.run([kubectl, *step["delete"]], check=False)
    time.sleep(5)

if "cmd" in step:
    subprocess.run([kubectl, *step["cmd"]], check=True)

if "manifest" in step:
    manifest = step["manifest"]
    path = manifest if manifest.startswith("/") else f"{churn_dir}/{manifest}"
    subprocess.run([kubectl, "apply", "-f", path], check=True)
PY
}

mapfile -t _preset_vals < <(_load_preset)
initial_wait="${_preset_vals[0]}"
step_gap="${_preset_vals[1]}"
preset_desc="${_preset_vals[2]:-}"

demo_info "Preset **${PRESET}** — ${preset_desc}"
demo_info "Initial wait ${initial_wait}s, gap ${step_gap}s between steps."

if command -v gum >/dev/null 2>&1; then
  gum style --foreground 212 --bold "$(date +%H:%M:%S)" "T+0 — baseline; waiting ${initial_wait}s"
else
  echo "=== T+0 — waiting ${initial_wait}s ==="
fi
sleep "${initial_wait}"

step_count="$(python3 -c "import json; print(len(json.load(open('${STEPS_FILE}'))['steps']))")"
for ((i = 0; i < step_count; i++)); do
  label="$(_run_step "${i}")"
  if command -v gum >/dev/null 2>&1; then
    gum style --foreground 212 --bold "$(date +%H:%M:%S)" "${label}"
  else
    echo "=== [$(date +%H:%M:%S)] ${label} ==="
  fi
  if (( i + 1 < step_count )); then
    sleep "${step_gap}"
  fi
done

demo_outcome "Churn complete (${PRESET}) — check UI catalog, Read API itemCount, and Git commits"
