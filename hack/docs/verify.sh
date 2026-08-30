#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly repo_root
cd "${repo_root}"

task lint:markdown

# Product truth and architecture contracts intentionally remain small, focused
# shell tests. This composition point makes them impossible for Docs CI to omit.
bash hack/test/docs_launch_truth_test.sh
bash hack/test/docs_adr_kollectsink_retcon_test.sh
bash hack/test/docs_coverage_floor_drift_test.sh
bash hack/test/security_architecture_docs_test.sh
bash hack/test/docs_removed_api_fields_test.sh
bash hack/test/ui_removal_reference_test.sh
bash hack/test/hyg_ui_gitignore_test.sh
bash hack/test/docs_pages_concurrency_test.sh
bash hack/test/docs_lab_evidence_contract_test.sh
bash hack/test/docs_lab_doc_01_runbook_test.sh
bash hack/test/docs_lab_doc_03_scale_claims_test.sh
bash hack/test/docs_lab_doc_04_fidelity_test.sh
bash hack/test/docs_lab_doc_05_scenario_matrix_test.sh
bash hack/test/lab_adr_0707_indexed_test.sh
bash hack/test/lab_harness_meta_suite.sh
bash hack/test/demo_04_samples_kustomize_test.sh
bash hack/test/repo_root_links_test.sh
bash hack/test/dist_adr_0708_indexed_test.sh
bash hack/test/dist_adr_0709_indexed_test.sh
bash hack/test/dist_install_docs_test.sh

# These contracts cover the complete nav and redirect map, orphan pages, current
# CRD/family-sink language, referenced samples and images, and the visual system.
python3 -m unittest discover -s test/docs -p 'test_*.py'

# Validate every committed sample against the shipped API and renderer logic.
go test ./test/samples

# Validate every YAML example published on the site against the same strict decode
# plus committed-CRD-schema checks the samples get.
go test ./test/docs

# Strict mode turns links, snippets, redirects, omitted pages, and render warnings
# into a required failure rather than publishing a partially broken site.
mkdocs build --strict

if [[ -n "${CHROME_BIN:-}" ]] || command -v google-chrome-stable >/dev/null 2>&1 ||
  command -v google-chrome >/dev/null 2>&1 || command -v chromium >/dev/null 2>&1 ||
  command -v chromium-browser >/dev/null 2>&1 ||
  [[ -x "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]]; then
  python3 test/docs/docs_visual_browser_test.py
elif [[ "${DOCS_REQUIRE_CHROME:-0}" == "1" ]]; then
  printf 'docs verify: Chrome/Chromium is required but was not found\n' >&2
  exit 1
else
  printf 'docs verify: Chrome/Chromium unavailable; browser layout check skipped locally\n'
fi

printf 'docs verify: all available checks passed\n'
