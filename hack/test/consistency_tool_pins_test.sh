#!/usr/bin/env bash
# CFC-2 (openspec change cross-file-consistency-gates): pinned tools build with, and run under,
# the repository's Go.
#
# Developer and CI tooling is pinned in four places: the Makefile's `go-install-tool` calls
# (kustomize, controller-gen, setup-envtest, golangci-lint), `go run <pkg>@<version>` lines in
# Taskfile.yml (go-arch-lint, govulncheck), any `go run|install <pkg>@<version>` under .github/
# or hack/, and the golangci-lint custom-build plugins in hack/tooling/.custom-gcl.yml. CI builds
# every one of them with the Go that go.mod names (setup-go reads go-version-file: go.mod). Two
# things go wrong, and both happened:
#
#   * a pin needs a NEWER Go than go.mod (its own `go` directive is higher), so it does not build
#     under that Go at all;
#   * a pinned ANALYSIS tool is too OLD for go.mod's Go: it builds, but its embedded
#     golang.org/x/tools cannot load or analyse the module. Bumping go.mod to 1.27.2 broke
#     golangci-lint v2.13.1 ("export data version 5 is greater than maximum supported version
#     4"), go-arch-lint v1.15.0 ("package ... without types was imported") and govulncheck
#     v1.1.4 (SSA panic) at once (evidence/G1.md). A go-directive comparison cannot see this.
#
# What it locks:
#   * every pin resolves through `go mod download -json <module>@<version>`, and the package it
#     names exists in that module (a typo in the package path does not pass by resolving the
#     parent module); a lookup failure -- typo, network, proxy -- is a failure, never a pass;
#   * every pin's `go` directive (from the downloaded go.mod) is <= go.mod's;
#   * golangci-lint, govulncheck and go-arch-lint are each pinned (an empty or partial scan never
#     passes) and each runs ONCE against the module under go.mod's Go and exits cleanly:
#       golangci-lint: built by the repo's own `make golangci-lint` target (the custom build with
#                      the logcheck plugin), then `run --default=none --enable=govet` with the
#                      repo config on SMOKE_PKG;
#       govulncheck:   `-format json SMOKE_PKG` (exit status reflects errors, not findings);
#       go-arch-lint:  `check --arch-file .go-arch-lint.yml` on the whole module;
#   * hack/tooling/.custom-gcl.yml's `version:` equals the Makefile's GOLANGCI_LINT_VERSION (the
#     custom build compiles that version, so two values would be two pins).
#
# "Under go.mod's Go": every go command here runs with GOTOOLCHAIN=go<go.mod version>, which is
# exactly that toolchain and never another (no +auto upgrade, no fallback to a newer local Go).
# In CI setup-go installs that version, so it is the local toolchain; elsewhere go fetches it
# once. A tool that needs a newer Go fails here exactly as it would under GOTOOLCHAIN=local in CI.
#
# Network: needed (module proxy, golangci-lint's custom build, the vuln DB). Built tools are
# cached for the run in a temp dir, or in KOLLECT_TOOL_PINS_CACHE when set (the self-test sets
# it so its mutants share one build per tool version).
#
# Usage:
#   hack/test/consistency_tool_pins_test.sh              # check the repository tree
#   hack/test/consistency_tool_pins_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/consistency_tool_pins_test.sh <tree-root>  # check another tree root -- a TRUSTED one:
#                                                          # it runs that tree's Makefile and tools
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi

ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi
ROOT="${ROOT%/}"

# A real package of the module that imports the standard library: loading it is what an analysis
# tool whose x/tools predates the Go version fails on.
SMOKE_PKG="./internal/envtestassets/"
ARCH_FILE=".go-arch-lint.yml"

command -v jq >/dev/null 2>&1 || fail "jq not found; install jq to run this test"
command -v go >/dev/null 2>&1 || fail "go not found; install Go to run this test"
command -v make >/dev/null 2>&1 || fail "make not found; install make to run this test"

# Make and task pass state to children through these; a guard run from `task` or `make` must
# evaluate the Makefile as written.
unset MAKEFLAGS MAKELEVEL MFLAGS
export GOWORK=off

# Every temp dir lives under WORKROOT, created here in the parent shell, so the EXIT trap
# removes them all even though new_tmp mostly runs inside $(...).
WORKROOT="$(mktemp -d)"
FIXTURE_IN_MODCACHE=0
cleanup() {
  local mc
  chmod -R u+w "${WORKROOT}" 2>/dev/null || true
  rm -rf "${WORKROOT}" 2>/dev/null || true
  if [[ "${FIXTURE_IN_MODCACHE}" == 1 ]]; then
    # The self-test's newer-Go fixture module passes through the shared module cache.
    mc="$(go env GOMODCACHE 2>/dev/null || true)"
    if [[ -n "${mc}" ]]; then
      chmod -R u+w "${mc}/cache/download/kollect.invalid" "${mc}/kollect.invalid@v1.0.0" "${mc}/kollect.invalid" 2>/dev/null || true
      rm -rf "${mc}/cache/download/kollect.invalid" "${mc}/kollect.invalid" 2>/dev/null || true
    fi
  fi
  return 0
}
trap cleanup EXIT

new_tmp() {
  mktemp -d "${WORKROOT}/XXXXXXXX"
}

# make_var <root> <NAME>: the value make itself computes for a Makefile variable, so derived pins
# (ENVTEST_VERSION is computed from go.mod) are read exactly as the build reads them.
make_var() {
  # shellcheck disable=SC2016 # $($*) is make syntax, expanded by make, not the shell
  make -s --no-print-directory -C "$1" \
    --eval 'kollect-print-var-%: ; @printf "%s\n" "$($*)"' "kollect-print-var-$2" 2>/dev/null
}

# taskfile_var <root> <NAME>: a top-level `vars:` entry of Taskfile.yml.
taskfile_var() {
  sed -n -E "s/^  $2:[[:space:]]*[\"']?([^\"'#[:space:]]+)[\"']?.*$/\1/p" "$1/Taskfile.yml" | head -1
}

# scan_pins <root>: one "<source>\t<package-or-module>\t<version>\t<kind>" line per pin, kind
# `pkg` (a package path to build) or `mod` (a module path). Unresolvable versions are emitted as
# "<unresolved:...>" so the check reports them instead of dropping them.
scan_pins() {
  local root="$1" call pkg var ver file line spec mod

  # Makefile: $(call go-install-tool,$(BIN),<package>,$(<VERSION_VAR>))
  if [[ -f "${root}/Makefile" ]]; then
    while IFS= read -r call; do
      [[ -n "${call}" ]] || continue
      pkg="$(sed -E 's/^\$\(call go-install-tool,[^,]*,([^,]+),.*$/\1/' <<<"${call}")"
      var="$(sed -E 's/^.*,\$\(([A-Za-z0-9_]+)\)\)$/\1/' <<<"${call}")"
      ver="$(make_var "${root}" "${var}" || true)"
      printf 'Makefile\t%s\t%s\tpkg\n' "${pkg}" "${ver:-<unresolved:\$(${var})>}"
    done < <(grep -oE '\$\(call go-install-tool,[^,]*,[^,]+,\$\([A-Za-z0-9_]+\)\)' "${root}/Makefile" || true)
  fi

  # Lines that run `go run` or `go install` in Makefile recipes, Taskfile.yml, .github/ and hack/
  # (not hack/test, whose guards carry pin-shaped fixtures); EVERY `<domain>/<path>@<version>`
  # token on such a line is a pin, whatever flags (with or without values) come before it, and
  # however many commands share the line. `@latest` is not a pin and is skipped.
  while IFS= read -r line; do
    [[ -n "${line}" ]] || continue
    file="${line%%:*}"
    while IFS= read -r spec; do
      [[ -n "${spec}" ]] || continue
      pkg="${spec%@*}"
      ver="${spec##*@}"
      [[ "${ver}" == "latest" ]] && continue
      if [[ "${ver}" =~ ^\{\{\.([A-Za-z0-9_]+)\}\}$ && "${file}" == "Taskfile.yml" ]]; then
        var="${BASH_REMATCH[1]}"
        ver="$(taskfile_var "${root}" "${var}")"
        [[ -n "${ver}" ]] || ver="<unresolved:{{.${var}}}>"
      elif [[ ! "${ver}" =~ ^v[0-9] ]]; then
        ver="<unresolved:${ver}>"
      fi
      printf '%s\t%s\t%s\tpkg\n' "${file}" "${pkg}" "${ver}"
    done < <(grep -oE '[a-z0-9.-]+\.[a-z]{2,}(/[A-Za-z0-9._~+-]+)+@[^[:space:]"'"'"'`);&|]+' <<<"${line#*:}" || true)
  done < <(
    cd "${root}" && {
      [[ -f Makefile ]] && echo Makefile
      [[ -f Taskfile.yml ]] && echo Taskfile.yml
      [[ -d .github ]] && find .github -type f \( -name '*.yml' -o -name '*.yaml' \)
      [[ -d hack ]] && find hack -path hack/test -prune -o -type f -print
    } | sed 's|^\./||' | sort -u | while IFS= read -r file; do
      # Backslash continuations are joined and quotes dropped first, so `go install \` + `pkg@v`
      # on the next line, and `pkg@"v1.2.3"`, are read as pins too.
      sed -e ':a' -e '/\\$/{N;s/\\\n/ /;ta' -e '}' "${file}" | tr -d "\"'" |
        grep -IE 'go[[:space:]]+(run|install)[[:space:]].*[a-z0-9.-]+\.[a-z]{2,}/[^[:space:]]*@' |
        sed "s|^|${file}:|" || true
    done
  )

  # golangci-lint custom-build plugins: module + version pairs.
  file="hack/tooling/.custom-gcl.yml"
  if [[ -f "${root}/${file}" ]]; then
    awk '
      /^[[:space:]]*-?[[:space:]]*module:/ { m = $0; sub(/^[^:]*:[[:space:]]*/, "", m); gsub(/["\047]/, "", m); next }
      m != "" && /^[[:space:]]+version:/ { v = $0; sub(/^[^:]*:[[:space:]]*/, "", v); gsub(/["\047]/, "", v); print m "\t" v; m = "" }
    ' "${root}/${file}" | while IFS=$'\t' read -r mod ver; do
      printf '%s\t%s\t%s\tmod\n' "${file}" "${mod}" "${ver}"
    done
  fi
}

# resolve <path> <version> <kind> <workdir>: prints "<module>\t<go-directive>"; on failure prints
# "NEWGO\t<module>\t<required go>" (go itself refused: the module needs a newer Go) or
# "ERROR\t<reasons>" and returns 1. For a package path the longest module prefix that resolves
# wins, and the package directory must exist in it.
resolve() {
  local path="$1" ver="$2" kind="$3" work="$4" mod="$1" out e err="" gomod dir rel godir
  while :; do
    out="$(cd "${work}" && go mod download -json "${mod}@${ver}" 2>&1)" || true
    if jq -e '.GoMod and (.Error | not)' >/dev/null 2>&1 <<<"${out}"; then
      break
    fi
    e="$(jq -r '.Error // empty' 2>/dev/null <<<"${out}" || true)"
    [[ -n "${e}" ]] || e="$(tail -1 <<<"${out}")"
    if [[ "${e}" =~ requires\ go\ \>=\ ([0-9][0-9.]*) ]]; then
      printf 'NEWGO\t%s\t%s\n' "${mod}" "${BASH_REMATCH[1]}"
      return 1
    fi
    err="${err:+${err} | }${e}"
    if [[ "${kind}" == "pkg" && "${mod}" == */*/* ]]; then
      mod="${mod%/*}"
      continue
    fi
    printf 'ERROR\t%s\n' "${err}"
    return 1
  done
  gomod="$(jq -r '.GoMod' <<<"${out}")"
  dir="$(jq -r '.Dir // empty' <<<"${out}")"
  if [[ "${kind}" == "pkg" ]]; then
    rel="${path#"${mod}"}"
    if [[ -z "${dir}" ]] || ! compgen -G "${dir}${rel}/*.go" >/dev/null; then
      printf 'ERROR\tmodule %s@%s resolves but has no package %s\n' "${mod}" "${ver}" "${path}"
      return 1
    fi
  fi
  godir="$(sed -n -E 's/^go[[:space:]]+([0-9][0-9.]*)[[:space:]]*(\/\/.*)?$/\1/p' "${gomod}" | head -1)"
  printf '%s\t%s\n' "${mod}" "${godir}"
}

tool_cache() {
  if [[ -n "${KOLLECT_TOOL_PINS_CACHE:-}" ]]; then
    mkdir -p "${KOLLECT_TOOL_PINS_CACHE}"
    echo "${KOLLECT_TOOL_PINS_CACHE}"
  else
    mkdir -p "${WORKROOT}/tools"
    echo "${WORKROOT}/tools"
  fi
}

# build_tool <name> <pkg> <version> <root> <source>: prints the binary path; output of a failed
# build goes to stderr. The Makefile's golangci-lint pin is built by the repo's own Makefile target
# (custom build with plugins), keyed by the Makefile + .custom-gcl.yml contents, and the binary
# must report the pinned version; every other pin (a golangci-lint pinned elsewhere included) is
# built by go install at exactly its version. A build dir is complete only once `.ok` exists.
build_tool() {
  local name="$1" pkg="$2" ver="$3" root="$4" src="$5" cache key dir bin want
  cache="$(tool_cache)"
  if [[ "${name}" == "golangci-lint" && "${src}" == "Makefile" ]]; then
    key="$(cat "${root}/Makefile" "${root}/hack/tooling/.custom-gcl.yml" 2>/dev/null | sha256sum | cut -c1-12)"
    dir="${cache}/${name}-make-${ver}-${key}"
    bin="${dir}/golangci-lint"
    if [[ ! -e "${dir}/.ok" ]]; then
      rm -rf "${dir}"
      make -s --no-print-directory -C "${root}" golangci-lint LOCALBIN="${dir}" >&2 || return 1
      [[ -x "${bin}" ]] || { echo "make golangci-lint produced no binary" >&2; return 1; }
      # Plain build: "has version 2.14.0 built ..."; custom build: "has version v2.14.0-custom-gcl-...".
      want="${ver#v}"
      want="${want//./\\.}"
      if ! "${bin}" version 2>&1 | grep -Eq "has version v?${want}( |-custom-gcl-)"; then
        echo "make golangci-lint built '$("${bin}" version 2>&1 | head -1)', not the pinned ${ver}" >&2
        return 1
      fi
      touch "${dir}/.ok"
    fi
    echo "${bin}"
    return 0
  fi
  dir="${cache}/${name}-${ver}"
  bin="${dir}/${pkg##*/}"
  if [[ ! -e "${dir}/.ok" ]]; then
    rm -rf "${dir}"
    (cd "${cache}" && GOBIN="${dir}" go install "${pkg}@${ver}") >&2 || return 1
    touch "${dir}/.ok"
  fi
  echo "${bin}"
}

# smoke <name> <pkg> <version> <root> <source>: one run of the analysis tool against the module.
# Returns non-zero with the reason on stderr.
smoke() {
  local name="$1" pkg="$2" ver="$3" root="$4" src="$5" bin log rc=0 cfg=()
  log="$(new_tmp)/smoke.log"
  if ! bin="$(build_tool "${name}" "${pkg}" "${ver}" "${root}" "${src}" 2>"${log}")"; then
    echo "build failed: $(grep -v '^[[:space:]]*$' "${log}" | tail -2 | tr '\n' ' ')" >&2
    return 1
  fi
  case "${name}" in
    golangci-lint)
      # The repo config needs the logcheck plugin, which only the Makefile's custom build has.
      [[ "${src}" == "Makefile" ]] || cfg=(--no-config)
      (cd "${root}" && "${bin}" run ${cfg[@]+"${cfg[@]}"} --default=none --enable=govet "${SMOKE_PKG}") >"${log}" 2>&1 || rc=$?
      ;;
    govulncheck)
      (cd "${root}" && "${bin}" -format json "${SMOKE_PKG}") >/dev/null 2>"${log}" || rc=$?
      ;;
    go-arch-lint)
      (cd "${root}" && "${bin}" check --arch-file "${ARCH_FILE}") >"${log}" 2>&1 || rc=$?
      ;;
  esac
  if [[ "${rc}" -eq 0 ]] && grep -Eq '^(panic: |fatal error: )' "${log}"; then
    rc=99
  fi
  if [[ "${rc}" -ne 0 ]]; then
    echo "exit ${rc}: $(grep -Ev '^[[:space:]]*$|level=warning' "${log}" | grep -m1 -E 'panic|error|Error|FAIL|failed|could not' || tail -1 "${log}")" >&2
    return 1
  fi
}

# analysis_tool <pkg>: the analysis tool a package path is, or nothing.
analysis_tool() {
  case "$1" in
    github.com/golangci/golangci-lint/cmd/golangci-lint | github.com/golangci/golangci-lint/v*/cmd/golangci-lint) echo golangci-lint ;;
    golang.org/x/vuln/cmd/govulncheck) echo govulncheck ;;
    github.com/fe3dback/go-arch-lint | github.com/fe3dback/go-arch-lint/v*) echo go-arch-lint ;;
  esac
}

# The whole contract for one tree. Prints one FAIL line per broken invariant, returns non-zero
# when any is broken.
check_tool_pins() {
  local root="$1" broken=0 go_version pins work src path ver kind res mod godir lowest
  local name seen="" gcl_ver mk_ver reason t0 line

  go_version="$(sed -n -E 's/^go[[:space:]]+([0-9][0-9.]*)[[:space:]]*$/\1/p' "${root}/go.mod" 2>/dev/null | head -1)"
  if [[ -z "${go_version}" ]]; then
    echo "FAIL: go.mod has no 'go <version>' directive -- there is no Go to check the pins against" >&2
    return 1
  fi
  export GOTOOLCHAIN="go${go_version}"
  work="$(new_tmp)"
  if [[ "$(cd "${work}" && go env GOVERSION 2>/dev/null)" != "go${go_version}" ]]; then
    echo "FAIL: cannot run go ${go_version} (GOTOOLCHAIN=${GOTOOLCHAIN}) -- the pins can only be checked under go.mod's Go" >&2
    return 1
  fi

  pins="$(scan_pins "${root}" | sort -u)"
  echo "pins found under ${root} (go.mod go ${go_version}):"
  while IFS= read -r line; do echo "  ${line}"; done <<<"${pins}"

  while IFS=$'\t' read -r src path ver kind; do
    [[ -n "${path}" ]] || continue
    if [[ "${ver}" == "<unresolved:"* ]]; then
      echo "FAIL: ${src}: ${path}@${ver} -- the version is not a literal or a variable this check can read, so the pin cannot be checked" >&2
      broken=1
      continue
    fi
    if ! res="$(resolve "${path}" "${ver}" "${kind}" "${work}")"; then
      if [[ "${res}" == NEWGO$'\t'* ]]; then
        IFS=$'\t' read -r _ mod godir <<<"${res}"
        echo "FAIL: ${src}: ${path}@${ver} (module ${mod}) needs go ${godir}, newer than go.mod's go ${go_version} -- it does not build under the repository's Go" >&2
        broken=1
        continue
      fi
      reason="${res#ERROR$'\t'}"
      if [[ "${reason}" == "module "*" has no package "* ]]; then
        echo "FAIL: ${src}: ${path}@${ver} has no package: ${reason}" >&2
      else
        echo "FAIL: ${src}: ${path}@${ver} cannot be resolved by go mod download (${reason}) -- an unresolved pin is never a pass" >&2
      fi
      broken=1
      continue
    fi
    IFS=$'\t' read -r mod godir <<<"${res}"
    if [[ -n "${godir}" && "${godir}" != "${go_version}" ]]; then
      lowest="$(printf '%s\n%s\n' "${godir}" "${go_version}" | sort -V | head -1)"
      if [[ "${lowest}" == "${go_version}" ]]; then
        echo "FAIL: ${src}: ${path}@${ver} (module ${mod}) needs go ${godir}, newer than go.mod's go ${go_version} -- it does not build under the repository's Go" >&2
        broken=1
      fi
    fi
  done <<<"${pins}"

  # golangci-lint's custom build compiles .custom-gcl.yml's version; it must be the Makefile pin.
  mk_ver="$(make_var "${root}" GOLANGCI_LINT_VERSION || true)"
  gcl_ver="$(sed -n -E "s/^version:[[:space:]]*[\"']?([^\"'#[:space:]]+).*$/\1/p" "${root}/hack/tooling/.custom-gcl.yml" 2>/dev/null | head -1)"
  if [[ -f "${root}/hack/tooling/.custom-gcl.yml" && "${gcl_ver}" != "${mk_ver}" ]]; then
    echo "FAIL: hack/tooling/.custom-gcl.yml version ${gcl_ver:-<none>} differs from Makefile GOLANGCI_LINT_VERSION ${mk_ver:-<none>} -- the custom build compiles the former, so the pin is stated twice" >&2
    broken=1
  fi

  # Smoke run: each analysis tool, once per pinned version, against the module.
  while IFS=$'\t' read -r src path ver kind; do
    [[ "${kind}" == "pkg" ]] || continue
    name="$(analysis_tool "${path}")"
    [[ -n "${name}" ]] || continue
    case " ${seen} " in *" ${name}@${ver}@${src} "*) continue ;; esac
    seen="${seen} ${name}@${ver}@${src}"
    if [[ "${ver}" == "<unresolved:"* ]]; then
      continue
    fi
    t0="${SECONDS}"
    if reason="$(smoke "${name}" "${path}" "${ver}" "${root}" "${src}" 2>&1)"; then
      echo "smoke: ${name} ${ver} ran clean under go ${go_version} ($((SECONDS - t0)) s)"
    else
      echo "FAIL: ${name} ${ver} (pinned in ${src}) failed its smoke run under go ${go_version}: ${reason} -- an analysis tool too old for go.mod's Go cannot load the module; a build or network failure fails closed too" >&2
      broken=1
    fi
  done <<<"${pins}"

  for name in golangci-lint govulncheck go-arch-lint; do
    case " ${seen} " in
      *" ${name}@"*) ;;
      *)
        echo "FAIL: no pin found for analysis tool ${name} -- the scan missed it or it was removed; an empty or partial scan never passes" >&2
        broken=1
        ;;
    esac
  done

  return "${broken}"
}

if [[ "${MODE}" == "check" ]]; then
  if check_tool_pins "${ROOT}"; then
    pass "every tool pin resolves, needs no newer Go than go.mod, and each analysis tool runs clean under go.mod's Go"
    exit 0
  fi
  fail "tool pins disagree with go.mod's Go (listed above)"
fi

# ---------------------------------------------------------------------------
# Self-test (CFC-6): copy the tracked tree, break one invariant per copy, and assert the gate
# reds WITH THE MESSAGE of the assertion the mutant targets -- a non-zero exit alone is not
# evidence. One unmutated copy must pass. Tools are built once per version into a shared cache.
# ---------------------------------------------------------------------------
command -v git >/dev/null 2>&1 || fail "git not found; the self-test copies the tracked tree"
command -v zip >/dev/null 2>&1 || fail "zip not found; the self-test builds a module proxy fixture"
git -C "${ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1 ||
  fail "self-test: ${ROOT} is not a git work tree; the self-test copies its tracked files"

export KOLLECT_TOOL_PINS_CACHE="${KOLLECT_TOOL_PINS_CACHE:-$(new_tmp)}"

new_copy() {
  local copy
  copy="$(new_tmp)"
  (cd "${ROOT}" && git ls-files -z | tar --null --ignore-failed-read -T - -cf - 2>/dev/null) |
    tar -xf - -C "${copy}"
  echo "${copy}"
}

# A module proxy on disk serving kollect.invalid/newgo@v1.0.0, whose go.mod needs Go 1.99.0. The
# `go` command itself reads it, so the fixture is checked by the tool it feeds.
make_proxy() {
  local base stage mv
  base="$(new_tmp)"
  stage="${base}/stage/kollect.invalid/newgo@v1.0.0"
  mv="${base}/proxy/kollect.invalid/newgo/@v"
  mkdir -p "${stage}" "${mv}"
  printf 'module kollect.invalid/newgo\n\ngo 1.99.0\n' >"${stage}/go.mod"
  printf 'package main\n\nfunc main() {}\n' >"${stage}/main.go"
  cp "${stage}/go.mod" "${mv}/v1.0.0.mod"
  echo v1.0.0 >"${mv}/list"
  echo '{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}' >"${mv}/v1.0.0.info"
  (cd "${base}/stage" && zip -qrD "${mv}/v1.0.0.zip" kollect.invalid)
  echo "${base}/proxy"
}

# mutant_rejected <label> <sentinel> <probe-regex> <probe-file> <command...>
# The command runs inside the copy; the probe proves the mutation landed before the gate runs.
# MUTANT_ENV (array of NAME=value) is applied to the gate run.
MUTANT_ENV=()
mutant_rejected() {
  local label="$1" sentinel="$2" probe="$3" probe_file="$4"
  shift 4
  local copy out rc t0="${SECONDS}"
  copy="$(new_copy)"
  (cd "${copy}" && "$@") ||
    fail "self-test: the mutation step for '${label}' failed -- nothing was tested"
  grep -Eq -- "${probe}" "${copy}/${probe_file}" ||
    fail "self-test: the '${label}' mutant did not land (no '${probe}' in ${probe_file}) -- nothing was tested"
  out="$(env ${MUTANT_ENV[@]+"${MUTANT_ENV[@]}"} bash "${SELF}" "${copy}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Fq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected '${sentinel}', got: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: ${label} rejected with '${sentinel}' ($((SECONDS - t0)) s)"
}

# No-op control: an unmutated copy passes (and warms the tool cache for the mutants).
CONTROL="$(new_copy)"
if ! out="$(bash "${SELF}" "${CONTROL}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree (or the gate) first: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
fi
pass "self-test: no-op control passes on an unmutated copy"

PROXY="$(make_proxy)"
FIXTURE_IN_MODCACHE=1 # the EXIT trap takes the fixture module out of the shared cache again
MUTANT_ENV=("GOPROXY=file://${PROXY},$(go env GOPROXY)" "GONOSUMDB=kollect.invalid" "GOFLAGS=-modcacherw")
mutant_rejected "pin needs a newer Go than go.mod" \
  "kollect.invalid/newgo@v1.0.0 (module kollect.invalid/newgo) needs go 1.99.0, newer than go.mod's go" \
  'kollect\.invalid/newgo@v1\.0\.0' Taskfile.yml \
  bash -c 'printf "\n  kollect-newgo-fixture:\n    cmds:\n      - go run kollect.invalid/newgo@v1.0.0\n" >> Taskfile.yml'
MUTANT_ENV=()

mutant_rejected "unresolvable pin (version typo)" \
  "golang.org/x/vuln/cmd/govulncheck@v1.9.99 cannot be resolved by go mod download" \
  'govulncheck@v1\.9\.99' Taskfile.yml \
  sed -i -E 's#(golang\.org/x/vuln/cmd/govulncheck)@v[0-9.]+#\1@v1.9.99#' Taskfile.yml

mutant_rejected "package path typo in a real module" \
  "golang.org/x/vuln/cmd/govulncheckk@" 'govulncheckk@' Taskfile.yml \
  sed -i -E 's#golang\.org/x/vuln/cmd/govulncheck@#golang.org/x/vuln/cmd/govulncheckk@#' Taskfile.yml

# setup-envtest is pinned to a branch (release-0.N): resolving it always needs the network, so a
# disabled proxy must fail the gate, not skip the pin.
MUTANT_ENV=("GOPROXY=off")
mutant_rejected "module lookup fails (proxy unreachable)" \
  "sigs.k8s.io/controller-runtime/tools/setup-envtest@release-" '^ENVTEST_VERSION' Makefile \
  true
MUTANT_ENV=()

mutant_rejected "golangci-lint too old for go.mod's Go" \
  "golangci-lint v2.13.1 (pinned in Makefile) failed its smoke run" 'version: v2\.13\.1' hack/tooling/.custom-gcl.yml \
  sed -i -E 's/^(GOLANGCI_LINT_VERSION \?= )v[0-9.]+/\1v2.13.1/; s/^version: v[0-9.]+/version: v2.13.1/' Makefile hack/tooling/.custom-gcl.yml

mutant_rejected "go-arch-lint too old for go.mod's Go" \
  "go-arch-lint v1.15.0 (pinned in Taskfile.yml) failed its smoke run" 'GO_ARCH_LINT_VERSION: v1\.15\.0' Taskfile.yml \
  sed -i -E 's/^(  GO_ARCH_LINT_VERSION: )v[0-9.]+/\1v1.15.0/' Taskfile.yml

mutant_rejected "govulncheck too old for go.mod's Go" \
  "govulncheck v1.1.4 (pinned in Taskfile.yml) failed its smoke run" 'govulncheck@v1\.1\.4' Taskfile.yml \
  sed -i -E 's#(golang\.org/x/vuln/cmd/govulncheck)@v[0-9.]+#\1@v1.1.4#' Taskfile.yml

mutant_rejected "analysis tool pin missing" \
  "no pin found for analysis tool govulncheck" '^  vulncheck:' Taskfile.yml \
  sed -i -E '/golang\.org\/x\/vuln\/cmd\/govulncheck@/d' Taskfile.yml

mutant_rejected "custom-gcl version differs from the Makefile pin" \
  "hack/tooling/.custom-gcl.yml version v2.13.1 differs from Makefile GOLANGCI_LINT_VERSION" \
  '^version: v2\.13\.1' hack/tooling/.custom-gcl.yml \
  sed -i -E 's/^version: v[0-9.]+/version: v2.13.1/' hack/tooling/.custom-gcl.yml

# Extraction: a pin is not dropped because it shares a line, follows a flag that takes a value,
# or sits in a literal Makefile recipe.
mutant_rejected "second pin on one line" \
  "golang.org/x/vuln/cmd/govulncheck@v1.9.98 cannot be resolved" 'govulncheck@v1\.9\.98' Taskfile.yml \
  bash -c 'printf "\n  kollect-two-pins:\n    cmds:\n      - go install golang.org/x/vuln/cmd/govulncheck@v1.9.0 && go run golang.org/x/vuln/cmd/govulncheck@v1.9.98 ./...\n" >> Taskfile.yml'

mutant_rejected "pin after a flag with a separate value" \
  "golang.org/x/vuln/cmd/govulncheck@v1.9.97 cannot be resolved" 'govulncheck@v1\.9\.97' .github/workflows/ci.yaml \
  bash -c 'printf "\n# kollect fixture: go run -tags integration golang.org/x/vuln/cmd/govulncheck@v1.9.97 ./...\n" >> .github/workflows/ci.yaml'

mutant_rejected "literal pin in a Makefile recipe" \
  "Makefile: golang.org/x/vuln/cmd/govulncheck@v1.9.96 cannot be resolved" 'govulncheck@v1\.9\.96' Makefile \
  bash -c 'printf "\n.PHONY: kollect-fixture\nkollect-fixture:\n\tgo install golang.org/x/vuln/cmd/govulncheck@v1.9.96\n" >> Makefile'

mutant_rejected "pin continued onto the next line" \
  "golang.org/x/vuln/cmd/govulncheck@v1.9.95 cannot be resolved" 'govulncheck@v1\.9\.95' Makefile \
  bash -c 'printf "\n.PHONY: kollect-fixture\nkollect-fixture:\n\tgo install \\\\\n\t  golang.org/x/vuln/cmd/govulncheck@v1.9.95\n" >> Makefile'

mutant_rejected "quoted pin version" \
  "golang.org/x/vuln/cmd/govulncheck@v1.9.94 cannot be resolved" 'govulncheck@"v1\.9\.94"' Taskfile.yml \
  bash -c "printf '\n  kollect-quoted:\n    cmds:\n      - go run golang.org/x/vuln/cmd/govulncheck@\"v1.9.94\" ./...\n' >> Taskfile.yml"

# A golangci-lint pinned outside the Makefile is built and smoke-run at ITS version (go install,
# --no-config), not replaced by the Makefile's custom build: the sentinel is v2.13.1's own failure
# (the Makefile's v2.14.0 passes, so only the Taskfile pin can produce it).
mutant_rejected "golangci-lint pinned outside the Makefile is too old" \
  "export data version 5 is greater than maximum supported version 4" 'golangci-lint@v2\.13\.1' Taskfile.yml \
  bash -c 'printf "\n  kollect-old-gcl:\n    cmds:\n      - go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1 run\n" >> Taskfile.yml'

echo "All tool-pin consistency self-tests passed."
