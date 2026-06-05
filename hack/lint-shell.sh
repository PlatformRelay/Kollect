#!/usr/bin/env bash
# Lint repository shell scripts with ShellCheck (pinned to v0.11.0 in CI).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v shellcheck >/dev/null 2>&1; then
  echo "shellcheck not found; install ShellCheck v0.11.0 or run: pre-commit run shellcheck --all-files" >&2
  exit 1
fi

mapfile -t scripts < <(find "${ROOT}/hack" -name '*.sh' -type f | sort)
scripts+=("${ROOT}/.devcontainer/post-install.sh")

shellcheck --severity=warning "${scripts[@]}"
