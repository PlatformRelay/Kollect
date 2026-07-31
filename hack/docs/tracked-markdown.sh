#!/usr/bin/env bash
set -euo pipefail

# Emit only Git-tracked Markdown paths. NUL delimiters preserve unusual names and
# --no-globs at the caller prevents markdownlint-cli2 from re-expanding config globs.
git ls-files -z -- '*.md'
