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
bash hack/test/security_architecture_docs_test.sh
bash hack/test/ui_removal_reference_test.sh
bash hack/test/hyg_ui_gitignore_test.sh

# These contracts cover the complete nav and redirect map, orphan pages, current
# CRD/family-sink language, referenced samples and images, and the visual system.
python3 -m unittest discover -s test/docs -p 'test_*.py'

# Validate every committed sample against the shipped API and renderer logic.
go test ./test/samples

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
