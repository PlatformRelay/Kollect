#!/usr/bin/env bash
# Fail unless OpenSpec loads openspec/config.yaml and hands its context and rules to every
# artifact. OpenSpec only warns about an unparseable config or an unknown artifact ID in
# `rules`, and carries on without them, so the project rules could vanish silently.
set -euo pipefail

root=$(pwd)
cli="${root}/hack/tooling/openspec/node_modules/.bin/openspec"
export DO_NOT_TRACK=1 OPENSPEC_TELEMETRY=0 OPENSPEC_NO_UPDATE_CHECK=1

# Probe in a scratch copy so no throwaway change is ever written into the repository.
scratch=$(mktemp -d)
trap 'rm -rf "${scratch}"' EXIT
mkdir -p "${scratch}/openspec"
cp "${root}/openspec/config.yaml" "${scratch}/openspec/config.yaml"
cd "${scratch}"

fail() {
  printf '%s\n' "$2" >&2
  echo "openspec/config.yaml: $1" >&2
  exit 1
}

out=$("${cli}" new change config-probe 2>&1) || fail "openspec new change failed" "${out}"
for artifact in proposal specs design tasks; do
  out=$("${cli}" instructions "${artifact}" --change config-probe 2>&1) ||
    fail "openspec instructions ${artifact} failed" "${out}"
  if grep -qiE 'could not parse|unknown artifact id' <<<"${out}"; then
    fail "OpenSpec warned while loading it (${artifact})" "${out}"
  fi
  grep -q '<project_context>' <<<"${out}" || fail "no context reached ${artifact}" "${out}"
  grep -q '<rules>' <<<"${out}" || fail "no rules reached ${artifact}" "${out}"
done
