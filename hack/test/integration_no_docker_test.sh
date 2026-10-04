#!/usr/bin/env bash
# Prove what a container-backed integration test does on a host WITHOUT Docker.
# SPDX-License-Identifier: MIT
#
# testcontainers-go falls back to /var/run/docker.sock whatever DOCKER_HOST says, so
# "no Docker" cannot be simulated on a workstation that has it. This runs one L3 test
# inside a golang container with no Docker socket mounted and asserts both outcomes:
#
#   KOLLECT_REQUIRE_DOCKER unset  -> the test SKIPs (developer default)
#   KOLLECT_REQUIRE_DOCKER=true   -> the test FAILs (what CI's test-integration job sets)
#
# Requires Docker on the host (to start the Docker-less container). Usage:
#   task test-integration:no-docker
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

fail() {
  printf 'integration no-docker: %s\n' "$*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || fail "docker is needed on the host to start the Docker-less container"

go_version="$(sed -n 's/^go //p' go.mod | head -1)"
[[ -n "${go_version}" ]] || fail "go.mod declares no go version"
image="golang:${go_version}"
test_name="TestExportPostgres"
package="./internal/sink/postgres/"

mounts=(-v "${repo_root}:/src:ro")
if modcache="$(go env GOMODCACHE 2>/dev/null)" && [[ -d "${modcache}" ]]; then
  mounts+=(-v "${modcache}:/go/pkg/mod:ro")
fi

# A named volume keeps the build cache between the two runs (and across invocations), so only the
# first run compiles. The module cache comes read-only from the host; run `go mod download` there
# first if the script reports that the test did not run on a fresh clone.
cache_volume="kollect-no-docker-gocache"

run_without_docker() {
  docker run --rm "${mounts[@]}" -v "${cache_volume}:/gocache" -w /src \
    -e GOTOOLCHAIN=local -e GOCACHE=/gocache \
    -e TESTCONTAINERS_RYUK_DISABLED=true -e "KOLLECT_REQUIRE_DOCKER=$1" \
    "${image}" go test -tags=integration -count=1 -v -run "^${test_name}\$" "${package}" 2>&1
}

set +e
optional_out="$(run_without_docker "")"
optional_rc=$?
required_out="$(run_without_docker "true")"
required_rc=$?
set -e

# A mistyped -run pattern exits 0 with "no tests to run"; that is not a skip.
for out in "${optional_out}" "${required_out}"; do
  grep -q "^=== RUN   ${test_name}\$" <<<"${out}" ||
    fail "${test_name} did not run; output:"$'\n'"${out}"
done

if [[ ${optional_rc} -ne 0 ]] || ! grep -q -- "--- SKIP: ${test_name}" <<<"${optional_out}" ||
  ! grep -q "docker not available" <<<"${optional_out}"; then
  fail "without KOLLECT_REQUIRE_DOCKER the test must SKIP; output:"$'\n'"${optional_out}"
fi

if [[ ${required_rc} -eq 0 ]] || ! grep -q -- "--- FAIL: ${test_name}" <<<"${required_out}" ||
  ! grep -q "docker required" <<<"${required_out}"; then
  fail "with KOLLECT_REQUIRE_DOCKER=true the test must FAIL; output:"$'\n'"${required_out}"
fi

printf 'integration no-docker: ok (%s skips when optional, fails when required; %s)\n' \
  "${test_name}" "${image}"
