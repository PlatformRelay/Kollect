#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Thin wrapper around the kollect CLI stub for GitOps-friendly remote secret YAML.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec go run "${ROOT}/cmd/kollect" create-remote-secret "$@"
