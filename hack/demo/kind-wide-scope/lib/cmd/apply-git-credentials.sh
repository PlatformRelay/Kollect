#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
set -euo pipefail

: "${KUBECTL:=kubectl}"
: "${GITHUB_TOKEN:?GITHUB_TOKEN required}"
"${KUBECTL}" create secret generic git-push-credentials -n default \
  --from-literal=token="${GITHUB_TOKEN}" \
  --dry-run=client -o yaml | "${KUBECTL}" apply -f -
