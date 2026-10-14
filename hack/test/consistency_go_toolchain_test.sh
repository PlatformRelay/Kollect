#!/usr/bin/env bash
# CFC-1 (openspec change cross-file-consistency-gates): the shipped binary is built with the
# tested toolchain.
#
# Tests, govulncheck and every CI Go setup run the Go that go.mod's `go` directive names
# (actions/setup-go reads it through `go-version-file: go.mod`). The release image is built by
# the `FROM golang:<version>` line of each Dockerfile, and contributors use the golang image of
# .devcontainer/devcontainer.json. Nothing made those agree: Renovate bumps the golang image
# and, by default, never the `go` directive, so the image ran ahead and the binary that shipped
# was built by a Go no test or vulnerability scan had seen. This gate fails when they differ.
#
# What it locks:
#   * every golang image (Dockerfile* FROM lines, devcontainer image) equals go.mod's `go`;
#   * the images agree with each other;
#   * go.mod has a `go` directive and no `toolchain` line (a toolchain line is a second source
#     of truth that setup-go and the images would not follow);
#   * no workflow or composite action carries a `go-version:` literal, and every
#     actions/setup-go step reads `go-version-file: go.mod`;
#   * renovate.json moves the `go` directive and the golang image in ONE group: the gomod
#     manager is told to bump the directive (rangeStrategy bump, depType golang -- it does not
#     by default), and the group rule comes AFTER the "go minor and patch dependencies" and
#     "docker images" rules, because a later packageRule overrides an earlier one's groupName.
#     Verified with a local Renovate lookup dry-run on 2026-10-14 (evidence/G1.md);
#   * an empty scan (no golang image found) is a failure, never a pass.
#
# Usage:
#   hack/test/consistency_go_toolchain_test.sh              # check the repository tree
#   hack/test/consistency_go_toolchain_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/consistency_go_toolchain_test.sh <tree-root>  # check an arbitrary tree root
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

command -v jq >/dev/null 2>&1 || fail "jq not found; install jq to run this test"

# Every Dockerfile* under the root, relative paths, sorted. Vendored, generated and
# local-only trees are pruned; a Dockerfile with no golang FROM line is simply not a Go build.
dockerfiles() {
  local root="$1"
  (cd "${root}" && find . \( -name .git -o -name node_modules -o -name references -o -name bin \
    -o -name .task -o -name dist -o -name vendor -o -name worktrees \) -prune -o \
    -type f -name 'Dockerfile*' -print | sed 's|^\./||' | sort)
}

# "<relpath> <tag>" for each golang image reference: Dockerfile FROM lines and the devcontainer
# `image`. The tag is everything after `golang:` up to a digest or whitespace; an untagged or
# digest-only reference yields the tag "<untagged>", which the check rejects as not comparable.
golang_images() {
  local root="$1" f
  while IFS= read -r f; do
    [[ -n "${f}" ]] || continue
    # Backslash continuations are joined first, so `FROM --platform=... \` + `golang:...` on
    # the next line is one instruction.
    awk -v f="${f}" '
      {
        line = (cont != "") ? cont " " $0 : $0
        cont = ""
        if (line ~ /\\[[:space:]]*$/) { sub(/\\[[:space:]]*$/, "", line); cont = line; next }
        if (line ~ /^[[:space:]]*#/) next
        sub(/^[[:space:]]+/, "", line)
        n = split(line, w, /[[:space:]]+/)
        if (toupper(w[1]) != "FROM") next
        for (i = 2; i <= n; i++) {
          if (w[i] ~ /^--/) continue
          img = w[i]
          sub(/^(docker\.io\/)?(library\/)?/, "", img)
          if (img ~ /^golang:/) { sub(/^golang:/, "", img); sub(/@.*$/, "", img); print f, img }
          else if (img ~ /^golang(@|$)/) { print f, "<untagged>" }
          break
        }
      }
    ' "${root}/${f}"
  done < <(dockerfiles "${root}")

  f=".devcontainer/devcontainer.json"
  if [[ -f "${root}/${f}" ]]; then
    # devcontainer.json is JSONC: `//` comment lines are dropped, and the key must open a line
    # or follow `{` / `,`, so compact (one-line) JSON is read and a commented-out image is not.
    # /* */ block comments are not handled.
    { grep -vE '^[[:space:]]*//' "${root}/${f}" |
      grep -oE '(^|[{,])[[:space:]]*"image"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } |
      sed -E 's/^[{,]?[[:space:]]*"image"[[:space:]]*:[[:space:]]*"//; s/"$//; s#^(docker\.io/)?(library/)?##' |
      while IFS= read -r img; do
        case "${img}" in
          golang:*) img="${img#golang:}"; echo "${f} ${img%%@*}" ;;
          golang | golang@*) echo "${f} <untagged>" ;;
        esac
      done
  fi
}

# Every CI YAML that can set up Go: workflows and composite actions.
ci_files() {
  local root="$1" dir
  for dir in "${root}/.github/workflows" "${root}/.github/actions"; do
    [[ -d "${dir}" ]] || continue
    find "${dir}" -type f \( -name '*.yml' -o -name '*.yaml' \)
  done | sort
}

# "<file>:<line>" of every actions/setup-go step whose with: block has no
# `go-version-file: go.mod` before the next step starts.
setup_go_without_gomod() {
  local root="$1" file
  while IFS= read -r file; do
    awk -v f="${file#"${root}"/}" '
      function close_step() { if (open && !ok) print f ":" at; open = 0 }
      /^[[:space:]]*#/ { next }
      /^[[:space:]]*-[[:space:]]/ { close_step() }
      /["\047]?uses["\047]?:[[:space:]]*["\047]?actions\/setup-go@/ { open = 1; ok = 0; at = NR; next }
      open && /^[[:space:]]*["\047]?go-version-file["\047]?:[[:space:]]*["\047]?go\.mod["\047]?[[:space:]]*(#.*)?$/ { ok = 1 }
      END { close_step() }
    ' "${file}"
  done < <(ci_files "${root}")
}

count_setup_go() {
  local root="$1" file n=0 c
  while IFS= read -r file; do
    c="$(grep -Ec "^[^#]*[\"']?uses[\"']?:[[:space:]]*[\"']?actions/setup-go@" "${file}" || true)"
    n=$((n + c))
  done < <(ci_files "${root}")
  echo "${n}"
}

# The whole contract for one tree. Prints one FAIL line per broken invariant, returns non-zero
# when any is broken.
check_go_toolchain() {
  local root="$1" broken=0
  local gomod="${root}/go.mod" renovate="${root}/renovate.json"
  local go_version="" images path tag ver distinct lowest line file n

  # --- go.mod -------------------------------------------------------------------------------
  if [[ -f "${gomod}" ]]; then
    go_version="$(sed -n -E 's/^go[[:space:]]+([0-9][0-9.]*)[[:space:]]*$/\1/p' "${gomod}" | head -1)"
  fi
  if [[ -z "${go_version}" ]]; then
    echo "FAIL: go.mod has no 'go <version>' directive -- there is nothing for the images to agree with" >&2
    broken=1
  fi
  if [[ -f "${gomod}" ]] && grep -Eq '^[[:space:]]*toolchain[[:space:]]' "${gomod}"; then
    echo "FAIL: go.mod carries a toolchain line ($(grep -E '^[[:space:]]*toolchain[[:space:]]' "${gomod}" | head -1)) -- a second Go version that setup-go and the images do not follow; keep the go directive as the one source" >&2
    broken=1
  fi

  # --- golang images ------------------------------------------------------------------------
  images="$(golang_images "${root}")"
  if [[ -z "${images}" ]]; then
    echo "FAIL: no golang image found in any Dockerfile* FROM line or in .devcontainer/devcontainer.json -- the scan set is empty, so the comparison would pass without comparing anything" >&2
    broken=1
  else
    while read -r path tag; do
      ver="${tag%%-*}"
      if [[ ! "${ver}" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
        echo "FAIL: ${path}: golang:${tag} is not a literal Go version, so it cannot be compared with go.mod" >&2
        broken=1
        continue
      fi
      [[ -n "${go_version}" ]] || continue
      [[ "${ver}" == "${go_version}" ]] && continue
      lowest="$(printf '%s\n%s\n' "${ver}" "${go_version}" | sort -V | head -1)"
      if [[ "${lowest}" == "${go_version}" ]]; then
        echo "FAIL: ${path}: golang:${ver} is newer than go.mod's go ${go_version} -- the binary would ship built by a Go that tests and govulncheck never ran" >&2
      else
        echo "FAIL: ${path}: golang:${ver} is older than go.mod's go ${go_version} -- the binary would ship built by a Go that tests and govulncheck never ran" >&2
      fi
      broken=1
    done <<<"${images}"

    if [[ -f "${root}/.devcontainer/devcontainer.json" ]] &&
      ! grep -q '^\.devcontainer/devcontainer\.json ' <<<"${images}"; then
      echo "FAIL: .devcontainer/devcontainer.json has no golang image -- CFC-1 compares it with go.mod; if the devcontainer stopped using the golang image on purpose, change this gate with it" >&2
      broken=1
    fi

    distinct="$(awk '{ v = $2; sub(/-.*/, "", v); print v }' <<<"${images}" | sort -u -V | tr '\n' ' ')"
    distinct="${distinct% }"
    if [[ "${distinct}" == *" "* ]]; then
      echo "FAIL: the golang images disagree with each other: ${distinct}" >&2
      broken=1
    fi
  fi

  # --- CI Go setups -------------------------------------------------------------------------
  while IFS= read -r file; do
    while IFS= read -r line; do
      [[ -n "${line}" ]] || continue
      echo "FAIL: ${file#"${root}"/}: go-version: literal ($(sed -E 's/^[[:space:]]+//' <<<"${line#*:}")) -- CI must read the Go version from go.mod via go-version-file" >&2
      broken=1
    done < <(grep -nE "^[[:space:]]*-?[[:space:]]*[\"']?go-version[\"']?:" "${file}" || true)
  done < <(ci_files "${root}")

  n="$(count_setup_go "${root}")"
  if [[ "${n}" -eq 0 ]]; then
    echo "FAIL: no actions/setup-go step found under .github -- the CI half of this check would pass without checking anything" >&2
    broken=1
  fi
  while IFS= read -r line; do
    [[ -n "${line}" ]] || continue
    echo "FAIL: ${line}: actions/setup-go step does not read go-version-file: go.mod" >&2
    broken=1
  done < <(setup_go_without_gomod "${root}")

  # --- Renovate -----------------------------------------------------------------------------
  if [[ ! -f "${renovate}" ]] || ! jq -e '.packageRules | type == "array"' "${renovate}" >/dev/null 2>&1; then
    echo "FAIL: renovate.json is missing, unparsable or has no packageRules -- nothing moves go.mod and the golang image together" >&2
    return 1
  fi

  local group_idx gomod_idx docker_idx
  group_idx="$(jq '[.packageRules | to_entries[]
      | select((.value.matchDepNames // []) | (index("go") != null and index("golang") != null))
      | select(.value.groupName // "" | length > 0) | .key] | last // -1' "${renovate}")"
  gomod_idx="$(jq '[.packageRules | to_entries[]
      | select(.value.groupName == "go minor and patch dependencies") | .key] | last // -1' "${renovate}")"
  docker_idx="$(jq '[.packageRules | to_entries[]
      | select(.value.groupName == "docker images") | .key] | last // -1' "${renovate}")"

  if [[ "${group_idx}" -lt 0 ]]; then
    echo "FAIL: renovate.json has no packageRule that groups depNames go and golang -- Renovate opens a lone golang image PR and go.mod stays behind" >&2
    broken=1
  else
    if [[ "${gomod_idx}" -gt "${group_idx}" ]]; then
      echo "FAIL: renovate.json's go/golang group rule (packageRules[${group_idx}]) comes before the \"go minor and patch dependencies\" rule (packageRules[${gomod_idx}]); the later rule wins and pulls the go directive into its own group" >&2
      broken=1
    fi
    if [[ "${docker_idx}" -gt "${group_idx}" ]]; then
      echo "FAIL: renovate.json's go/golang group rule (packageRules[${group_idx}]) comes before the \"docker images\" rule (packageRules[${docker_idx}]); the later rule wins and pulls the golang image into its own group" >&2
      broken=1
    fi
  fi

  # Any later rule that sets a different groupName may match go or golang (a catch-all, a
  # matchPackageNames or matchDatasources filter, a custom.regex dep named golang) and would win.
  # Deciding which can match means re-implementing Renovate's matchers, so the group rule must
  # simply be the LAST rule that sets a groupName.
  if [[ "${group_idx}" -ge 0 ]]; then
    local later
    later="$(jq -r --argjson g "${group_idx}" '.packageRules[$g].groupName as $name
      | [.packageRules | to_entries[] | select(.key > $g)
        | select((.value.groupName // $name) != $name)
        | "packageRules[\(.key)] (\(.value.groupName))"] | join(", ")' "${renovate}")"
    if [[ -n "${later}" ]]; then
      echo "FAIL: renovate.json's go/golang group rule (packageRules[${group_idx}]) is overridden by a later rule that sets another groupName: ${later} -- keep the go toolchain group the last group rule" >&2
      broken=1
    fi
  fi

  if ! jq -e '[.packageRules[]
      | select((.matchDepTypes // []) | index("golang") != null)
      | select((.matchDepNames // ["go"]) | index("go") != null)
      | select((.matchPackageNames // ["go"]) | index("go") != null)
      | select(.rangeStrategy == "bump")
      | select((.matchManagers // ["gomod"]) | index("gomod") != null)
      | select((.matchFileNames // ["go.mod"]) | index("go.mod") != null)] | length > 0' \
    "${renovate}" >/dev/null; then
    echo "FAIL: renovate.json has no packageRule that sets rangeStrategy bump for the gomod go directive (matchDepTypes golang) -- Renovate does not bump the go directive by default, so the group would carry only the image" >&2
    broken=1
  fi

  return "${broken}"
}

if [[ "${MODE}" == "check" ]]; then
  if check_go_toolchain "${ROOT}"; then
    pass "go.mod, every golang image, the CI Go setups and the Renovate go group agree"
    exit 0
  fi
  fail "the Go toolchain is stated inconsistently (listed above) -- the binary that ships would be built by a Go the tests never ran"
fi

# ---------------------------------------------------------------------------
# Self-test (CFC-6): copy the files the gate reads, break one invariant per copy, and assert
# the gate reds WITH THE MESSAGE of the assertion the mutant targets -- a non-zero exit alone
# is not evidence. One unmutated copy must pass.
# ---------------------------------------------------------------------------
SCRATCH=""
trap 'rm -rf ${SCRATCH} 2>/dev/null' EXIT

make_copy() {
  local dst="$1" f
  mkdir -p "${dst}/.github"
  cp "${ROOT}/go.mod" "${ROOT}/renovate.json" "${dst}/"
  for f in "${ROOT}"/Dockerfile*; do
    [[ -f "${f}" ]] && cp "${f}" "${dst}/"
  done
  [[ -d "${ROOT}/.devcontainer" ]] && cp -R "${ROOT}/.devcontainer" "${dst}/.devcontainer"
  cp -R "${ROOT}/.github/workflows" "${dst}/.github/workflows"
  [[ -d "${ROOT}/.github/actions" ]] && cp -R "${ROOT}/.github/actions" "${dst}/.github/actions"
  return 0
}

new_copy() {
  local copy
  copy="$(mktemp -d)"
  SCRATCH="${SCRATCH} ${copy}"
  make_copy "${copy}"
  echo "${copy}"
}

# mutant_rejected <label> <sentinel> <probe-regex> <probe-file> <command...>
# The command runs inside the copy; the probe proves the mutation landed before the gate runs.
mutant_rejected() {
  local label="$1" sentinel="$2" probe="$3" probe_file="$4"
  shift 4
  local copy out rc
  copy="$(new_copy)"
  (cd "${copy}" && "$@") ||
    fail "self-test: the mutation step for '${label}' failed -- nothing was tested"
  grep -Eq -- "${probe}" "${copy}/${probe_file}" ||
    fail "self-test: the '${label}' mutant did not land (no '${probe}' in ${probe_file}) -- nothing was tested"
  out="$(bash "${SELF}" "${copy}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Fq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected '${sentinel}', got: $(head -3 <<<"${out}" | tr '\n' ' ')"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

# No-op control: an unmutated copy passes. Also proves the gate reads the root it is given.
CONTROL="$(new_copy)"
if ! out="$(bash "${SELF}" "${CONTROL}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree (or the gate) first: $(head -3 <<<"${out}" | tr '\n' ' ')"
fi
pass "self-test: no-op control passes on an unmutated copy"

# Each mutant uses a version no real file carries, so its sentinel cannot be satisfied by some
# other, unrelated drift in the copy.
mutant_rejected "image newer than go.mod" \
  "Dockerfile: golang:1.99.0 is newer than go.mod's go" 'golang:1\.99\.0' Dockerfile \
  sed -i -E 's/golang:[0-9][0-9.]*/golang:1.99.0/' Dockerfile

mutant_rejected "image older than go.mod" \
  "Dockerfile.pipeline: golang:1.1.0 is older than go.mod's go" 'golang:1\.1\.0' Dockerfile.pipeline \
  sed -i -E 's/golang:[0-9][0-9.]*/golang:1.1.0/' Dockerfile.pipeline

mutant_rejected "devcontainer image differs from go.mod" \
  ".devcontainer/devcontainer.json: golang:1.98.0 is newer than go.mod's go" 'golang:1\.98\.0' \
  .devcontainer/devcontainer.json \
  sed -i -E 's/golang:[0-9][0-9.]*/golang:1.98.0/' .devcontainer/devcontainer.json

mutant_rejected "go.mod behind the images" \
  "go.mod's go 1.0.1" '^go 1\.0\.1$' go.mod \
  sed -i -E 's/^go [0-9][0-9.]*$/go 1.0.1/' go.mod

mutant_rejected "images disagree with each other" \
  "the golang images disagree with each other" 'golang:1\.97\.0' Dockerfile.pipeline \
  bash -c "sed -i -E 's/golang:[0-9][0-9.]*/golang:1.96.0/' Dockerfile && sed -i -E 's/golang:[0-9][0-9.]*/golang:1.97.0/' Dockerfile.pipeline"

mutant_rejected "toolchain line in go.mod" \
  "go.mod carries a toolchain line" '^toolchain go' go.mod \
  bash -c 'printf "\ntoolchain go1.99.0\n" >> go.mod'

mutant_rejected "no go directive in go.mod" \
  "go.mod has no 'go <version>' directive" '^module ' go.mod \
  sed -i -E '/^go [0-9]/d' go.mod

mutant_rejected "go-version literal in a CI Go setup" \
  "go-version: literal" 'go-version: "1\.99"' .github/actions/go-cache/action.yml \
  sed -i -E 's/go-version-file: go\.mod/go-version: "1.99"/' .github/actions/go-cache/action.yml

mutant_rejected "setup-go not reading go.mod" \
  "does not read go-version-file: go.mod" 'go-version-file: \.go-version' \
  .github/actions/go-cache/action.yml \
  sed -i -E 's/go-version-file: go\.mod/go-version-file: .go-version/' .github/actions/go-cache/action.yml

mutant_rejected "quoted setup-go not reading go.mod" \
  "does not read go-version-file: go.mod" 'uses: "actions/setup-go@' .github/actions/go-cache/action.yml \
  sed -i -E 's#uses: actions/setup-go@([0-9a-f]+)#uses: "actions/setup-go@\1"#; s/go-version-file: go\.mod/go-version-file: .go-version/' \
  .github/actions/go-cache/action.yml

mutant_rejected "devcontainer image differs, compact one-line JSON" \
  ".devcontainer/devcontainer.json: golang:1.94.0 is newer than go.mod's go" '^\{"image":"golang:1\.94\.0' \
  .devcontainer/devcontainer.json \
  bash -c 'printf "{\"image\":\"golang:1.94.0@sha256:%064d\",\"name\":\"x\"}\n" 0 > .devcontainer/devcontainer.json'

mutant_rejected "devcontainer without a golang image" \
  ".devcontainer/devcontainer.json has no golang image" '"image": "debian:' .devcontainer/devcontainer.json \
  sed -i -E 's/"image": "golang:/"image": "debian:/' .devcontainer/devcontainer.json

mutant_rejected "untagged golang image in a Dockerfile" \
  "Dockerfile: golang:<untagged> is not a literal Go version" '^FROM golang AS builder' Dockerfile \
  sed -i -E 's/^FROM golang:[^ ]+ AS builder/FROM golang AS builder/' Dockerfile

mutant_rejected "digest-only golang image in a Dockerfile" \
  "Dockerfile.pipeline: golang:<untagged> is not a literal Go version" '^FROM golang@sha256:' Dockerfile.pipeline \
  sed -i -E 's/^FROM golang:[^@ ]+@/FROM golang@/' Dockerfile.pipeline

mutant_rejected "nested Dockerfile drifts" \
  "build/nested/Dockerfile: golang:1.93.0 is newer than go.mod's go" '^FROM golang:1\.93\.0' build/nested/Dockerfile \
  bash -c 'mkdir -p build/nested && printf "FROM golang:1.93.0 AS b\n" > build/nested/Dockerfile'

mutant_rejected "no golang image found" \
  "no golang image found" 'debian:1' Dockerfile \
  sed -i -E 's/golang:/debian:/' Dockerfile Dockerfile.pipeline .devcontainer/devcontainer.json

mutant_rejected "FROM continued onto the next line drifts" \
  "Dockerfile: golang:1.92.0 is newer than go.mod's go" '^  golang:1\.92\.0' Dockerfile \
  sed -i -E 's/^FROM golang:[^ ]+ AS builder/FROM --platform=linux\/amd64 \\\n  golang:1.92.0 AS builder/' Dockerfile

# shellcheck disable=SC2016 # ${GO_VERSION} is literal Dockerfile text, not a shell expansion
mutant_rejected "golang tag from a build arg" \
  "Dockerfile: golang:\${GO_VERSION} is not a literal Go version" 'golang:\$\{GO_VERSION\}' Dockerfile \
  sed -i -E 's/^FROM golang:[^ ]+ AS builder/FROM golang:${GO_VERSION} AS builder/' Dockerfile

mutant_rejected "quoted YAML keys on a setup-go step not reading go.mod" \
  "does not read go-version-file: go.mod" '"uses": actions/setup-go@' .github/actions/go-cache/action.yml \
  sed -i -E 's#uses: actions/setup-go@#"uses": actions/setup-go@#; s/go-version-file: go\.mod/"go-version-file": .go-version/' \
  .github/actions/go-cache/action.yml

mutant_rejected "quoted go-version key literal" \
  "go-version: literal" '"go-version": "1\.99"' .github/actions/go-cache/action.yml \
  sed -i -E 's/go-version-file: go\.mod/go-version-file: go.mod\n        "go-version": "1.99"/' .github/actions/go-cache/action.yml

# Green controls: legitimate spellings the gate must accept.
control_passes() {
  local label="$1" probe="$2" probe_file="$3"
  shift 3
  local copy out
  copy="$(new_copy)"
  (cd "${copy}" && "$@") || fail "self-test: the control step for '${label}' failed -- nothing was tested"
  grep -Eq -- "${probe}" "${copy}/${probe_file}" ||
    fail "self-test: the '${label}' control did not land (no '${probe}' in ${probe_file})"
  out="$(bash "${SELF}" "${copy}" 2>&1)" ||
    fail "self-test: the gate red on the '${label}' control -- a legitimate spelling must pass: $(head -2 <<<"${out}" | tr '\n' ' ')"
  pass "self-test: ${label} control passes"
}

control_passes "commented-out devcontainer image (JSONC)" '^  // "image": "golang:1\.0\.0"' \
  .devcontainer/devcontainer.json \
  sed -i -E '2i\  // "image": "golang:1.0.0",' .devcontainer/devcontainer.json

# Renovate mutants rewrite renovate.json with one jq program and fail when the rewrite changed
# nothing, so a selector that stopped matching cannot pass as a tested mutant. The programs use
# jq variables ($g, $d), hence single quotes.
GO_GROUP='((.matchDepNames // []) | (index("go") != null and index("golang") != null))'
renovate_jq() {
  jq "$1" renovate.json >r.tmp
  if cmp -s r.tmp renovate.json; then
    echo "jq program left renovate.json unchanged: $1" >&2
    return 1
  fi
  mv r.tmp renovate.json
}

mutant_rejected "Renovate go group rule missing" \
  "renovate.json has no packageRule that groups depNames go and golang" '"packageRules"' renovate.json \
  renovate_jq "del(.packageRules[] | select(${GO_GROUP}))"

# shellcheck disable=SC2016 # $g is a jq variable
mutant_rejected "Renovate go group rule mis-ordered" \
  "comes before the \"go minor and patch dependencies\" rule" '"packageRules"' renovate.json \
  renovate_jq '.packageRules |= ([.[] | select('"${GO_GROUP}"')] as $g | $g + [.[] | select('"${GO_GROUP}"' | not)])'

# shellcheck disable=SC2016 # $g and $d are jq variables
mutant_rejected "Renovate go group rule before the docker images rule" \
  "comes before the \"docker images\" rule" '"packageRules"' renovate.json \
  renovate_jq '.packageRules |= ([.[] | select('"${GO_GROUP}"')] as $g | [.[] | select('"${GO_GROUP}"' | not)] | (map(.groupName == "docker images") | index(true)) as $d | .[:$d] + $g + .[$d:])'

mutant_rejected "Renovate go directive bump missing" \
  "no packageRule that sets rangeStrategy bump for the gomod go directive" '"packageRules"' renovate.json \
  renovate_jq 'del(.packageRules[] | select((.matchDepTypes // []) | index("golang") != null))'

mutant_rejected "Renovate go directive bump names another dep" \
  "no packageRule that sets rangeStrategy bump for the gomod go directive" '"not-go"' renovate.json \
  renovate_jq '(.packageRules[] | select((.matchDepTypes // []) | index("golang") != null) | .matchDepNames) |= ["not-go"]'

mutant_rejected "Renovate later rule overrides the go group" \
  "is overridden by a later rule that sets another groupName" '"late override"' renovate.json \
  renovate_jq '.packageRules += [{"matchManagers": ["dockerfile"], "groupName": "late override"}]'

mutant_rejected "Renovate later catch-all group rule" \
  "is overridden by a later rule that sets another groupName" '"catch all"' renovate.json \
  renovate_jq '.packageRules += [{"groupName": "catch all"}]'

mutant_rejected "Renovate later matchPackageNames golang rule" \
  "is overridden by a later rule that sets another groupName" '"images by name"' renovate.json \
  renovate_jq '.packageRules += [{"matchPackageNames": ["golang"], "groupName": "images by name"}]'

mutant_rejected "Renovate go directive bump filtered to another package" \
  "no packageRule that sets rangeStrategy bump for the gomod go directive" '"not-go-pkg"' renovate.json \
  renovate_jq '(.packageRules[] | select((.matchDepTypes // []) | index("golang") != null)) += {"matchPackageNames": ["not-go-pkg"]}'

echo "All Go toolchain consistency self-tests passed."
