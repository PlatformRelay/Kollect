#!/usr/bin/env bash
# Nightly shuffled, repeated unit run (TDS-2, openspec change test-depth-signals).
#
# Runs every unit package three times under one -shuffle seed, so a test that depends on the
# tests before it, or on running once per process, fails on some night and the log names the
# seed that reproduces it:
#   - a package whose _test.go files call RunSpecs (Ginkgo) runs as three `go test -count=1`
#     invocations: Ginkgo v2 refuses any other -count;
#   - every other package runs once at -count=3 (one shuffled order, repeated three times).
# Packages come from `go list ./...` only, minus EXCLUDE below. Envtest is provisioned as
# hack/coverage.sh does. The seed is SHUFFLE_SEED (a decimal integer) or a fresh one, printed
# before any test runs. -shuffle reorders top-level Test functions only; inside RunSpecs Ginkgo
# keeps its own seed, which its suite output reports.
#
# Usage (from the repository root):
#   task test:shuffle                         # a fresh seed
#   SHUFFLE_SEED=<n> task test:shuffle        # rerun a nightly's seed
# Locked by hack/test/test_shuffle_test.sh.
set -euo pipefail

say() { printf 'test-shuffle: %s\n' "$*"; }
die() {
  printf '::error::test-shuffle: %s\n' "$*"
  exit 1
}

# Packages left out, one "<import path>|<reason>" per entry. Only e2e packages may be listed
# (decision A, 2026-10-16: envtest packages stay in) and every entry needs a reason.
# Empty on 2026-10-18: `go list ./...` holds no e2e package.
EXCLUDE=(
)
for entry in "${EXCLUDE[@]}"; do
  pkg="${entry%%|*}" reason="${entry#*|}"
  [[ "${entry}" == *"|"* && -n "${reason// /}" ]] || die "exclusion '${entry}' has no reason"
  [[ "${pkg}" =~ (^|/)e2e(/|$) ]] || die "exclusion '${pkg}' is not an e2e package; only e2e packages may be excluded"
done

if [[ -n "${SHUFFLE_SEED+set}" ]]; then # set, even empty: an empty seed is an error, not "pick one"
  [[ "${SHUFFLE_SEED}" =~ ^[0-9]{1,18}$ ]] || die "SHUFFLE_SEED must be a decimal integer, got '${SHUFFLE_SEED}'"
  seed="$((10#${SHUFFLE_SEED}))"
else
  seed="$(od -An -N4 -tu4 /dev/urandom | tr -d ' ')"
fi
say "seed ${seed} (reproduce: SHUFFLE_SEED=${seed} task test:shuffle)"

make setup-envtest >&2
echo >&2 # setup-envtest prints its path without a newline
KUBEBUILDER_ASSETS="$(make -s echo-kubebuilder-assets)" || die "make -s echo-kubebuilder-assets failed"
export KUBEBUILDER_ASSETS
[[ -n "${KUBEBUILDER_ASSETS}" ]] || die "make -s echo-kubebuilder-assets printed nothing: envtest assets missing"

listing="$(go list -f '{{.ImportPath}} {{.Dir}}' ./...)" || die "go list ./... failed"
plain=() ginkgo=()
while read -r pkg dir; do
  [[ -n "${pkg}" ]] || continue
  excluded=""
  for entry in "${EXCLUDE[@]}"; do
    [[ "${pkg}" != "${entry%%|*}" ]] || excluded="${entry#*|}"
  done
  if [[ -n "${excluded}" ]]; then
    say "excluded ${pkg}: ${excluded}"
  elif grep -s 'RunSpecs(' "${dir}"/*_test.go >/dev/null; then
    ginkgo+=("${pkg}")
  else
    plain+=("${pkg}")
  fi
done <<<"${listing}"
((${#plain[@]} + ${#ginkgo[@]} > 0)) || die "go list ./... gave no package to test"
say "${#plain[@]} packages at -count=3; Ginkgo (RunSpecs) packages three times at -count=1: ${ginkgo[*]:-none}"

rc=0
if ((${#plain[@]})); then
  go test -shuffle="${seed}" -count=3 "${plain[@]}" || rc=1
fi
if ((${#ginkgo[@]})); then
  for run in 1 2 3; do
    say "Ginkgo run ${run} of 3"
    go test -shuffle="${seed}" -count=1 "${ginkgo[@]}" || rc=1
  done
fi
if ((rc)); then
  die "FAILED with seed ${seed}; reproduce: SHUFFLE_SEED=${seed} task test:shuffle"
fi
say "passed with seed ${seed}"
