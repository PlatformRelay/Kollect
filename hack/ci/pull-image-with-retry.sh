#!/usr/bin/env bash
set -euo pipefail

readonly IMAGE="${1:?usage: pull-image-with-retry.sh IMAGE}"
readonly MAX_ATTEMPTS="${PULL_RETRY_ATTEMPTS:-4}"
readonly INITIAL_DELAY_SECONDS="${PULL_RETRY_DELAY_SECONDS:-5}"

if [[ ! "${MAX_ATTEMPTS}" =~ ^[1-9][0-9]*$ ]]; then
  echo "PULL_RETRY_ATTEMPTS must be a positive integer" >&2
  exit 2
fi
if [[ ! "${INITIAL_DELAY_SECONDS}" =~ ^[0-9]+$ ]]; then
  echo "PULL_RETRY_DELAY_SECONDS must be a non-negative integer" >&2
  exit 2
fi

attempt=1
delay_seconds="${INITIAL_DELAY_SECONDS}"
while ((attempt <= MAX_ATTEMPTS)); do
  echo "Pulling ${IMAGE} (attempt ${attempt}/${MAX_ATTEMPTS})"
  if docker pull "${IMAGE}"; then
    exit 0
  fi

  if ((attempt == MAX_ATTEMPTS)); then
    break
  fi

  echo "Pull failed; retrying in ${delay_seconds}s" >&2
  sleep "${delay_seconds}"
  delay_seconds=$((delay_seconds * 2))
  attempt=$((attempt + 1))
done

echo "Failed to pull ${IMAGE} after ${MAX_ATTEMPTS} attempts" >&2
exit 1
