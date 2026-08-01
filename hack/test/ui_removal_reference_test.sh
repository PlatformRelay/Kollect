#!/usr/bin/env bash
# Reference check: frozen kollect-ui product surface must be gone (UI-REMOVE-01).
# Fails while SPA/chart/CI/docs paths or product wiring remain. Allowlists
# Charm Gum demo helper lib/ui.sh, webhook paths, and CHANGELOG history.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

fail() {
  printf 'ui removal reference: %s\n' "$*" >&2
  exit 1
}

# --- Paths that must not exist -------------------------------------------------
forbidden_paths=(
  ui
  charts/kollect-ui
  charts/kollect/charts/kollect-ui-0.1.0.tgz
  .github/workflows/ui-ci.yaml
  docs/operator-manual/ui.md
  docs/examples/ui-local-development.md
  docs/assets/ui-inventory-placeholder.png
  docs/assets/ui-inventory-placeholder.svg
  docs/adr/0408-read-api-ui-architecture.md
  docs/adr/0409-kollect-ui-deployment.md
  docs/adr/0410-ui-engineering-and-quality-gates.md
  docs/adr/0411-read-api-extensions-for-ui.md
  docs/adr/0412-mock-read-api-for-ui-development.md
  hack/ci/ui-verify.sh
  hack/verify-ui-headers.sh
  hack/add-ui-headers.sh
  hack/verify-ui-mock.sh
  hack/ui-e2e-docker.sh
  hack/test/sonar_ko_04_ui_automount_test.sh
)

for path in "${forbidden_paths[@]}"; do
  if [[ -e "${path}" ]]; then
    fail "forbidden path still present: ${path}"
  fi
done

# --- Product wiring strings (scanned files; CHANGELOG + lib/ui.sh excluded) ----
scan_files=()
while IFS= read -r -d '' f; do
  scan_files+=("${f}")
done < <(
  find . \
    \( -path './.git' -o -path './ui' -o -path './charts/kollect-ui' \
       -o -path './node_modules' -o -path './docs/node_modules' \
       -o -path './bin' -o -path './dist' -o -path './site' \
       -o -path './.worktrees' -o -path './.claude' \
       -o -path './agent-context' -o -path './references' \) -prune -o \
    -type f \
    \( -name '*.yaml' -o -name '*.yml' -o -name '*.md' -o -name '*.sh' \
       -o -name '*.gotmpl' -o -name 'Taskfile.yml' -o -name 'renovate.json' \
       -o -name 'mkdocs.yml' -o -name '.go-arch-lint.yml' \
       -o -name 'sonar-project.properties' -o -name 'SECURITY.md' \
       -o -name 'GOVERNANCE.md' -o -name 'CONTRIBUTING.md' \
       -o -name 'Chart.lock' \) \
    ! -name 'CHANGELOG.md' \
    ! -path './hack/demo/*/lib/ui.sh' \
    -print0
)

pattern='kollect-ui|UI_IMAGE_|sbom-ui|ui-playwright-msw|ui-ci\.yaml|charts/kollect-ui|operator-manual/ui\.md|ui-local-development|0408-read-api-ui|0409-kollect-ui|0410-ui-engineering|0411-read-api-extensions|0412-mock-read-api|task ui-|build-ui|ghcr\.io/.*/kollect-ui'

hits="$(
  rg -n --no-heading -e "${pattern}" "${scan_files[@]}" 2>/dev/null || true
)"

# Drop allowlisted false positives (webhook "ui", Charm Gum helper mentions).
filtered="$(
  printf '%s\n' "${hits}" | awk '
    NF == 0 { next }
    /hack\/demo\/.*\/lib\/ui\.sh/ { next }
    /webhook/ && !/kollect-ui/ && !/UI_IMAGE_/ && !/sbom-ui/ { next }
    { print }
  '
)"

if [[ -n "${filtered}" ]]; then
  printf '%s\n' "${filtered}" >&2
  fail "residual product UI references remain (see above)"
fi

# Nav must not advertise removed pages.
if grep -Eq 'operator-manual/ui\.md|examples/ui-local-development\.md|0408-|0409-|0410-|0411-|0412-' mkdocs.yml; then
  fail "mkdocs.yml still navigates to removed UI pages or ADRs"
fi

printf 'ui removal reference: ok\n'
