#!/usr/bin/env bash
# DTP-1 to DTP-7 (openspec change developer-toolchain-pins): the
# developer and CI tools that are pinned in more than one file agree, meet their floor, and can be
# installed, and the update bot (Renovate) finds and groups every pin.
#
# gitleaks, git-cliff and shellcheck are each pinned in three files, and nothing kept the three
# equal: a bot bump or a hand edit moves one site and the others silently run another release.
# What it locks:
#   * gitleaks: every `GITLEAKS_VERSION:` under .github/, every `${GITLEAKS_VERSION:-<v>}` default
#     in hack/ (not hack/test) and the gitleaks `rev:` in .pre-commit-config.yaml agree and are at
#     least GITLEAKS_FLOOR; ci.yaml, hack/install-gitleaks.sh and .pre-commit-config.yaml must each
#     carry one;
#   * git-cliff: Taskfile.yml `GIT_CLIFF_VERSION`, the `with: version:` input of every
#     orhun/git-cliff-action step under .github/ (a step without one installs the latest
#     release; a `version:` under `env:` or another map is not the input) and every literal `install-git-cliff.sh <v>` call agree with Taskfile.yml and are
#     at least GIT_CLIFF_FLOOR; Taskfile.yml, release.yaml and hack/verify-changelog.sh must each
#     carry one;
#   * shellcheck: every `SHELLCHECK_VERSION:` under .github/, every `${SHELLCHECK_VERSION:-<v>}`
#     default in hack/ and the shellcheck-py `rev:` (first three components: the fourth is
#     the packaging revision of the pre-commit wrapper, not a shellcheck release) agree, and
#     hack/install-shellcheck.sh's pinned_sha256 table has an entry for that version on every
#     OS and architecture the installer maps (a bump without digests cannot install);
#   * Task (DTP-1): the `with: version:` input of every go-task/setup-task step under .github/
#     (read like git-cliff's: a step without one installs an unpinned Task and fails) and
#     mise.toml's [tools] task agree and are at least TASK_FLOOR; ci.yaml and the kind-e2e-setup
#     composite action must each carry one; whether mise.toml has a task entry at all is the
#     DEV-MISE-01 check below;
#   * govulncheck (DTP-2): Taskfile.yml defines GOVULNCHECK_VERSION directly under
#     `# renovate: datasource=go depName=golang.org/x/vuln` (the Go module datasource; a
#     github-releases annotation would extract but never look up), and every govulncheck the
#     vulncheck task runs reads {{.GOVULNCHECK_VERSION}}: an inline version, another variable or a
#     task-level GOVULNCHECK_VERSION is a failure. The value is one plain vN.N.N in the exact form
#     renovate.json's Taskfile manager extracts. No floor: CFC-2 (consistency_tool_pins_test.sh)
#     builds and runs the pin;
#   * versions compare with a leading `v` stripped and must be plain releases (N.N.N); a
#     variable, `latest` or an empty value is a failure, never skipped;
#   * an empty scan for any tool fails, never passes;
#   * DTP-6: UNVERIFIED_BY_DESIGN in hack/test/ci_fetch_lib_hardening_test.sh (installers exempt
#     from its checksum scan) is defined once, on one line, and otherwise only read (a closed list
#     of read forms; any other mention is a write), and lists nothing outside
#     UNVERIFIED_LIMIT, which is empty: no installer is exempt from the checksum scan, and
#     --self-test runs each checksum-verifying installer on valid, tampered and missing-checksum
#     downloads;
#   * DEV-MISE-01 (folded in from the former mise pin-drift guard): mise.toml exists and has a [tools]
#     table with a task entry (its value is DTP-1's) and node and python equal to every
#     `node-version:` and `python-version:` under .github/ (which must agree among themselves
#     and must not be empty); Go is not under [tools], idiomatic_version_file_enable_tools
#     names "go" and go.mod has a `go` line, so mise reads Go from go.mod as actions/setup-go does;
#   * DTP-5 comments: a comment in mise.toml or Taskfile.yml may state a tool version only in the
#     unbroken run of comment lines directly above a pin, or inline on it, naming that pin's tool
#     at the pin's value; any other version (a dotted or quoted number after a tool name) and any
#     count of pin sites ("17 occurrences") is a failure;
#   * DTP-4 Renovate: every customManagers entry in renovate.json matches a tracked file with a
#     file pattern AND extracts a dependency from it (a pattern hit alone is a dead manager), no
#     file pattern matches nothing, every gitleaks, git-cliff, shellcheck, Task, govulncheck and
#     golangci-lint site above is extracted at its line and value under that tool's depName (the
#     golangci-lint pins must be plain vN.N.N), each tool's sites resolve to one groupName (the
#     last matching packageRule wins), and shellcheck's group holds nothing else (its bump is red
#     until its digests are added). python3 evaluates renovate.json itself; shapes it does not
#     model (RE2-foreign regex syntax included) fail.
# Lines whose first non-blank character is `#` are comments and never count as a site.
#
# --self-test (DTP-7) copies the tracked tree once per case, breaks one clause, and asserts the
# gate reds with that clause's message; one unmutated copy and the four-component shellcheck-py
# rev must pass; removing this guard's lint steps must red hack/test/ci_workflow_security_test.sh
# (CWS-6 for one step, CFC-6 for both).
#
# Usage:
#   hack/test/consistency_toolchain_pins_test.sh              # check the repository tree
#   hack/test/consistency_toolchain_pins_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/consistency_toolchain_pins_test.sh <tree-root>  # check another tree root
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

GITLEAKS_FLOOR="8.30.1"
GIT_CLIFF_FLOOR="2.13.1"
TASK_FLOOR="3.52.0"
# The exemptions UNVERIFIED_BY_DESIGN may hold: none since DTP-6 (task 4.3) emptied the list.
# Shrink-only, so it stays empty.
UNVERIFIED_LIMIT=()
FETCH_GATE="hack/test/ci_fetch_lib_hardening_test.sh"

WORKROOT="$(mktemp -d)"
cleanup() {
  rm -rf "${WORKROOT:?}" 2>/dev/null || true
}
trap cleanup EXIT

BROKEN=0
err() {
  echo "FAIL: $*" >&2
  BROKEN=1
}

# yaml_files <root>: workflow and composite-action files, relative to root.
yaml_files() {
  (cd "$1" && [[ -d .github ]] && find .github -type f \( -name '*.yml' -o -name '*.yaml' \) | sed 's|^\./||' | sort) || true
}

# hack_scripts <root>: shell scripts under hack/, guards (hack/test) excluded: they carry
# pin-shaped fixtures.
hack_scripts() {
  (cd "$1" && [[ -d hack ]] && find hack -path hack/test -prune -o -type f -name '*.sh' -print | sort) || true
}

# Every site line below is "<file>:<line>\t<value as written>\t<value compared>".

# yaml_env_sites <root> <VAR>: `<VAR>: <value>` keys in .github YAML.
yaml_env_sites() {
  local root="$1" var="$2" f
  while IFS= read -r f; do
    awk -v f="${f}" -v var="${var}" '
      $0 ~ "^[[:space:]]*" var ":" {
        v = $0
        sub("^[[:space:]]*" var ":[[:space:]]*", "", v)
        sub(/[[:space:]]+#.*$/, "", v)
        gsub(/^["\047]|["\047][[:space:]]*$/, "", v)
        print f ":" NR "\t" v
      }' "${root}/${f}"
  done < <(yaml_files "${root}")
}

# default_sites <root> <VAR>: `${<VAR>:-<value>}` defaults in hack/ scripts, comments excluded.
default_sites() {
  local root="$1" var="$2" f
  while IFS= read -r f; do
    awk -v f="${f}" -v var="${var}" '
      /^[[:space:]]*#/ { next }
      {
        line = $0
        key = "${" var ":-"
        while ((i = index(line, key)) > 0) {
          rest = substr(line, i + length(key))
          j = index(rest, "}")
          v = (j > 0) ? substr(rest, 1, j - 1) : rest
          print f ":" NR "\t" v
          line = rest
        }
      }' "${root}/${f}"
  done < <(hack_scripts "${root}")
}

# precommit_rev_sites <root> <repo-ERE>: the `rev:` of every .pre-commit-config.yaml repo whose
# URL matches.
precommit_rev_sites() {
  local root="$1" repo="$2" f=".pre-commit-config.yaml"
  [[ -f "${root}/${f}" ]] || return 0
  awk -v f="${f}" -v repo="${repo}" '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*-[[:space:]]*repo:/ {
      cur = $0
      sub(/^[^:]*:[[:space:]]*/, "", cur)
      sub(/[[:space:]]+#.*$/, "", cur)
      gsub(/^["\047]|["\047][[:space:]]*$/, "", cur)
      next
    }
    /^[[:space:]]*rev:/ && cur ~ repo {
      v = $0
      sub(/^[[:space:]]*rev:[[:space:]]*/, "", v)
      sub(/[[:space:]]+#.*$/, "", v)
      gsub(/^["\047]|["\047][[:space:]]*$/, "", v)
      print f ":" NR "\t" v
    }' "${root}/${f}"
}

# action_version_sites <root> <action>: the `with: version:` input of every step that uses
# <action> (`owner/name`), or "<no version: input>" when the step has none (the action then
# installs whatever it defaults to). A step is read whole (key order does not matter): it opens at
# a `- ` line and ends at the next `- ` line at its indent or a line left of it. Only a direct
# child of the step's `with:` is the input; a `version:` under `env:` or another map is not, and
# must not stand in for it.
action_version_sites() {
  local root="$1" action="$2" f
  while IFS= read -r f; do
    awk -v f="${f}" -v action="${action}" '
      function indent(s) { match(s, /^[[:space:]]*/); return RLENGTH }
      function flush() {
        if (open && hit) print f ":" (ver == "" ? at "\t<no version: input>" : vat "\t" ver)
        open = 0; hit = 0; ver = ""; w = -1; wc = -1
      }
      /^[[:space:]]*#/ { next }
      !/[^[:space:]]/ { next }
      {
        ind = indent($0)
        isdash = ($0 ~ /^[[:space:]]*-[[:space:]]/)
        if (open && (ind < step || (ind == step && isdash))) flush()
        if (!open && isdash) {
          open = 1; step = ind; at = NR; w = -1; wc = -1
          match($0, /^[[:space:]]*-[[:space:]]+/); kl = RLENGTH
        }
        if (!open) next
        lvl = (isdash && ind == step) ? kl : ind
        k = $0
        sub(/^[[:space:]]*(-[[:space:]]+)?/, "", k)
        sub(/[[:space:]]+#.*$/, "", k)
        if (lvl == kl) {
          w = -1
          if (index(k, "uses:") == 1) {
            u = substr(k, 6)
            gsub(/^[[:space:]]*["\047]?/, "", u)
            if (index(u, action "@") == 1) { hit = 1; at = NR }
          }
          if (k ~ /^with:[[:space:]]*$/) { w = kl; wc = -1 }
          next
        }
        if (w >= 0 && lvl > w) {
          if (wc < 0) wc = lvl
          if (lvl == wc && k ~ /^version:/) {
            v = k
            sub(/^version:[[:space:]]*/, "", v)
            gsub(/^["\047]|["\047][[:space:]]*$/, "", v)
            ver = (v == "") ? "<empty version: input>" : v
            vat = NR
          }
        }
      }
      END { flush() }' "${root}/${f}"
  done < <(yaml_files "${root}")
}

# cliff_install_sites <root>: literal `install-git-cliff.sh <version>` calls in hack/ scripts,
# Makefile, Taskfile.yml and .github YAML. `{{.GIT_CLIFF_VERSION}}` reads the Taskfile site and
# is not a site of its own; any other non-literal argument is reported, never skipped.
cliff_install_sites() {
  local root="$1" f
  while IFS= read -r f; do
    awk -v f="${f}" '
      /^[[:space:]]*#/ { next }
      {
        line = $0
        while (match(line, /install-git-cliff\.sh[[:space:]]+[^[:space:]]+/)) {
          v = substr(line, RSTART, RLENGTH)
          sub(/^install-git-cliff\.sh[[:space:]]+/, "", v)
          gsub(/["\047]/, "", v)
          if (v != "{{.GIT_CLIFF_VERSION}}") print f ":" NR "\t" v
          line = substr(line, RSTART + RLENGTH)
        }
      }' "${root}/${f}"
  done < <(
    hack_scripts "${root}"
    [[ -f "${root}/Makefile" ]] && echo Makefile
    [[ -f "${root}/Taskfile.yml" ]] && echo Taskfile.yml
    yaml_files "${root}"
  )
}

taskfile_sites() {
  local root="$1" var="$2" f="Taskfile.yml"
  [[ -f "${root}/${f}" ]] || return 0
  awk -v f="${f}" -v var="${var}" '
    $0 ~ "^[[:space:]]*" var ":" {
      v = $0
      sub("^[[:space:]]*" var ":[[:space:]]*", "", v)
      sub(/[[:space:]]+#.*$/, "", v)
      gsub(/^["\047]|["\047][[:space:]]*$/, "", v)
      print f ":" NR "\t" v
    }' "${root}/${f}"
}

# with_compare <mode>: appends the compared form. `plain` strips a leading v; `first3` also
# drops a fourth numeric component (shellcheck-py's packaging revision).
with_compare() {
  local mode="$1" loc v c
  while IFS=$'\t' read -r loc v; do
    [[ -n "${loc}" ]] || continue
    c="${v#v}"
    if [[ "${mode}" == "first3" && "${c}" =~ ^([0-9]+\.[0-9]+\.[0-9]+)\.[0-9]+$ ]]; then
      c="${BASH_REMATCH[1]}"
    fi
    printf '%s\t%s\t%s\n' "${loc}" "${v}" "${c}"
  done
}

# version_lt <a> <b>: a < b for validated N.N.N strings.
version_lt() {
  [[ "$1" != "$2" && "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -1)" == "$1" ]]
}

# check_tool <tool> <floor or -> <reference file> <sites> <required file>...
# Sets AGREED to the shared version when every site is plain and equal, else to "".
AGREED=""
check_tool() {
  local tool="$1" floor="$2" ref_file="$3" sites="$4"
  shift 4
  local f loc v c ref_loc="" ref_v="" ref_c="" values="" ok=1
  AGREED=""
  if [[ -z "${sites}" ]]; then
    err "${tool}: the scan found no pin site (looked in $*) -- an empty scan never passes"
    return 0
  fi
  echo "${tool} sites:"
  while IFS=$'\t' read -r loc v c; do echo "  ${loc} ${v}"; done <<<"${sites}"
  for f in "$@"; do
    awk -v f="${f}" 'index($0, f ":") == 1 { found = 1 } END { exit !found }' <<<"${sites}" ||
      { err "${tool}: no pin found in ${f} -- a named site lost its pin or the scan can no longer read it"; ok=0; }
  done
  while IFS=$'\t' read -r loc v c; do
    if [[ ! "${c}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      err "${tool}: ${loc} says '${v}', not a plain release version -- a variable, a range or 'latest' cannot be compared"
      ok=0
      continue
    fi
    if [[ -z "${ref_loc}" && "${loc%%:*}" == "${ref_file}" ]]; then
      ref_loc="${loc}" ref_v="${v}" ref_c="${c}"
    fi
    values="${values} ${c}"
  done <<<"${sites}"
  [[ -n "${values}" ]] || return 0
  if [[ -z "${ref_loc}" ]]; then
    # The reference file has no plain site (reported above); compare against the first plain one.
    IFS=$'\t' read -r ref_loc ref_v ref_c < <(awk -F'\t' '$3 ~ /^[0-9]+\.[0-9]+\.[0-9]+$/' <<<"${sites}" | head -1)
  fi
  while IFS=$'\t' read -r loc v c; do
    [[ "${c}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
    if [[ "${c}" != "${ref_c}" ]]; then
      err "${tool}: ${loc} says ${v}, but the reference site ${ref_loc} says ${ref_v} -- one tool, one release"
      ok=0
    fi
  done <<<"${sites}"
  ((ok == 1)) || return 0
  if [[ "${floor}" != "-" ]] && version_lt "${ref_c}" "${floor}"; then
    err "${tool}: every site says ${ref_c}, below the floor ${floor}"
    return 0
  fi
  AGREED="${ref_c}"
}

# check_shellcheck_digests <root> <version>: pinned_sha256 has a 64-hex digest for every
# OS/architecture pair the installer maps.
check_shellcheck_digests() {
  local root="$1" ver="$2" f="hack/install-shellcheck.sh" oses archs os arch asset
  oses="$(sed -n -E 's/^[[:space:]]*[^#)]+\)[[:space:]]*OS="?([a-z0-9_]+)"?[[:space:]]*;;.*$/\1/p' "${root}/${f}" | sort -u)"
  archs="$(sed -n -E 's/^[[:space:]]*[^#)]+\)[[:space:]]*ARCH="?([a-z0-9_]+)"?[[:space:]]*;;.*$/\1/p' "${root}/${f}" | sort -u)"
  if [[ -z "${oses}" || -z "${archs}" ]]; then
    err "shellcheck: cannot read the supported platforms of ${f} (no OS=/ARCH= case arms) -- the digest check would pass on nothing"
    return 0
  fi
  for os in ${oses}; do
    for arch in ${archs}; do
      asset="shellcheck-v${ver}.${os}.${arch}.tar.xz"
      awk -v want="${asset})" '
        /^[[:space:]]*#/ { next }
        found { if ($0 ~ /^[[:space:]]*echo[[:space:]]+"[0-9a-f]{64}"[[:space:]]*$/) ok = 1; exit }
        { t = $0; gsub(/^[[:space:]]+|[[:space:]]+$/, "", t); if (t == want) found = 1 }
        END { exit !ok }' "${root}/${f}" ||
        err "shellcheck: ${f} pinned_sha256 has no entry for ${asset} -- every site says ${ver}, which cannot install until its digests are added"
    done
  done
}

# unverified_other_uses <file>: line numbers of every non-comment mention of UNVERIFIED_BY_DESIGN
# that is not the definition `UNVERIFIED_BY_DESIGN=( ... )`, a read `${UNVERIFIED_BY_DESIGN[@]}`,
# `${UNVERIFIED_BY_DESIGN[*]}` or `${#UNVERIFIED_BY_DESIGN[@]}`, or the bare word between spaces
# inside a double-quoted string (a message). A closed list: `+=`, `[i]=`, `${...:=}`, mapfile,
# `read -a`, `printf -v`, a nameref and any other form are writes.
unverified_other_uses() {
  awk '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*UNVERIFIED_BY_DESIGN=\(.*\)[[:space:]]*$/ { next }
    {
      line = $0
      gsub(/\$\{#?UNVERIFIED_BY_DESIGN\[[@*]\]\}/, "", line)
      name = "UNVERIFIED_BY_DESIGN"
      done = 0
      while (!done && (i = index(line, name)) > 0) {
        pre = substr(line, 1, i - 1)
        q = gsub(/"/, "\"", pre) - gsub(/\\"/, "&", pre)
        before = substr(line, i - 1, 1)
        after = substr(line, i + length(name), 1)
        if (!(q % 2 == 1 && before == " " && after == " ")) { print NR; done = 1 }
        line = substr(line, i + length(name))
      }
    }' "$1"
}

check_unverified_limit() {
  local root="$1" f="${FETCH_GATE}" defs n others body entry known ok entries=()
  if [[ ! -f "${root}/${f}" ]]; then
    err "DTP-6: ${f} not found -- the exemptions list cannot be read"
    return 0
  fi
  defs="$(grep -E '^[[:space:]]*UNVERIFIED_BY_DESIGN=\(.*\)[[:space:]]*$' "${root}/${f}" || true)"
  others="$(unverified_other_uses "${root}/${f}" | tr '\n' ' ')"
  if [[ -z "${defs}" ]]; then
    err "DTP-6: ${f} has no single-line UNVERIFIED_BY_DESIGN=( ... ) definition -- the exemptions list cannot be read"
    return 0
  fi
  n="$(grep -c . <<<"${defs}" || true)"
  if [[ "${n}" != "1" || -n "${others}" ]]; then
    err "DTP-6: UNVERIFIED_BY_DESIGN is written $((n + $(wc -w <<<"${others}"))) times in ${f} (definitions: ${n}; other writes or unrecognised uses at lines: ${others:-none}) -- one definition and plain reads only, so the list cannot grow behind it"
    return 0
  fi
  [[ "${defs}" =~ ^[[:space:]]*UNVERIFIED_BY_DESIGN=\((.*)\)[[:space:]]*$ ]]
  body="${BASH_REMATCH[1]//\"/}"
  body="${body//\'/}"
  read -r -a entries <<<"${body}"
  for entry in ${entries[@]+"${entries[@]}"}; do
    ok=0
    for known in ${UNVERIFIED_LIMIT[@]+"${UNVERIFIED_LIMIT[@]}"}; do
      [[ "${entry}" == "${known}" ]] && ok=1
    done
    ((ok == 1)) ||
      err "DTP-6: UNVERIFIED_BY_DESIGN lists ${entry}, which is not in this guard's limit (${UNVERIFIED_LIMIT[*]:-empty}) -- the exceptions list only shrinks"
  done
  echo "UNVERIFIED_BY_DESIGN: ${body:-<empty>}"
}

# --- DEV-MISE-01 (folded in from the former mise pin-drift guard, mutant for mutant) ---
# input_versions <root> <key>: every value of a plain workflow input (node-version, python-version).
input_versions() {
  local root="$1" key="$2" f
  while IFS= read -r f; do
    awk -v key="${key}" '
      /^[[:space:]]*#/ { next }
      $0 ~ "^[[:space:]]*" key ":[[:space:]]*" {
        line = $0
        sub("^[[:space:]]*" key ":[[:space:]]*", "", line)
        sub(/[[:space:]]*#.*$/, "", line)
        gsub(/["\047[:space:]]/, "", line)
        if (line != "") print line
      }' "${root}/${f}"
  done < <(yaml_files "${root}")
}

# mise_tool_sites <file> <tool>: "mise.toml:<line>\t<value>" for one key in mise.toml's [tools]
# table; nothing if absent.
mise_tool_sites() {
  awk -v tool="$2" '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*\[/ { section = $0; gsub(/[[:space:]]/, "", section); next }
    section == "[tools]" && $0 ~ "^[[:space:]]*" tool "[[:space:]]*=" {
      line = $0
      sub("^[[:space:]]*" tool "[[:space:]]*=[[:space:]]*", "", line)
      sub(/[[:space:]]*#.*$/, "", line)
      gsub(/["\047[:space:]]/, "", line)
      print "mise.toml:" NR "\t" line
    }' "$1"
}

# mise_tool_value <file> <tool>: the value of one key in mise.toml's [tools] table; empty if absent.
mise_tool_value() {
  mise_tool_sites "$1" "$2" | cut -f2
}

# mise_assert_pin <label> <mise value> <CI values, one per line> <origin>: one mise.toml literal
# against the CI pins it mirrors. An empty CI scan or a missing mise value is a failure.
mise_assert_pin() {
  local label="$1" mise_value="$2" ci_list="$3" origin="$4" distinct
  if [[ -z "${ci_list}" ]]; then
    err "mise: the scan set is empty -- found none of ${origin} under .github, so the ${label} check would pass without comparing anything"
    return 0
  fi
  distinct="$(sort -u <<<"${ci_list}")"
  if [[ "$(grep -c '' <<<"${distinct}")" != "1" ]]; then
    err "mise: CI disagrees with itself on ${label} -- ${origin} yields $(tr '\n' ' ' <<<"${distinct}")"
    return 0
  fi
  if [[ -z "${mise_value}" ]]; then
    err "mise: mise.toml has no ${label} entry under [tools], but CI pins ${label} ${distinct} -- the local toolchain is unpinned while the gates are not"
    return 0
  fi
  [[ "${mise_value}" == "${distinct}" ]] ||
    err "mise: mise.toml pins ${label} = ${mise_value} but ${origin} pins ${distinct} -- contributors would run a different ${label} than the merge gates"
}

# check_mise <root>: mise.toml has a task entry (DTP-1 compares its value) and its node/python equal
# every CI pin; Go stays out of [tools] and is read from go.mod (idiomatic_version_file_enable_tools),
# as actions/setup-go reads it.
check_mise() {
  local root="$1" mise="mise.toml" go_pin
  if [[ ! -f "${root}/${mise}" ]]; then
    err "mise: ${mise} does not exist -- this check exists to keep it honest, so its absence is a failure, not a skip"
    return 0
  fi
  if ! grep -Eq '^[[:space:]]*\[tools\][[:space:]]*$' "${root}/${mise}"; then
    err "mise: ${mise} has no [tools] table -- every pin lookup would return empty and pass vacuously"
    return 0
  fi
  go_pin="$(mise_tool_value "${root}/${mise}" go)"
  [[ -z "${go_pin}" ]] ||
    err "mise: mise.toml pins go = ${go_pin} under [tools]; that value WINS over go.mod and goes stale the next time go.mod is bumped -- remove it and let idiomatic_version_file_enable_tools read go.mod, as actions/setup-go does via go-version-file"
  grep -Eq '^[[:space:]]*idiomatic_version_file_enable_tools[[:space:]]*=.*"go"' "${root}/${mise}" ||
    err "mise: mise.toml no longer enables the idiomatic version file for go -- without idiomatic_version_file_enable_tools = [\"go\"] mise stops reading go.mod and Go silently becomes unmanaged"
  { [[ -f "${root}/go.mod" ]] && grep -Eq '^go[[:space:]]+[0-9]' "${root}/go.mod"; } ||
    err "mise: go.mod has no 'go <version>' line -- the file mise and actions/setup-go both read has nothing to read"
  # Task: DTP-1 compares the value with every go-task/setup-task input; only presence is checked here.
  [[ -n "$(mise_tool_value "${root}/${mise}" task)" ]] ||
    err "mise: mise.toml has no task entry under [tools], but CI pins task with go-task/setup-task -- the local toolchain is unpinned while the gates are not"
  mise_assert_pin node "$(mise_tool_value "${root}/${mise}" node)" \
    "$(input_versions "${root}" node-version)" "the actions/setup-node node-version: inputs"
  mise_assert_pin python "$(mise_tool_value "${root}/${mise}" python)" \
    "$(input_versions "${root}" python-version)" "the actions/setup-python python-version: inputs"
  echo "mise.toml: task $(mise_tool_value "${root}/${mise}" task), node $(mise_tool_value "${root}/${mise}" node), python $(mise_tool_value "${root}/${mise}" python), go from go.mod"
}

# --- DTP-5 comments: a version is restated only beside the pin it names ---
# comment_findings <file> <mise|taskfile>: one line per comment that restates a version away from
# its pin, or counts pin sites. A pin is a mise.toml [tools] key, or a Taskfile.yml `<X>_VERSION:`
# with a literal value. A comment is attached to a pin when it sits in the unbroken run of comment
# lines directly above it, or inline on its line. A version is a dotted or quoted number (`v` and a
# `-rc1` suffix allowed), or a bare number right after a pin name (`node 20`); the word before it is
# the tool it names. An attached comment may name only its
# pin's key or CI input name (node-version, python-version, setup-task's version), or for a
# Taskfile pin its tool (HELM_DOCS_VERSION -> helm-docs), at the pin's value, `v` stripped.
comment_findings() {
  awk -v kind="$2" -v file="$(basename "$1")" '
    function vstrip(s) { gsub(/["\047]/, "", s); sub(/^[vV]/, "", s); return s }
    function pin_of(i,    l, k, v) {
      l = line[i]
      if (l ~ /^[[:space:]]*(#|$)/) return ""
      sub(/[[:space:]]#.*$/, "", l)
      if (kind == "mise") {
        if (sect[i] != "[tools]" || l !~ /^[[:space:]]*[A-Za-z0-9_-]+[[:space:]]*=/) return ""
        k = l; sub(/^[[:space:]]*/, "", k); sub(/[[:space:]]*=.*$/, "", k)
        v = l; sub(/^[^=]*=[[:space:]]*/, "", v)
      } else {
        if (l !~ /^[[:space:]]*[A-Z][A-Z0-9_]*_VERSION:[[:space:]]*["\047]?[vV]?[0-9]/) return ""
        k = l; sub(/^[[:space:]]*/, "", k); sub(/:.*$/, "", k)
        v = l; sub(/^[^:]*:[[:space:]]*/, "", v)
      }
      gsub(/[[:space:]]/, "", v)
      pinval[i] = vstrip(v)
      return k
    }
    function names_of(k,    t) {
      if (kind == "mise") {
        if (k == "node") return " node node-version "
        if (k == "python") return " python python-version "
        if (k == "task") return " task version "
        return " " tolower(k) " "
      }
      t = k; sub(/_VERSION$/, "", t); gsub(/_/, "-", t)
      return " " tolower(k) " " tolower(t) " "
    }
    function check(i, c, p,    tok, pre, name, val, cnt, lc) {
      c = " " c
      lc = tolower(c)
      # One count reds the line; later counts on it add nothing.
      if (match(lc, /[0-9]+[[:space:]]+(occurrences|sites|places|workflows)([^a-z]|$)/)) {
        cnt = substr(lc, RSTART, RLENGTH); sub(/[^a-z]$/, "", cnt)
        print file ":" i " comment counts pin sites (\"" cnt "\") -- the count goes stale with the next site; point to the owning file instead"
      }
      # The character before the number is kept in tok: an opening quote marks a quoted version.
      while (match(c, /[^A-Za-z0-9._-][vV]?[0-9]+(\.[0-9]+)*(-[0-9A-Za-z]+(\.[0-9A-Za-z]+)*)?/)) {
        tok = substr(c, RSTART, RLENGTH)
        pre = substr(c, 1, RSTART)
        c = substr(c, RSTART + RLENGTH)
        sub(/^[^0-9vV"\047]/, "", tok)
        val = vstrip(tok)
        sub(/[[:space:]:=(\140"\047]+$/, "", pre)
        name = ""
        if (match(pre, /[A-Za-z][A-Za-z0-9_-]*$/)) name = substr(pre, RSTART)
        # A bare number (no dot, no quotes) is a version only right after a pin name ("node 20");
        # elsewhere it is prose ("all 17", "Phase 2").
        if (val !~ /\./ && tok !~ /^["\047]/ && index(allnames, " " tolower(name) " ") == 0) continue
        if (p == 0) {
          print file ":" i " comment restates " (name == "" ? "a version" : name) " " val " away from its pin -- point to the owning file instead, or move it directly above (or onto) the pin it names"
        } else if (index(names_of(pin[p]), " " tolower(name) " ") == 0) {
          print file ":" i " comment attached to " pin[p] " = " pinval[p] " names " (name == "" ? "an unnamed version" : name) " " val ", which is not that pin" "\047" "s tool"
        } else if (val != pinval[p]) {
          print file ":" i " comment attached to " pin[p] " = " pinval[p] " says " name " " val ", not " pinval[p] " -- an attached comment gone stale"
        }
      }
    }
    { line[NR] = $0 }
    /^[[:space:]]*\[/ { s = $0; gsub(/[[:space:]]/, "", s) }
    { sect[NR] = s }
    END {
      allnames = " "
      for (i = 1; i <= NR; i++) {
        pin[i] = pin_of(i)
        if (pin[i] != "") allnames = allnames substr(names_of(pin[i]), 2)
      }
      for (i = 1; i <= NR; i++) {
        l = line[i]
        if (l ~ /^[[:space:]]*#/) {
          j = i + 1
          while (j <= NR && line[j] ~ /^[[:space:]]*#/) j++
          c = l; sub(/^[[:space:]]*#/, "", c)
          check(i, c, (j <= NR && pin[j] != "") ? j : 0)
        } else if (match(l, /[[:space:]]#/)) {
          check(i, substr(l, RSTART + 2), pin[i] != "" ? i : 0)
        }
      }
    }' "$1"
}

# check_comments <root>: DTP-5 over mise.toml and Taskfile.yml.
check_comments() {
  local root="$1" f kind finding n=0
  for f in mise.toml Taskfile.yml; do
    [[ -f "${root}/${f}" ]] || continue
    kind=taskfile
    [[ "${f}" == mise.toml ]] && kind=mise
    while IFS= read -r finding; do
      [[ -n "${finding}" ]] || continue
      err "DTP-5: ${finding}"
      n=$((n + 1))
    done < <(comment_findings "${root}/${f}" "${kind}")
  done
  echo "comments: ${n} DTP-5 finding(s) in mise.toml and Taskfile.yml"
}

# --- DTP-4 Renovate managers: every manager is alive, every pin site is covered, one group per tool ---
# Mechanism (task 3.1): a model of Renovate's regex managers and packageRules order, evaluated by
# python3 over renovate.json itself (no copy of the regexes to drift). Checked against Renovate
# 44.115.4 `--platform=local` dry-runs on this tree (openspec change developer-toolchain-pins,
# evidence/G3.md). Anything the model does not implement (a glob it cannot read, an unknown
# matcher or manager key, a template helper, a rule whose group depends on the update type) is a
# failure, never a guess.
#
# renovate_deps <root>: reads the tracked file list on stdin and prints one line per dependency a
# custom regex manager extracts, "DEP\t<manager>\t<file>\t<line of the value>\t<value>\t<depName>\t
# <groupName or ->", and one "ERR\t<message>" per manager that is dead (no file, or no dependency
# from its files), per file pattern that matches no tracked file, and per config shape the model
# cannot evaluate.
renovate_deps() {
  # The program comes from -c: stdin carries the file list.
  python3 -c "${RENOVATE_MODEL}" "$1"
}
RENOVATE_MODEL="$(cat <<'PY'
import json, re, sys

root = sys.argv[1]
files = [l for l in sys.stdin.read().split("\n") if l]
out = []

def err(msg):
    out.append("ERR\t" + msg)

try:
    raw = open(root + "/renovate.json", encoding="utf-8").read()
    cfg = json.loads(raw)
except (OSError, ValueError) as e:
    print("ERR\trenovate.json cannot be read as JSON (%s) -- no manager can be checked" % e)
    sys.exit(0)

def js_regex(src, flags=""):
    # Renovate compiles with RE2 (JS syntax). RE2 has no lookaround, backreference, atomic group or
    # possessive quantifier, all of which Python would accept; named groups are the only spelling
    # Python differs on. Character classes are skipped: `[*+]` is two literals.
    i = 0
    while i < len(src):
        c = src[i]
        if c == "\\":
            if i + 1 < len(src) and (src[i + 1] in "123456789" or src[i + 1] == "k"):
                raise ValueError("backreference %r is not RE2" % src[i:i + 2])
            i += 2
            continue
        if c == "[":
            j = i + 1
            if src.startswith("^", j):
                j += 1
            if src.startswith("]", j):
                j += 1
            while j < len(src) and src[j] != "]":
                j += 2 if src[j] == "\\" else 1
            i = j + 1
            continue
        if src.startswith(("(?=", "(?!", "(?<=", "(?<!"), i):
            raise ValueError("lookaround %r is not RE2" % src[i:i + 4])
        for t in ("(?P=", "(?>"):
            if src.startswith(t, i):
                raise ValueError("construct %r is not RE2" % t)
        if c in "*+?}" and src.startswith("+", i + 1):
            raise ValueError("possessive quantifier %r is not RE2" % src[i:i + 2])
        i += 1
    py = re.sub(r"\(\?<([A-Za-z_][A-Za-z0-9_]*)>", r"(?P<\1>", src)
    f = 0
    for c in flags:
        if c == "i":
            f |= re.I
        else:
            raise ValueError("regex flag %r is not modelled" % c)
    return re.compile(py, f)

def glob_regex(g):
    if re.search(r"[{}\[\]]", g):
        raise ValueError("glob %r uses braces or classes, which the model does not read" % g)
    s, i = "", 0
    while i < len(g):
        if g.startswith("**/", i):
            s += "(?:.*/)?"; i += 3
        elif g.startswith("**", i):
            s += ".*"; i += 2
        elif g[i] == "*":
            s += "[^/]*"; i += 1
        elif g[i] == "?":
            s += "[^/]"; i += 1
        else:
            s += re.escape(g[i]); i += 1
    return re.compile(s + r"\Z")

def pattern(p):
    # "/re/flags" is a regex (searched), anything else a glob (whole string).
    m = re.fullmatch(r"/(.*)/([a-z]*)", p, re.S)
    if m:
        r = js_regex(m.group(1), m.group(2))
        return lambda s: r.search(s) is not None
    r = glob_regex(p)
    return lambda s: r.match(s) is not None

def match_list(pats, value):
    # Renovate's matchRegexOrGlobList: any positive matches, and no "!" negation matches.
    if value is None:
        return False
    pos = [p for p in pats if not p.startswith("!")]
    neg = [p[1:] for p in pats if p.startswith("!")]
    if pos and not any(pattern(p)(value) for p in pos):
        return False
    return not any(pattern(n)(value) for n in neg)

def render(tpl, groups, what):
    def sub(m):
        return groups.get(m.group(1) or m.group(2)) or ""
    if re.search(r"\{\{[#/^>!&]", tpl):
        raise ValueError("%s template %r uses a helper the model does not run" % (what, tpl))
    return re.sub(r"\{\{\{\s*([A-Za-z_]\w*)\s*\}\}\}|\{\{\s*([A-Za-z_]\w*)\s*\}\}", sub, tpl)

try:
    files = [f for f in files
             if not any(p in f or glob_regex(p).match(f) for p in cfg.get("ignorePaths", []))]
except ValueError as e:
    err("renovate.json ignorePaths: %s" % e)

em = cfg.get("enabledManagers")
if em is not None and not ({"custom.regex", "regex"} & set(em)):
    err("renovate.json enabledManagers leaves out custom.regex -- every custom manager is off")

MANAGER_KEYS = {"customType", "description", "managerFilePatterns", "matchStrings",
                "matchStringsStrategy", "datasourceTemplate", "depNameTemplate",
                "packageNameTemplate", "versioningTemplate", "extractVersionTemplate",
                "autoReplaceStringTemplate", "currentValueTemplate", "registryUrlTemplate"}
FIELDS = ("datasource", "depName", "packageName", "currentValue")

starts = [raw.count("\n", 0, m.start()) + 1 for m in re.finditer(r'"customType"', raw)]
deps = []
for i, m in enumerate(cfg.get("customManagers", [])):
    desc = (m.get("description") or "").split(" -- ")[0]
    if len(desc) > 60:
        desc = desc[:57] + "..."
    label = 'customManagers[%d] "%s" (renovate.json:%s)' % (i, desc, starts[i] if i < len(starts) else "?")
    try:
        extra = sorted(set(m) - MANAGER_KEYS)
        if extra:
            raise ValueError("keys %s are not modelled" % ", ".join(extra))
        for k in sorted(m):
            # Every template, rendered here or not (autoReplaceStringTemplate is not), may only
            # substitute names: a helper is a shape the model does not run.
            if k.endswith("Template"):
                render(m[k], {}, k)
        if m.get("customType") != "regex":
            raise ValueError("customType %r is not modelled" % m.get("customType"))
        if m.get("matchStringsStrategy", "any") != "any":
            raise ValueError("matchStringsStrategy %r is not modelled" % m.get("matchStringsStrategy"))
        pats = m.get("managerFilePatterns") or []
        strings = [js_regex(s) for s in m.get("matchStrings") or []]
        if not pats or not strings:
            raise ValueError("no managerFilePatterns or no matchStrings")
        hit = []
        for p in pats:
            ok = [f for f in files if pattern(p)(f)]
            if not ok:
                err("%s: file pattern %s matches no tracked file -- the pattern has rotted" % (label, p))
            hit += [f for f in ok if f not in hit]
        found = 0
        for f in hit:
            try:
                text = open(root + "/" + f, encoding="utf-8").read()
            except (OSError, UnicodeDecodeError):
                continue
            for r in strings:
                for mm in r.finditer(text):
                    g = {k: v for k, v in mm.groupdict().items() if v is not None}
                    d = {}
                    for k in FIELDS:
                        tpl = m.get(k + "Template")
                        d[k] = render(tpl, g, k) if tpl is not None else g.get(k, "")
                    if not d["packageName"]:
                        d["packageName"] = d["depName"]
                    if not (d["currentValue"] and d["depName"] and d["datasource"]):
                        continue
                    pos = mm.start("currentValue") if "currentValue" in g else mm.start()
                    d.update(file=f, line=text.count("\n", 0, pos) + 1, manager=label)
                    deps.append(d)
                    found += 1
        if not hit:
            err("%s matches no tracked file -- a dead manager" % label)
        elif not found:
            err("%s matches %d file(s) (%s) but its matchStrings extract no dependency from them -- a dead manager"
                % (label, len(hit), ", ".join(hit[:3]) + (", ..." if len(hit) > 3 else "")))
    except (ValueError, re.error) as e:
        err("%s cannot be evaluated: %s" % (label, e))

# What each matcher compares for a custom regex dependency (None: never set, so never matches).
MATCHERS = {"matchManagers": lambda d: "custom.regex", "matchDepNames": lambda d: d["depName"],
            "matchPackageNames": lambda d: d["packageName"], "matchFileNames": lambda d: d["file"],
            "matchDatasources": lambda d: d["datasource"], "matchDepTypes": lambda d: None,
            "matchUpdateTypes": None}
for d in deps:
    group, enabled = "", True
    for n, rule in enumerate(cfg.get("packageRules", [])):
        try:
            other = [k for k in rule if k.startswith(("match", "exclude")) and k not in MATCHERS]
            if other:
                raise ValueError("matcher %s is not modelled" % ", ".join(other))
            hit = True
            for k, get in MATCHERS.items():
                if k in rule and get is not None:
                    vals = ["custom.regex" if v == "regex" else v for v in rule[k]] if k == "matchManagers" else rule[k]
                    hit = hit and match_list(vals, get(d))
            if not hit:
                continue
            if "matchUpdateTypes" in rule and ("groupName" in rule or "enabled" in rule):
                raise ValueError("its groupName/enabled depends on the update type, which only a lookup knows")
            # The LAST matching rule that sets a key wins.
            group = rule.get("groupName", group)
            enabled = rule.get("enabled", enabled)
        except (ValueError, re.error) as e:
            err("renovate.json packageRules[%d] cannot be evaluated for %s (%s:%d): %s"
                % (n, d["depName"], d["file"], d["line"], e))
    if enabled:
        out.append("\t".join(["DEP", d["manager"], d["file"], str(d["line"]), d["currentValue"],
                              d["depName"], group or "-"]))
print("\n".join(out))
PY
)"

# tracked_files <root>: the files Renovate sees. A git work tree lists its index; a self-test copy
# holds the tracked files only, so it lists everything.
tracked_files() {
  if [[ "$(git -C "$1" rev-parse --show-toplevel 2>/dev/null)" == "$(cd "$1" && pwd -P)" ]]; then
    git -C "$1" ls-files
  else
    (cd "$1" && find . -type f | sed 's|^\./||' | sort)
  fi
}

# golangci_sites <root>: Makefile GOLANGCI_LINT_VERSION and the custom build's `version:`. Their
# agreement and floor are CFC-2's (consistency_tool_pins_test.sh); DTP-4 only needs the sites.
golangci_sites() {
  local root="$1"
  if [[ -f "${root}/Makefile" ]]; then
    awk '/^GOLANGCI_LINT_VERSION[[:space:]]*\?=/ { v = $0; sub(/^[^=]*=[[:space:]]*/, "", v); sub(/[[:space:]]+$/, "", v); print "Makefile:" NR "\t" v }' "${root}/Makefile"
  fi
  if [[ -f "${root}/hack/tooling/.custom-gcl.yml" ]]; then
    awk '/^version:/ { v = $0; sub(/^version:[[:space:]]*/, "", v); sub(/[[:space:]]+#.*$/, "", v); gsub(/["\047]/, "", v); print "hack/tooling/.custom-gcl.yml:" NR "\t" v }' "${root}/hack/tooling/.custom-gcl.yml"
  fi
}

# COVER: "<tool>\t<file>:<line>\t<value as written>\t<value compared>" for every pin site DTP-4
# requires a manager for (sites without a compared form compare with a leading v stripped).
COVER=""
cover_add() {
  local tool="$1" loc v c
  while IFS=$'\t' read -r loc v c; do
    [[ -n "${loc}" ]] && COVER+="${tool}"$'\t'"${loc}"$'\t'"${v}"$'\t'"${c:-${v#v}}"$'\n'
  done <<<"$2"
}

# The depNames a manager must give each tool's sites: a site extracted under another name is looked
# up, and grouped, as another tool.
declare -A TOOL_DEPS=(
  [gitleaks]="gitleaks/gitleaks"
  [git-cliff]="orhun/git-cliff"
  [shellcheck]="koalaman/shellcheck shellcheck-py/shellcheck-py"
  [task]="go-task/task"
  [golangci-lint]="golangci/golangci-lint"
  [govulncheck]="golang.org/x/vuln"
)

# check_managers <root>: DTP-4 over COVER (filled by check_all) and renovate_deps.
check_managers() {
  local root="$1" deps line tool loc v c file ln covered tag dfile dline dval dname dgroup
  local -A groups=() where=()
  if ! command -v python3 >/dev/null 2>&1; then
    err "renovate: python3 not found; the manager model needs it -- an unchecked config never passes"
    return 0
  fi
  if [[ ! -f "${root}/renovate.json" ]]; then
    err "renovate: renovate.json is missing -- no pin site has a manager"
    return 0
  fi
  if ! deps="$(tracked_files "${root}" | renovate_deps "${root}" 2>"${WORKROOT}/model.err")"; then
    err "renovate: the renovate.json model failed ($(tail -1 "${WORKROOT}/model.err")) -- a config it cannot read never passes"
    return 0
  fi
  while IFS=$'\t' read -r tag line; do
    [[ "${tag}" == "ERR" ]] && err "renovate: ${line}"
  done <<<"${deps}"
  while IFS=$'\t' read -r tool loc v c; do
    [[ -n "${tool}" ]] || continue
    # A value that is not a plain release is already the tool check's failure (one clause per site).
    [[ "${c}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
    file="${loc%%:*}" ln="${loc##*:}" covered=""
    while IFS=$'\t' read -r tag _ dfile dline dval dname dgroup; do
      [[ "${tag}" == "DEP" && "${dfile}" == "${file}" && "${dline}" == "${ln}" && "${dval#v}" == "${v#v}" ]] || continue
      [[ " ${TOOL_DEPS[${tool}]} " == *" ${dname} "* ]] || continue
      covered="${dgroup}"
      break
    done <<<"${deps}"
    if [[ -z "${covered}" ]]; then
      err "renovate: ${tool} pin ${v} at ${loc} is extracted by no custom manager as ${TOOL_DEPS[${tool}]// / or } -- the bot never bumps ${file}"
      continue
    fi
    groups[${tool}]+="${covered}"$'\n'
    where[${tool}]+="${loc} in '${covered}'; "
  done <<<"${COVER}"
  for tool in "${!groups[@]}"; do
    if [[ "$(sort -u <<<"${groups[${tool}]}" | grep -c .)" -ne 1 ]] || grep -qx -- '-' <<<"${groups[${tool}]}"; then
      err "renovate: ${tool} sites are not in one group (${where[${tool}]%; }) -- a bump can move one site and leave the others behind"
    fi
  done
  if [[ -n "${groups[shellcheck]:-}" ]]; then
    dgroup="$(head -1 <<<"${groups[shellcheck]}")"
    while IFS=$'\t' read -r tag _ dfile dline dval dname line; do
      [[ "${tag}" == "DEP" && "${line}" == "${dgroup}" && "${dgroup}" != "-" ]] || continue
      [[ " ${TOOL_DEPS[shellcheck]} " == *" ${dname} "* ]] && continue
      err "renovate: shellcheck shares the group '${dgroup}' with ${dname} (${dfile}:${dline}) -- a shellcheck bump stays red until install-shellcheck.sh has its digests, and must hold back no other tool"
      break
    done <<<"${deps}"
  fi
  echo "renovate: $(grep -c '^DEP' <<<"${deps}" || true) custom-manager dependencies, $(grep -c . <<<"${COVER}") pin sites checked"
}

# --- DTP-2 govulncheck ---
GOVULN_PKG="golang.org/x/vuln/cmd/govulncheck"
GOVULN_ANNOTATION="# renovate: datasource=go depName=golang.org/x/vuln"

# vulncheck_lines <root>: "<line>\t<text>" for every non-comment line of the top-level `vulncheck:`
# task in Taskfile.yml, quotes dropped (`pkg@"v1.9.0"` is the same pin); nothing if the task is absent.
vulncheck_lines() {
  awk '
    /^tasks:/ { in_tasks = 1; next }
    in_tasks && /^  vulncheck:[[:space:]]*$/ { in_task = 1; next }
    !in_task || /^[[:space:]]*(#|$)/ { next }
    { match($0, /^[[:space:]]*/) }
    RLENGTH <= 2 { in_task = 0; next }
    { line = $0; gsub(/["\047]/, "", line); print NR "\t" line }
  ' "$1/Taskfile.yml"
}

# check_govulncheck <root>: Taskfile.yml defines GOVULNCHECK_VERSION directly under its Renovate
# annotation, and every govulncheck the vulncheck task runs reads that variable.
check_govulncheck() {
  local root="$1" f="Taskfile.yml" def line above lines spec ver n text found=0
  # Only the top-level vars: block defines it; the same key under env: is not {{.GOVULNCHECK_VERSION}}.
  def="$(awk '
    /^[^[:space:]#]/ { section = $0; sub(/:.*/, "", section) }
    section == "vars" && /^  GOVULNCHECK_VERSION:/ { print NR; exit }
  ' "${root}/${f}" 2>/dev/null || true)"
  if [[ -z "${def}" ]]; then
    err "govulncheck: ${f} defines no GOVULNCHECK_VERSION under vars: -- {{.GOVULNCHECK_VERSION}} resolves to nothing and Renovate has no pin to bump"
  else
    line="$(sed -n "${def}p" "${root}/${f}")"
    above="$(sed -n "$((def - 1))p" "${root}/${f}")"
    # Exact forms: renovate.json's Taskfile manager needs `depName=<x>` then a newline, and
    # `_VERSION: ` then the value, so trailing blanks or a second space extract nothing.
    [[ "${above}" =~ ^[[:space:]]*"${GOVULN_ANNOTATION}"$ ]] ||
      err "govulncheck: ${f}:${def} GOVULNCHECK_VERSION has no '${GOVULN_ANNOTATION}' line directly above it -- Renovate stops seeing the pin"
    [[ "${line}" =~ ^"  GOVULNCHECK_VERSION: "v[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
      err "govulncheck: ${f}:${def} GOVULNCHECK_VERSION is '${line#*: }', not one plain release (vN.N.N) -- latest or a variable floats, and Renovate cannot bump it"
  fi
  if ! grep -qE '^  vulncheck:[[:space:]]*$' "${root}/${f}" 2>/dev/null; then
    err "govulncheck: ${f} has no vulncheck task -- the pin it should run is unchecked"
    return 0
  fi
  lines="$(vulncheck_lines "${root}")"
  while IFS=$'\t' read -r n text; do
    [[ -n "${n}" ]] || continue
    # A task-level var wins over the global one, so the annotated pin would not be the one that runs.
    [[ ! "${text}" =~ GOVULNCHECK_VERSION[[:space:]]*: ]] ||
      err "govulncheck: ${f}:${n} task vulncheck sets its own GOVULNCHECK_VERSION -- a task var overrides the annotated one under vars:"
    while IFS= read -r spec; do
      [[ -n "${spec}" ]] || continue
      found=1
      ver="${spec#*@}"
      if [[ "${ver}" == "{{.GOVULNCHECK_VERSION}}" ]]; then
        continue
      elif [[ "${ver}" =~ ^v?[0-9] ]]; then
        err "govulncheck: ${f}:${n} task vulncheck pins ${spec} inline -- the version belongs in GOVULNCHECK_VERSION, where Renovate's annotation sees it"
      else
        err "govulncheck: ${f}:${n} task vulncheck runs ${spec}, not {{.GOVULNCHECK_VERSION}} -- the annotated pin is not the one that runs"
      fi
    done < <(grep -oE "${GOVULN_PKG//./\\.}@[^[:space:];&|)]+" <<<"${text}" || true)
  done <<<"${lines}"
  [[ "${found}" == 1 ]] ||
    err "govulncheck: ${f} task vulncheck runs no ${GOVULN_PKG}@{{.GOVULNCHECK_VERSION}} -- the annotated pin is not the one that runs"
  echo "govulncheck: task vulncheck runs GOVULNCHECK_VERSION $(taskfile_sites "${root}" GOVULNCHECK_VERSION | cut -f2 | head -1) (${f}:${def:-<undefined>})"
}

check_all() {
  local root="$1" sites f loc v
  BROKEN=0
  COVER=""

  sites="$({
    yaml_env_sites "${root}" GITLEAKS_VERSION
    default_sites "${root}" GITLEAKS_VERSION
    precommit_rev_sites "${root}" 'github\.com/gitleaks/gitleaks(\.git)?$'
  } | with_compare plain)"
  check_tool gitleaks "${GITLEAKS_FLOOR}" hack/install-gitleaks.sh "${sites}" \
    .github/workflows/ci.yaml hack/install-gitleaks.sh .pre-commit-config.yaml
  cover_add gitleaks "${sites}"

  sites="$({
    taskfile_sites "${root}" GIT_CLIFF_VERSION
    action_version_sites "${root}" orhun/git-cliff-action
    cliff_install_sites "${root}"
  } | with_compare plain)"
  check_tool git-cliff "${GIT_CLIFF_FLOOR}" Taskfile.yml "${sites}" \
    Taskfile.yml .github/workflows/release.yaml hack/verify-changelog.sh
  cover_add git-cliff "${sites}"

  sites="$({
    yaml_env_sites "${root}" SHELLCHECK_VERSION | with_compare plain
    default_sites "${root}" SHELLCHECK_VERSION | with_compare plain
    precommit_rev_sites "${root}" 'github\.com/shellcheck-py/shellcheck-py(\.git)?$' | with_compare first3
  })"
  check_tool shellcheck - hack/install-shellcheck.sh "${sites}" \
    .github/workflows/ci.yaml hack/install-shellcheck.sh .pre-commit-config.yaml
  cover_add shellcheck "${sites}"
  if [[ -n "${AGREED}" ]]; then
    check_shellcheck_digests "${root}" "${AGREED}"
  fi

  # DTP-1: mise.toml is a site, but its presence is check_mise's (one clause, one message).
  sites="$(action_version_sites "${root}" go-task/setup-task)"
  if [[ -z "${sites}" ]]; then
    err "task: the scan found no go-task/setup-task step under .github -- an empty scan never passes"
  else
    sites="$({
      printf '%s\n' "${sites}"
      if [[ -f "${root}/mise.toml" ]]; then mise_tool_sites "${root}/mise.toml" task; fi
    } | with_compare plain)"
    check_tool task "${TASK_FLOOR}" .github/workflows/ci.yaml "${sites}" \
      .github/workflows/ci.yaml .github/actions/kind-e2e-setup/action.yml
    cover_add task "${sites}"
  fi

  sites="$(golangci_sites "${root}")"
  for f in Makefile hack/tooling/.custom-gcl.yml; do
    awk -v f="${f}" 'index($0, f ":") == 1 { found = 1 } END { exit !found }' <<<"${sites}" ||
      err "golangci-lint: no pin found in ${f} -- DTP-4 cannot check that a manager covers it"
  done
  while IFS=$'\t' read -r loc v; do
    [[ -z "${loc}" || "${v}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
      err "golangci-lint: ${loc} says '${v}', not a plain vN.N.N release -- renovate.json's golangci-lint manager extracts only that form"
  done <<<"${sites}"
  cover_add golangci-lint "${sites}"
  # DTP-2's site, the definition under vars: (a task-level one is check_govulncheck's failure); its
  # annotation form is check_govulncheck's too, DTP-4 checks the manager extracts it.
  if [[ -f "${root}/Taskfile.yml" ]]; then
    cover_add govulncheck "$(awk '
      /^[^[:space:]#]/ { section = $0; sub(/:.*/, "", section) }
      section == "vars" && /^  GOVULNCHECK_VERSION:/ { v = $0; sub(/^  GOVULNCHECK_VERSION:[[:space:]]*/, "", v); print "Taskfile.yml:" NR "\t" v }
    ' "${root}/Taskfile.yml")"
  fi

  check_govulncheck "${root}"
  check_unverified_limit "${root}"
  check_mise "${root}"
  check_comments "${root}"
  check_managers "${root}"
  return "${BROKEN}"
}

if [[ "${MODE}" == "check" ]]; then
  if check_all "${ROOT}"; then
    pass "Task, gitleaks, git-cliff and shellcheck sites agree and meet their floors, shellcheck has digests for its pin, task vulncheck runs the annotated GOVULNCHECK_VERSION, UNVERIFIED_BY_DESIGN is within its limit, mise.toml agrees with every node/python pin in .github with Go read from go.mod, and no mise.toml or Taskfile.yml comment restates a version away from its pin, and every renovate.json custom manager is alive, covers every pin site and keeps each tool in one group (shellcheck alone)"
    exit 0
  fi
  fail "toolchain pins disagree (listed above)"
fi

# ---------------------------------------------------------------------------
# Self-test (DTP-7): copy the tracked tree, break one clause per copy, and assert the gate reds
# WITH THE MESSAGE of that clause -- a non-zero exit alone is not evidence. Sentinels are EREs.
# ---------------------------------------------------------------------------
command -v git >/dev/null 2>&1 || fail "git not found; the self-test copies the tracked tree"
command -v yq >/dev/null 2>&1 || fail "yq not found; the wiring cases run hack/test/ci_workflow_security_test.sh, which needs it"
git -C "${ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1 ||
  fail "self-test: ${ROOT} is not a git work tree; the self-test copies its tracked files"

# Called directly (never as $(new_copy)), so COPY is set in this shell; WORKROOT's trap removes it.
COPY=""
# Each case needs only its own copy: the previous one is removed first, so ~120 cases hold one
# tree (~2000 files) at a time instead of every tree until exit (inodes on a shared /tmp).
new_copy() {
  if [[ -n "${COPY}" ]]; then rm -rf "${COPY:?}"; fi
  COPY="$(mktemp -d "${WORKROOT}/XXXXXXXX")"
  (cd "${ROOT}" && git ls-files -z | tar --null --ignore-failed-read -T - -cf - 2>/dev/null) |
    tar -xf - -C "${COPY}"
}

# probe <label> <probe> <file>: the mutation landed. A probe starting with `!` must NOT match (a
# missing file matches nothing, so a deleting mutant also proves the deletion in its command);
# several probes joined by `@@` must each hold.
probe() {
  local label="$1" p="$2" file="$3" one
  if [[ "${p}" == *@@* ]]; then
    while IFS= read -r one; do probe "${label}" "${one}" "${file}"; done < <(sed 's/@@/\n/g' <<<"${p}")
    return 0
  fi
  if [[ "${p}" == '!'* ]]; then
    ! grep -Eq -- "${p#!}" "${COPY}/${file}" ||
      fail "self-test: the '${label}' mutant did not land ('${p#!}' still in ${file}) -- nothing was tested"
  else
    grep -Eq -- "${p}" "${COPY}/${file}" ||
      fail "self-test: the '${label}' mutant did not land (no '${p}' in ${file}) -- nothing was tested"
  fi
}

# GATE: the script each case runs against the copy; a relative path is the copy's own script.
GATE="${SELF}"
run_gate() {
  local g="${GATE}"
  [[ "${g}" == /* ]] || g="${COPY}/${g}"
  bash "${g}" "${COPY}" 2>&1
}

# mutant_rejected <label> <sentinel-ERE> <probe> <probe-file> <command...>
mutant_rejected() {
  local label="$1" sentinel="$2" p="$3" file="$4" out rc others
  shift 4
  new_copy
  (cd "${COPY}" && "$@") || fail "self-test: the mutation step for '${label}' failed -- nothing was tested"
  probe "${label}" "${p}" "${file}"
  out="$(run_gate)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Eq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected '${sentinel}', got: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  # One clause per mutant: every other FAIL line (bar this gate's closing summary) is a mutant
  # that also trips a clause it does not name, so it proves less than its label says.
  others="$(grep '^FAIL: ' <<<"${out}" | grep -v '^FAIL: toolchain pins disagree (listed above)$' | grep -Ev -- "${sentinel}" || true)"
  # ALSO_RED (set on the call): a second clause the mutant trips by construction. It must fire,
  # and it is the only other clause allowed.
  if [[ -n "${ALSO_RED:-}" ]]; then
    grep -Eq -- "${ALSO_RED}" <<<"${others}" ||
      fail "self-test: the '${label}' mutant did not also trip '${ALSO_RED}', which it declares"
    others="$(grep -Ev -- "${ALSO_RED}" <<<"${others}" || true)"
  fi
  [[ -z "${others}" ]] ||
    fail "self-test: the '${label}' mutant also trips other clauses: $(head -3 <<<"${others}" | tr '\n' ' ')"
  pass "self-test: ${label} rejected"
}

# control_passes <label> <probe> <probe-file> <command...>
control_passes() {
  local label="$1" p="$2" file="$3" out
  shift 3
  new_copy
  (cd "${COPY}" && "$@") || fail "self-test: the control step for '${label}' failed -- nothing was tested"
  probe "${label}" "${p}" "${file}"
  out="$(run_gate)" ||
    fail "self-test: the gate red on the '${label}' control -- it must pass: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: ${label} control passes"
}

# drop_step <file> <run-line ERE>: delete each matching `run:` line and the `- name:` line above it.
drop_step() {
  awk -v re="$2" '
    have && $0 ~ re && prev ~ /^[[:space:]]*-[[:space:]]+name:/ { have = 0; next }
    { if (have) print prev; prev = $0; have = 1 }
    END { if (have) print prev }' "$1" >"$1.tmp" && mv "$1.tmp" "$1"
}

CI=.github/workflows/ci.yaml
PC=.pre-commit-config.yaml

# DTP-7: a no-op copy passes (a guard that reds on correct content is no guard).
new_copy
out="$(run_gate)" ||
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree (or the gate) first: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
pass "self-test: no-op control passes on an unmutated copy"

# DTP-7: a correct Renovate group bump keeps the guard green. Renovate rewrites every site its
# custom managers extract, each group in one PR; this copy moves EVERY extracted pin (Task,
# gitleaks, git-cliff, shellcheck, helm, govulncheck, golangci-lint and the rest) to the next
# patch release of its own value, components after the third reset to 0 (shellcheck-py
# v0.11.0.1 -> v0.11.1.0), and adds the digests a shellcheck bump needs by hand (the
# pinned_sha256 arms of the old version, copied under the new one with stand-in digests).
# Plain mode and --self-test must both pass on it, so no sentinel, probe or mutant value may
# hard-code today's pins. The nested run skips this control (it would recurse).
BUMP_PY="$(cat <<'PY'
import hashlib, re, sys

root = sys.argv[1]
def nxt(v):
    pre, num = ("v", v[1:]) if v.startswith("v") else ("", v)
    parts = num.split(".")
    if len(parts) < 3 or not all(p.isdigit() for p in parts):
        sys.exit("cannot bump %r: not a dotted release of three or more numbers" % v)
    parts[2] = str(int(parts[2]) + 1)
    return pre + ".".join(parts[:3] + ["0"] * (len(parts) - 3))

sites, sc = {}, None
for row in sys.stdin.read().splitlines():
    _, _, f, line, val, dep, _ = row.split("\t")
    sites.setdefault((f, int(line)), val)
    if dep == "koalaman/shellcheck":
        sc = (val.lstrip("v"), nxt(val).lstrip("v"))
if sc is None:
    sys.exit("no koalaman/shellcheck pin extracted")
edits = {}
for (f, line), val in sorted(sites.items()):
    edits.setdefault(f, []).append((line, val, nxt(val)))
for f, rows in edits.items():
    text = open(root + "/" + f, encoding="utf-8").read().split("\n")
    for line, val, new in rows:
        if val not in text[line - 1]:
            sys.exit("%s:%d does not hold %s" % (f, line, val))
        text[line - 1] = text[line - 1].replace(val, new, 1)
        print("%s\t%d\t%s" % (f, line, new))
    open(root + "/" + f, "w", encoding="utf-8").write("\n".join(text))
si = root + "/hack/install-shellcheck.sh"
src = open(si, encoding="utf-8").read()
arm = re.compile(r'^    shellcheck-v%s\.([a-z0-9_]+\.[a-z0-9_]+)\.tar\.xz\)\n      echo "[0-9a-f]{64}"\n      ;;\n' % re.escape(sc[0]), re.M)
new_arms = ['    shellcheck-v%s.%s.tar.xz)\n      echo "%s"\n      ;;\n' % (sc[1], m.group(1), hashlib.sha256(m.group(1).encode()).hexdigest())
            for m in arm.finditer(src)]
tail = "    *)\n      return 1\n"
if not new_arms or tail not in src:
    sys.exit("no pinned_sha256 arms for shellcheck %s in hack/install-shellcheck.sh" % sc[0])
open(si, "w", encoding="utf-8").write(src.replace(tail, "".join(new_arms) + tail, 1))
PY
)"
if [[ -z "${KOLLECT_TP_NESTED:-}" ]]; then
  new_copy
  before="$(tracked_files "${COPY}" | renovate_deps "${COPY}" | grep '^DEP' || true)"
  [[ -n "${before}" ]] || fail "self-test: the Renovate model extracts no dependency from the tree -- the bump control has nothing to bump"
  want="$(python3 -c "${BUMP_PY}" "${COPY}" <<<"${before}" | sort -u)" ||
    fail "self-test: the bump control could not rewrite the copy"
  # Probe: the model extracts the same sites from the copy, each at its bumped value.
  got="$(tracked_files "${COPY}" | renovate_deps "${COPY}" | awk -F'\t' '$1 == "DEP" { print $3 "\t" $4 "\t" $5 }' | sort -u)"
  [[ -n "${want}" && "${got}" == "${want}" ]] ||
    fail "self-test: the bump control did not land as Renovate would: $(diff <(cat <<<"${want}") <(cat <<<"${got}") | head -4 | tr '\n' ' ')"
  out="$(run_gate)" ||
    fail "self-test: the gate red on a copy with every pin bumped to its next release -- a correct Renovate PR would be red: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: all-pins bump control passes ($(wc -l <<<"${want}") sites bumped)"
  # The nested --self-test copies the copy's tracked files: index them.
  { git -C "${COPY}" init -q && git -C "${COPY}" add -A -f; } ||
    fail "self-test: cannot index the bumped copy for its nested --self-test"
  out="$(KOLLECT_TP_NESTED=1 bash "${SELF}" --self-test "${COPY}" 2>&1)" ||
    fail "self-test: --self-test red on a copy with every pin bumped to its next release -- a sentinel, probe or mutant hard-codes a pin: $(grep FAIL <<<"${out}" | head -3 | tr '\n' ' ')"
  pass "self-test: all-pins bump control passes --self-test"
fi

# --- DTP-3 gitleaks ---
mutant_rejected "gitleaks pre-commit rev drifts" \
  'gitleaks: \.pre-commit-config\.yaml:[0-9]+ says v8\.29\.0, but the reference site hack/install-gitleaks\.sh' \
  'rev: v8\.29\.0' "${PC}" \
  sed -i -E '/gitleaks\/gitleaks/{n;s/rev: v[0-9.]+/rev: v8.29.0/}' "${PC}"

mutant_rejected "gitleaks below floor" \
  'gitleaks: every site says 8\.29\.0, below the floor 8\.30\.1' \
  'GITLEAKS_VERSION:-8\.29\.0' hack/install-gitleaks.sh \
  bash -c 'sed -i -E "s/^([[:space:]]*GITLEAKS_VERSION: )\"[0-9.]+\"/\1\"8.29.0\"/" '"${CI}"' &&
    sed -i -E "s/(GITLEAKS_VERSION:-)[0-9.]+/\18.29.0/" hack/install-gitleaks.sh &&
    sed -i -E "/gitleaks\/gitleaks/{n;s/rev: v[0-9.]+/rev: v8.29.0/}" '"${PC}"

mutant_rejected "gitleaks empty scan" \
  'gitleaks: the scan found no pin site' \
  '!GITLEAKS_VERSION' "${CI}" \
  bash -c 'sed -i -E "s/GITLEAKS_VERSION:/GL_VERSION:/" '"${CI}"' &&
    sed -i -E "s/GITLEAKS_VERSION:-/GL_VERSION:-/" hack/install-gitleaks.sh &&
    sed -i -E "s#github.com/gitleaks/gitleaks#github.com/example/gl#" '"${PC}"

mutant_rejected "gitleaks named site loses its pin" \
  'gitleaks: no pin found in hack/install-gitleaks\.sh' \
  'GITLEAKS_VERSION:\?' hack/install-gitleaks.sh \
  sed -i -E 's/\$\{GITLEAKS_VERSION:-[0-9.]+\}/${GITLEAKS_VERSION:?set GITLEAKS_VERSION}/' hack/install-gitleaks.sh

mutant_rejected "gitleaks pin is not a plain release" \
  "gitleaks: \\.github/workflows/ci\\.yaml:[0-9]+ says 'latest', not a plain release version" \
  'GITLEAKS_VERSION: "latest"' "${CI}" \
  sed -i -E 's/^([[:space:]]*GITLEAKS_VERSION: )"[0-9.]+"/\1"latest"/' "${CI}"

# Only shellcheck-py's rev may carry a fourth component; elsewhere it is not a release.
mutant_rejected "gitleaks pin with a fourth component" \
  "gitleaks: \\.github/workflows/ci\\.yaml:[0-9]+ says '8\\.30\\.1\\.1', not a plain release version" \
  'GITLEAKS_VERSION: "8\.30\.1\.1"' "${CI}" \
  sed -i -E 's/^([[:space:]]*GITLEAKS_VERSION: )"[0-9.]+"/\1"8.30.1.1"/' "${CI}"

# --- DTP-3 git-cliff ---
REL=.github/workflows/release.yaml
VC=hack/verify-changelog.sh
# Read from the tree, like SC_CUR below: a correct git-cliff bump must not break the cases that
# use today's value (the all-pins bump control above proves it).
GC_CUR="$(sed -n -E 's/^  GIT_CLIFF_VERSION: (v[0-9]+\.[0-9]+\.[0-9]+)$/\1/p' "${ROOT}/Taskfile.yml" | head -1)"
[[ -n "${GC_CUR}" ]] || fail "self-test: cannot read GIT_CLIFF_VERSION from Taskfile.yml -- run the plain mode first"
GC_RE="${GC_CUR//./\\.}"
mutant_rejected "git-cliff release.yaml drifts" \
  'git-cliff: \.github/workflows/release\.yaml:[0-9]+ says v2\.12\.0, but the reference site Taskfile\.yml' \
  'version: v2\.12\.0' "${REL}" \
  sed -i -E '/orhun\/git-cliff-action@/,/version:/ s/version: v[0-9.]+/version: v2.12.0/' "${REL}"

mutant_rejected "git-cliff verify-changelog.sh drifts" \
  'git-cliff: hack/verify-changelog\.sh:[0-9]+ says v2\.12\.0, but the reference site Taskfile\.yml' \
  'install-git-cliff\.sh v2\.12\.0' "${VC}" \
  sed -i -E 's/(install-git-cliff\.sh )v[0-9.]+/\1v2.12.0/' "${VC}"

mutant_rejected "git-cliff below floor" \
  'git-cliff: every site says 2\.12\.0, below the floor 2\.13\.1' \
  'GIT_CLIFF_VERSION: v2\.12\.0' Taskfile.yml \
  bash -c 'sed -i -E "s/^([[:space:]]*GIT_CLIFF_VERSION: )v[0-9.]+/\1v2.12.0/" Taskfile.yml &&
    sed -i -E "/orhun\/git-cliff-action@/,/version:/ s/version: v[0-9.]+/version: v2.12.0/" '"${REL}"' &&
    sed -i -E "s/(install-git-cliff\.sh )v[0-9.]+/\1v2.12.0/" '"${VC}"

mutant_rejected "git-cliff empty scan" \
  'git-cliff: the scan found no pin site' \
  '!GIT_CLIFF_VERSION:' Taskfile.yml \
  bash -c 'sed -i -E "s/^([[:space:]]*)GIT_CLIFF_VERSION:/\1GC_VERSION:/" Taskfile.yml &&
    sed -i -E "s#orhun/git-cliff-action@#example/cliff-action@#" '"${REL}"' &&
    sed -i -E "s/install-git-cliff\.sh v/install-cliff.sh v/" '"${VC}"' &&
    sed -i -E "s#orhun/git-cliff-action#example/cliff-action#; s#install-git-cliff#install-cliff#" renovate.json'
# (DTP-4: renovate.json follows the rename, so its git-cliff manager stays alive and only the
# scan clause trips.)

mutant_rejected "git-cliff-action step without a version input" \
  "git-cliff: \\.github/workflows/release\\.yaml:[0-9]+ says '<no version: input>', not a plain release version" \
  "!^[[:space:]]+version: ${GC_RE}" "${REL}" \
  sed -i -E '/orhun\/git-cliff-action@/,/version:/ { /^[[:space:]]+version: v[0-9.]+/d }' "${REL}"

# env: goes before and after the with: block, so neither a first-wins nor a last-wins reader passes.
# An env: between uses: and with: also hides the input from renovate.json's git-cliff manager,
# whose matchString expects with: next (DTP-4).
ALSO_RED='renovate: git-cliff pin v2\.12\.0 at \.github/workflows/release\.yaml:[0-9]+ is extracted by no custom manager' \
mutant_rejected "git-cliff env version masks a drifted input" \
  'git-cliff: \.github/workflows/release\.yaml:[0-9]+ says v2\.12\.0, but the reference site Taskfile\.yml' \
  "version: ${GC_RE} # mutant-env-before@@version: ${GC_RE} # mutant-env-after@@version: v2\\.12\\.0" "${REL}" \
  bash -c 'sed -i -E "/orhun\/git-cliff-action@/,/version:/ s/version: v[0-9.]+/version: v2.12.0/" '"${REL}"' &&
    sed -i -E "/orhun\/git-cliff-action@/{n;s/^([[:space:]]+)with:\$/\1env:\n\1  version: '"${GC_CUR}"' # mutant-env-before\n\1with:/}" '"${REL}"' &&
    sed -i -E "/orhun\/git-cliff-action@/,/github_token:/ s/^([[:space:]]+github_token:.*)\$/\1\n        env:\n          version: '"${GC_CUR}"' # mutant-env-after/" '"${REL}"

# Key order: the with: block before uses: is the same step and the same input, so DTP-3 raises
# nothing; renovate.json's manager expects with: after uses:, so DTP-4 is the only clause that reds.
mutant_rejected "git-cliff with: before uses:" \
  "renovate: git-cliff pin ${GC_RE} at \\.github/workflows/release\\.yaml:[0-9]+ is extracted by no custom manager" \
  'orhun/git-cliff-action@' "${REL}" \
  bash -c 'awk "/uses: orhun\/git-cliff-action@/ { u = \$0; next } u != \"\" && /github_token:/ { print; print u; u = \"\"; next } { print }" '"${REL}"' > m.yaml && mv m.yaml '"${REL}"' &&
    awk "p ~ /github_token:/ && /uses: orhun\/git-cliff-action@/ { ok = 1 } { p = \$0 } END { exit !ok }" '"${REL}"

# --- DTP-3 shellcheck ---
# Mutant values come from the tree, so a correct shellcheck bump (all sites plus digests) does not
# break the self-test: 99.0.0 is never a real release, the control adds a packaging revision.
SC_CUR="$(sed -n -E 's/.*\$\{SHELLCHECK_VERSION:-([0-9]+\.[0-9]+\.[0-9]+)\}.*/\1/p' "${ROOT}/hack/install-shellcheck.sh" | head -1)"
[[ -n "${SC_CUR}" ]] || fail "self-test: cannot read the shellcheck version from hack/install-shellcheck.sh -- run the plain mode first"
SI=hack/install-shellcheck.sh
mutant_rejected "shellcheck ci.yaml drifts" \
  'shellcheck: \.github/workflows/ci\.yaml:[0-9]+ says 99\.0\.0, but the reference site hack/install-shellcheck\.sh' \
  'SHELLCHECK_VERSION: "99\.0\.0"' "${CI}" \
  sed -i -E 's/^([[:space:]]*SHELLCHECK_VERSION: )"[0-9.]+"/\1"99.0.0"/' "${CI}"

mutant_rejected "shellcheck bumped without digests" \
  'shellcheck: hack/install-shellcheck\.sh pinned_sha256 has no entry for shellcheck-v99\.0\.0\.(linux|darwin)\.(x86_64|aarch64)\.tar\.xz' \
  'rev: v99\.0\.0\.0' "${PC}" \
  bash -c 'sed -i -E "s/^([[:space:]]*SHELLCHECK_VERSION: )\"[0-9.]+\"/\1\"99.0.0\"/" '"${CI}"' &&
    sed -i -E "s/(SHELLCHECK_VERSION:-)[0-9.]+/\199.0.0/" '"${SI}"' &&
    sed -i -E "/shellcheck-py\/shellcheck-py/{n;s/rev: v[0-9.]+/rev: v99.0.0.0/}" '"${PC}"

control_passes "shellcheck-py packaging revision v${SC_CUR}.99" \
  "rev: v${SC_CUR//./\\.}\\.99" "${PC}" \
  sed -i -E "/shellcheck-py\/shellcheck-py/{n;s/rev: v[0-9.]+/rev: v${SC_CUR}.99/}" "${PC}"

control_passes "pre-commit repo URLs quoted" \
  'repo: "https://github\.com/gitleaks/gitleaks"@@repo: "https://github\.com/shellcheck-py/shellcheck-py"' "${PC}" \
  sed -i -E 's#^([[:space:]]*-[[:space:]]*repo:[[:space:]]*)(https://[^[:space:]]+)#\1"\2"#' "${PC}"

mutant_rejected "shellcheck empty scan" \
  'shellcheck: the scan found no pin site' \
  '!SHELLCHECK_VERSION' "${CI}" \
  bash -c 'sed -i -E "s/SHELLCHECK_VERSION:/SC_VERSION:/" '"${CI}"' &&
    sed -i -E "s/SHELLCHECK_VERSION:-/SC_VERSION:-/" '"${SI}"' &&
    sed -i -E "s#github.com/shellcheck-py/shellcheck-py#github.com/example/sc#" '"${PC}"' &&
    sed -i -E "s#com/shellcheck-py/shellcheck-py#com/example/sc#" renovate.json'
# (DTP-4: renovate.json follows the rename, so its shellcheck-py manager stays alive.)

mutant_rejected "shellcheck platforms unreadable" \
  'shellcheck: cannot read the supported platforms of hack/install-shellcheck\.sh' \
  'OSNAME="linux"' "${SI}" \
  sed -i -E 's/\)[[:space:]]*OS="([a-z]+)"/) OSNAME="\1"/' "${SI}"

# --- DTP-6 exceptions list ---
FG="${FETCH_GATE}"
mutant_rejected "UNVERIFIED_BY_DESIGN gains an entry" \
  'DTP-6: UNVERIFIED_BY_DESIGN lists install-kind\.sh, which is not in this guard.s limit' \
  '^UNVERIFIED_BY_DESIGN=\(.*"install-kind\.sh"\)$' "${FG}" \
  sed -i -E 's/^(UNVERIFIED_BY_DESIGN=\(.*)\)$/\1 "install-kind.sh")/' "${FG}"

mutant_rejected "UNVERIFIED_BY_DESIGN definition removed" \
  'DTP-6: hack/test/ci_fetch_lib_hardening_test\.sh has no single-line UNVERIFIED_BY_DESIGN=\( \.\.\. \) definition' \
  '!^UNVERIFIED_BY_DESIGN=' "${FG}" \
  sed -i -E '/^UNVERIFIED_BY_DESIGN=\(/d' "${FG}"

mutant_rejected "UNVERIFIED_BY_DESIGN appended to" \
  'DTP-6: UNVERIFIED_BY_DESIGN is written 2 times' \
  '^UNVERIFIED_BY_DESIGN\+=' "${FG}" \
  sed -i -E '/^UNVERIFIED_BY_DESIGN=\(/a UNVERIFIED_BY_DESIGN+=("install-kind.sh")' "${FG}"

mutant_rejected "UNVERIFIED_BY_DESIGN gains an indexed entry" \
  'DTP-6: UNVERIFIED_BY_DESIGN is written 2 times' \
  '^UNVERIFIED_BY_DESIGN\[2\]=' "${FG}" \
  sed -i -E '/^UNVERIFIED_BY_DESIGN=\(/a UNVERIFIED_BY_DESIGN[2]="install-kind.sh"' "${FG}"

mutant_rejected "UNVERIFIED_BY_DESIGN filled by mapfile" \
  'DTP-6: UNVERIFIED_BY_DESIGN is written 2 times' \
  '^mapfile -t UNVERIFIED_BY_DESIGN' "${FG}" \
  sed -i -E '/^UNVERIFIED_BY_DESIGN=\(/a mapfile -t UNVERIFIED_BY_DESIGN < <(printf "%s\\n" install-kind.sh)' "${FG}"

mutant_rejected "UNVERIFIED_BY_DESIGN appended by length index" \
  'DTP-6: UNVERIFIED_BY_DESIGN is written 2 times' \
  '^UNVERIFIED_BY_DESIGN\[\$\{#UNVERIFIED_BY_DESIGN' "${FG}" \
  sed -i -E '/^UNVERIFIED_BY_DESIGN=\(/a UNVERIFIED_BY_DESIGN[${#UNVERIFIED_BY_DESIGN[@]}]="install-kind.sh"' "${FG}"

mutant_rejected "UNVERIFIED_BY_DESIGN written by printf -v" \
  'DTP-6: UNVERIFIED_BY_DESIGN is written 2 times' \
  "^printf -v 'UNVERIFIED_BY_DESIGN" "${FG}" \
  sed -i -E "/^UNVERIFIED_BY_DESIGN=\\(/a printf -v 'UNVERIFIED_BY_DESIGN[2]' '%s' install-kind.sh" "${FG}"

# The bare name between spaces is a write unless it sits inside a double-quoted string.
mutant_rejected "UNVERIFIED_BY_DESIGN written by printf -v with the bare name" \
  'DTP-6: UNVERIFIED_BY_DESIGN is written 2 times' \
  "^printf -v UNVERIFIED_BY_DESIGN '" "${FG}" \
  sed -i -E "/^UNVERIFIED_BY_DESIGN=\\(/a printf -v UNVERIFIED_BY_DESIGN '%s' install-kind.sh" "${FG}"

# Reads and messages are not writes: a loop over the list and a message naming it pass.
control_passes "UNVERIFIED_BY_DESIGN read in a loop and named in a message" \
  'listed in UNVERIFIED_BY_DESIGN here' "${FG}" \
  sed -i -E '/^UNVERIFIED_BY_DESIGN=\(/a for k in "${UNVERIFIED_BY_DESIGN[@]}"; do echo "${k} listed in UNVERIFIED_BY_DESIGN here (${#UNVERIFIED_BY_DESIGN[@]})"; done' "${FG}"

# The ratchet holds once the list is empty: the limit is empty too, so re-adding an installer reds.
mutant_rejected "install-git-cliff.sh re-added to the emptied UNVERIFIED_BY_DESIGN" \
  'DTP-6: UNVERIFIED_BY_DESIGN lists install-git-cliff\.sh, which is not in this guard.s limit \(empty\)' \
  '^UNVERIFIED_BY_DESIGN=\("install-git-cliff\.sh"\)$' "${FG}" \
  sed -i -E 's/^UNVERIFIED_BY_DESIGN=\(\)$/UNVERIFIED_BY_DESIGN=("install-git-cliff.sh")/' "${FG}"

# The checksum scan of the hardening gate (it owns the scan) sees each installer's verifier: delete
# it from a copy and that gate reds naming the installer. The step fails if the verifier is absent,
# so the case cannot pass on an installer that never had one.
GATE="${FETCH_GATE}"
mutant_rejected "sha512sum deleted from install-git-cliff.sh" \
  'hack/install-git-cliff\.sh no longer verifies a checksum' \
  '!sha512sum' hack/install-git-cliff.sh \
  bash -c 'grep -q sha512sum hack/install-git-cliff.sh && sed -i "/sha512sum/d" hack/install-git-cliff.sh'

mutant_rejected "verify_sha256 deleted from install-helm-docs.sh" \
  'hack/install-helm-docs\.sh no longer verifies a checksum' \
  '!^verify_sha256 ' hack/install-helm-docs.sh \
  bash -c 'grep -q "^verify_sha256 " hack/install-helm-docs.sh && sed -i "/^verify_sha256 /d" hack/install-helm-docs.sh'
GATE="${SELF}"

# --- DTP-6 installer execution: valid, tampered and missing-checksum downloads ---
# Each run uses a scratch root holding only what the installer sources: the installer under test,
# the real hack/lib/verify-sha256.sh and a fake hack/lib/fetch.sh whose fetch_to copies the fixture
# named by the URL's basename (logging each URL) or fails like the real one. uname is faked to
# Linux x86_64, so the asset names are fixed on any host. Version 99.0.0 is a fixture, never a
# release. Every case runs; the section fails at its end if any case did.
INST_BROKEN=0
inst_err() {
  echo "FAIL: $*" >&2
  INST_BROKEN=1
}

INST_VERSION="99.0.0"
# inst_meta <tool>: sets INST_ASSET, INST_TARBALL_URL, INST_SUM_FILE and INST_SUM_URL.
inst_meta() {
  case "$1" in
    git-cliff)
      INST_ASSET="git-cliff-${INST_VERSION}-x86_64-unknown-linux-gnu.tar.gz"
      INST_TARBALL_URL="https://github.com/orhun/git-cliff/releases/download/v${INST_VERSION}/${INST_ASSET}"
      INST_SUM_FILE="${INST_ASSET}.sha512"
      INST_SUM_URL="${INST_TARBALL_URL}.sha512"
      ;;
    helm-docs)
      INST_ASSET="helm-docs_${INST_VERSION}_Linux_x86_64.tar.gz"
      INST_TARBALL_URL="https://github.com/norwoodj/helm-docs/releases/download/v${INST_VERSION}/${INST_ASSET}"
      INST_SUM_FILE="checksums.txt"
      INST_SUM_URL="https://github.com/norwoodj/helm-docs/releases/download/v${INST_VERSION}/checksums.txt"
      ;;
  esac
}

# inst_fixtures <tool> <dir>: a valid archive in the release layout and its checksum file in the
# published format (probed upstream in task 4.1): git-cliff's per-asset `<sha512>  <asset>`, and
# helm-docs' checksums.txt of `<sha256>  <asset>` lines, here with a decoy line for another asset
# first. gzip -n zeroes the header's MTIME, which the tampered case flips.
inst_fixtures() {
  local tool="$1" dir="$2" stage="$2/stage" bin
  inst_meta "${tool}"
  case "${tool}" in
    git-cliff) bin="${stage}/git-cliff-${INST_VERSION}/git-cliff" ;;
    helm-docs) bin="${stage}/helm-docs" ;;
  esac
  mkdir -p "$(dirname "${bin}")"
  printf '#!/bin/sh\necho "%s fixture %s"\n' "${tool}" "${INST_VERSION}" >"${bin}"
  chmod 0755 "${bin}"
  tar -cf - -C "${stage}" . | gzip -n >"${dir}/${INST_ASSET}"
  case "${tool}" in
    git-cliff) (cd "${dir}" && sha512sum "${INST_ASSET}" >"${INST_SUM_FILE}") ;;
    helm-docs)
      {
        printf '%s  helm-docs_%s_Darwin_arm64.tar.gz\n' "$(printf decoy | sha256sum | awk '{print $1}')" "${INST_VERSION}"
        (cd "${dir}" && sha256sum "${INST_ASSET}")
      } >"${dir}/${INST_SUM_FILE}"
      ;;
  esac
}

# inst_run <tool> <installer> <scenario>: run a copy of <installer> in a fresh scratch root. Sets
# INST_ROOT, INST_RC, INST_OUT (combined output) and INST_LOG (the URLs fetched, in order).
INST_ROOT=""
inst_run() {
  local tool="$1" src="$2" scenario="$3" fx
  if [[ -n "${INST_ROOT}" ]]; then rm -rf "${INST_ROOT:?}"; fi
  INST_ROOT="$(mktemp -d "${WORKROOT}/inst.XXXXXXXX")"
  fx="${INST_ROOT}/fixtures"
  mkdir -p "${INST_ROOT}/hack/lib" "${INST_ROOT}/fakebin" "${fx}"
  cp "${src}" "${INST_ROOT}/hack/install-${tool}.sh"
  cp "${ROOT}/hack/lib/verify-sha256.sh" "${INST_ROOT}/hack/lib/verify-sha256.sh"
  cat >"${INST_ROOT}/hack/lib/fetch.sh" <<'EOF'
# shellcheck shell=bash
fetch_to() {
  local src="${FAKE_FETCH_DIR}/${1##*/}"
  echo "$1" >>"${FAKE_FETCH_LOG}"
  if [[ -f "${src}" ]]; then
    cp "${src}" "$2"
    return 0
  fi
  rm -f "$2"
  echo "failed to download ${3:-$1} from $1" >&2
  return 1
}
EOF
  printf '#!/bin/sh\ncase "$1" in -m) echo x86_64 ;; -s) echo Linux ;; *) exit 2 ;; esac\n' >"${INST_ROOT}/fakebin/uname"
  chmod 0755 "${INST_ROOT}/fakebin/uname"
  inst_fixtures "${tool}" "${fx}"
  case "${scenario}" in
    valid) ;;
    tampered)
      # One byte of the gzip MTIME: the archive still extracts, so only a checksum can catch it.
      printf '\001' | dd of="${fx}/${INST_ASSET}" bs=1 seek=4 count=1 conv=notrunc 2>/dev/null
      tar -tzf "${fx}/${INST_ASSET}" >/dev/null ||
        fail "self-test: the tampered ${tool} fixture no longer extracts -- the case would pass on tar, not on the checksum"
      ;;
    missing-checksum) rm -f "${fx:?}/${INST_SUM_FILE}" ;;
    other-asset)
      # The right digest, filed under another asset's name: only the name check stands in the way.
      sed -i "s/  ${INST_ASSET//./\\.}\$/  other-${INST_ASSET}/" "${fx}/${INST_SUM_FILE}"
      grep -q "  other-${INST_ASSET}\$" "${fx}/${INST_SUM_FILE}" ||
        fail "self-test: the other-asset ${tool} fixture did not rename its checksum line"
      ;;
  esac
  : >"${INST_ROOT}/fetch.log"
  INST_OUT="$(cd "${INST_ROOT}" && PATH="${INST_ROOT}/fakebin:${PATH}" FAKE_FETCH_DIR="${fx}" \
    FAKE_FETCH_LOG="${INST_ROOT}/fetch.log" bash "hack/install-${tool}.sh" "v${INST_VERSION}" "bin/${tool}" 2>&1)" &&
    INST_RC=0 || INST_RC=$?
  INST_LOG="$(cat "${INST_ROOT}/fetch.log")"
}

# inst_refused <tool> <scenario> <diagnostic-ERE>: the last run failed, printed the diagnostic and
# installed nothing. Prints why not and returns 1 otherwise.
inst_refused() {
  local tool="$1" scenario="$2" diag="$3"
  if [[ "${INST_RC}" -eq 0 ]]; then
    echo "the ${scenario} install of ${tool} exited 0"
    return 1
  fi
  if [[ -e "${INST_ROOT}/bin/${tool}" ]]; then
    echo "the ${scenario} install of ${tool} left bin/${tool} behind"
    return 1
  fi
  if ! grep -Eq -- "${diag}" <<<"${INST_OUT}"; then
    echo "the ${scenario} install of ${tool} failed without '${diag}': $(tail -2 <<<"${INST_OUT}" | tr '\n' ' ')"
    return 1
  fi
}

# inst_cases <tool> <sed script that removes the installer's verification>
inst_cases() {
  local tool="$1" unverify="$2" src="${ROOT}/hack/install-$1.sh" why expected stripped
  inst_meta "${tool}"
  # Valid first: an installer that always fails would pass every negative case below.
  inst_run "${tool}" "${src}" valid
  expected="$(printf '%s\n%s' "${INST_SUM_URL}" "${INST_TARBALL_URL}")"
  if [[ "${INST_RC}" -ne 0 ]]; then
    inst_err "DTP-6 ${tool}: a valid archive with a matching checksum did not install (exit ${INST_RC}): $(tail -2 <<<"${INST_OUT}" | tr '\n' ' ')"
  elif [[ ! -x "${INST_ROOT}/bin/${tool}" || "$("${INST_ROOT}/bin/${tool}")" != "${tool} fixture ${INST_VERSION}" ]]; then
    inst_err "DTP-6 ${tool}: the valid install exited 0 but bin/${tool} is not the fixture's executable"
  elif [[ "${INST_LOG}" != "${expected}" ]]; then
    inst_err "DTP-6 ${tool}: the valid install fetched [$(tr '\n' ' ' <<<"${INST_LOG}")], expected the checksum file then the archive [$(tr '\n' ' ' <<<"${expected}")]"
  else
    pass "self-test: DTP-6 ${tool} installs a valid archive"
  fi

  inst_run "${tool}" "${src}" tampered
  if why="$(inst_refused "${tool}" tampered 'checksum mismatch')"; then
    pass "self-test: DTP-6 ${tool} refuses a tampered archive"
  else
    inst_err "DTP-6 ${tool}: ${why}"
  fi

  inst_run "${tool}" "${src}" missing-checksum
  if why="$(inst_refused "${tool}" missing-checksum "failed to download .* from ${INST_SUM_URL//./\\.}")"; then
    pass "self-test: DTP-6 ${tool} refuses to install when the checksum file cannot be fetched"
  else
    inst_err "DTP-6 ${tool}: ${why}"
  fi

  inst_run "${tool}" "${src}" other-asset
  if why="$(inst_refused "${tool}" other-asset 'failed to resolve a (sha512|checksum) for ')"; then
    pass "self-test: DTP-6 ${tool} refuses a checksum file with no line for its asset"
  else
    inst_err "DTP-6 ${tool}: ${why}"
  fi

  # The tampered case can see a missing check: on a copy without the verification it must fail.
  stripped="${WORKROOT}/install-${tool}-unverified.sh"
  sed -E "${unverify}" "${src}" >"${stripped}"
  if cmp -s "${src}" "${stripped}"; then
    inst_err "DTP-6 ${tool}: removing the verification ('${unverify}') changed nothing -- the installer has no verification to remove"
  else
    inst_run "${tool}" "${stripped}" tampered
    if inst_refused "${tool}" tampered 'checksum mismatch' >/dev/null; then
      inst_err "DTP-6 ${tool}: the tampered case passed on a copy with its verification removed -- it cannot see a missing check"
    else
      pass "self-test: DTP-6 ${tool} tampered case fails on a copy without verification"
    fi
  fi
}

inst_cases git-cliff '/^actual="\$\(sha512sum /,/^fi$/d'
inst_cases helm-docs '/^verify_sha256 /d'
if [[ -n "${INST_ROOT}" ]]; then rm -rf "${INST_ROOT:?}"; fi
((INST_BROKEN == 0)) || fail "self-test: DTP-6 installer execution cases failed (listed above)"

# --- DEV-MISE-01, ported one-to-one from the former mise pin-drift guard (its `honest` fixture is the
# no-op control above). Old values kept: each is older than the real pin, never a next release.
M=mise.toml
# The task comparison is DTP-1's since 2.1, so this case reds with its message.
mutant_rejected "mise task drifts from CI (old task-drift)" \
  'task: mise\.toml:[0-9]+ says 3\.50\.0, but the reference site \.github/workflows/ci\.yaml:[0-9]+ says' \
  '^task = "3\.50\.0"' "${M}" \
  sed -i -E 's/^task = "[0-9.]+"/task = "3.50.0"/' "${M}"

# The attached comment moves with the pin (DTP-5 would red on it otherwise): only CI disagrees.
mutant_rejected "mise node drifts from CI (old node-drift)" \
  'mise: mise\.toml pins node = 20 but the actions/setup-node node-version: inputs pins [0-9]+' \
  '^node = "20"@@^#.*`node-version: "20"`' "${M}" \
  sed -i -E 's/^node = "[0-9.]+"/node = "20"/; s/^(#.*`node-version: )"[0-9.]+"/\1"20"/' "${M}"

mutant_rejected "mise python drifts from CI (old python-drift)" \
  'mise: mise\.toml pins python = 3\.11 but the actions/setup-python python-version: inputs pins [0-9.]+' \
  '^python = "3\.11"@@^#.*`python-version: "3\.11"`' "${M}" \
  sed -i -E 's/^python = "[0-9.]+"/python = "3.11"/; s/^(#.*`python-version: )"[0-9.]+"/\1"3.11"/' "${M}"

mutant_rejected "mise pins go under [tools] (old go-pinned)" \
  'mise: mise\.toml pins go = 1\.26\.6 under \[tools\]' \
  '^go = "1\.26\.6"$' "${M}" \
  sed -i -E 's/^\[tools\]$/[tools]\ngo = "1.26.6"/' "${M}"

mutant_rejected "mise stops reading go.mod (old go-setting-gone)" \
  'mise: mise\.toml no longer enables the idiomatic version file for go' \
  '!idiomatic_version_file_enable_tools' "${M}" \
  sed -i -E '/^[[:space:]]*idiomatic_version_file_enable_tools[[:space:]]*=/d' "${M}"

mutant_rejected "go.mod loses its go line (old gomod-gone)" \
  "mise: go\\.mod has no 'go <version>' line" \
  '!^go[[:space:]]+[0-9]' go.mod \
  sed -i -E '/^go[[:space:]]+[0-9]/d' go.mod

mutant_rejected "composite action disagrees with the workflows on task (old ci-self-drift)" \
  'task: \.github/actions/kind-e2e-setup/action\.yml:[0-9]+ says 3\.50\.0, but the reference site \.github/workflows/ci\.yaml' \
  '^[[:space:]]+version: 3\.50\.0$' .github/actions/kind-e2e-setup/action.yml \
  sed -i -E '/go-task\/setup-task@/,/version:/ s/version: [0-9.]+$/version: 3.50.0/' .github/actions/kind-e2e-setup/action.yml

# Every node/python pin under .github renamed away; the in-command grep proves none is left. The
# task half of the old case is DTP-1's empty-scan case below.
mutant_rejected "no node/python pin in CI (old no-ci-pins)" \
  'mise: the scan set is empty -- found none of the actions/setup-(node|python) ' \
  '!node-version:@@!python-version:' .github/workflows/docs.yaml \
  bash -c 'grep -rlE "(node|python)-version:" .github | xargs sed -i -E "s/(node|python)-version:/\1-ver:/" &&
    ! grep -rqE "(node|python)-version:" .github'

# The comments above the entries go too: left behind, they restate versions away from a pin (DTP-5).
mutant_rejected "mise [tools] table empty (old empty-tools)" \
  'mise: mise\.toml has no (task|node|python) entry under \[tools\]' \
  '^\[tools\]$@@!^(task|node|python)[[:space:]]*=' "${M}" \
  sed -i -E '/^\[tools\]$/,$ { /^[[:space:]]*[a-z]+[[:space:]]*=/d; /^[[:space:]]*#/d }' "${M}"

# Without the header the entries are no pins, so their comments go with it (as in empty-tools).
mutant_rejected "mise has no [tools] table (old no-tools-table)" \
  'mise: mise\.toml has no \[tools\] table' \
  '!^[[:space:]]*\[tools\]' "${M}" \
  sed -i -E '/^\[tools\]$/,$ { /^[[:space:]]*#/d }; /^\[tools\]$/d' "${M}"

# Not in the old guard (its section scope was untested): a task key outside [tools] is not a pin.
mutant_rejected "mise task moved from [tools] to [settings]" \
  'mise: mise\.toml has no task entry under \[tools\]' \
  '^\[tools\]$' "${M}" \
  bash -c 't="$(grep -E "^task = " mise.toml)" && sed -i -E "/^task = /d" mise.toml &&
    sed -i -E "s/^\[settings\]\$/[settings]\n${t}/" mise.toml &&
    awk "/^\[settings\]\$/ { s = 1 } /^\[tools\]\$/ { s = 0 } s && /^task = / { ok = 1 } END { exit !ok }" mise.toml'

# The file is gone: the in-command test is the absence probe (a probe of a missing file is vacuous).
# renovate.json's go-task manager names mise.toml, so its pattern rots with it (DTP-4).
ALSO_RED='renovate: customManagers\[[0-9]+\] "go-task versions[^"]*" \(renovate\.json:[0-9]+\): file pattern /\^mise\\\.toml\$/ matches no tracked file' \
mutant_rejected "mise.toml missing (old missing mise.toml case)" \
  'mise: mise\.toml does not exist' \
  '!.' "${M}" \
  bash -c 'rm -f mise.toml && test ! -e mise.toml'

# --- DTP-5 comments ---
# Pin values come from the tree, so a correct bump of a pin and its attached comment stays green.
NODE_CUR="$(mise_tool_value "${ROOT}/${M}" node)"
PY_CUR="$(mise_tool_value "${ROOT}/${M}" python)"
HELM_CUR="$(sed -n -E 's/^  HELM_VERSION: (v[0-9.]+)$/\1/p' "${ROOT}/Taskfile.yml" | head -1)"
[[ -n "${NODE_CUR}" && -n "${PY_CUR}" && -n "${HELM_CUR}" ]] ||
  fail "self-test: cannot read the node, python or helm pin from mise.toml / Taskfile.yml -- run the plain mode first"

mutant_rejected "mise comment restates helm (v3.21.4)" \
  'DTP-5: mise\.toml:2 comment restates helm 3\.21\.4 away from its pin' \
  '^# helm \(v3\.21\.4\) -- Taskfile\.yml HELM_VERSION$' "${M}" \
  sed -i -E '1a # helm (v3.21.4) -- Taskfile.yml HELM_VERSION' "${M}"

# Attached to a key that is not a pin ([settings]): still away from any pin.
mutant_rejected "mise comment restates go 1.27.2" \
  'DTP-5: mise\.toml:[0-9]+ comment restates go 1\.27\.2 away from its pin' \
  '^# currently go 1\.27\.2$' "${M}" \
  sed -i -E 's/^idiomatic_version_file_enable_tools/# currently go 1.27.2\n&/' "${M}"

# Directly above the task pin: a count reds even where a version would be allowed.
mutant_rejected "mise comment counts pin sites" \
  'DTP-5: mise\.toml:[0-9]+ comment counts pin sites \("17 occurrences"\)' \
  '^# unanimous across all 17 occurrences in \.github/workflows/$' "${M}" \
  sed -i -E 's#^task = #\# unanimous across all 17 occurrences in .github/workflows/\n&#' "${M}"

mutant_rejected "mise attached comment gone stale" \
  'DTP-5: mise\.toml:[0-9]+ comment attached to task = [0-9.]+ says version 3\.51\.1, not [0-9.]+ -- an attached comment gone stale' \
  '^# Matches `version: 3\.51\.1`\.$' "${M}" \
  sed -i -E 's/^task = /# Matches `version: 3.51.1`.\n&/' "${M}"

# The blank line detaches the whole Node block; the in-command awk proves it sits there.
mutant_rejected "mise node comment detached by a blank line" \
  "DTP-5: mise\\.toml:[0-9]+ comment restates node-version ${NODE_CUR//./\\.} away from its pin" \
  "node-version: \"${NODE_CUR//./\\.}\"" "${M}" \
  bash -c 'sed -i -E "s/^node = /\n&/" mise.toml &&
    awk "p ~ /^#/ { c = 1 } p == \"\" && c && /^node = / { ok = 1 } { p = \$0 } END { exit !ok }" mise.toml'

mutant_rejected "mise node comment names another tool" \
  "DTP-5: mise\\.toml:[0-9]+ comment attached to node = ${NODE_CUR//./\\.} names gitleaks 8\\.30\\.1, which is not that pin.s tool" \
  '^# gitleaks 8\.30\.1$' "${M}" \
  sed -i -E 's/^node = /# gitleaks 8.30.1\n&/' "${M}"

mutant_rejected "mise inline comment on a key that is not a pin" \
  'DTP-5: mise\.toml:[0-9]+ comment restates go 1\.27\.2 away from its pin' \
  '^idiomatic_version_file_enable_tools = .* # go 1\.27\.2$' "${M}" \
  sed -i -E 's/^(idiomatic_version_file_enable_tools = .*)$/\1 # go 1.27.2/' "${M}"

# Review round 1: a bare number after a pin name, a pre-release suffix, a matching value under the
# wrong name, and a count in another case.
TASK_CUR="$(mise_tool_value "${ROOT}/${M}" task)"
mutant_rejected "mise comment restates a bare node major" \
  'DTP-5: mise\.toml:2 comment restates node 20 away from its pin' \
  '^# Contributors want node 20 here\.$' "${M}" \
  sed -i -E '1a # Contributors want node 20 here.' "${M}"

mutant_rejected "mise attached comment with a pre-release suffix" \
  "DTP-5: mise\\.toml:[0-9]+ comment attached to task = ${TASK_CUR//./\\.} says version ${TASK_CUR//./\\.}-rc1, not ${TASK_CUR//./\\.}" \
  "^# Matches \`version: ${TASK_CUR//./\\.}-rc1\`\$" "${M}" \
  sed -i -E "s/^task = /# Matches \`version: ${TASK_CUR}-rc1\`\\n&/" "${M}"

mutant_rejected "mise node comment names another tool at the pin's value" \
  "DTP-5: mise\\.toml:[0-9]+ comment attached to node = ${NODE_CUR//./\\.} names gitleaks ${NODE_CUR//./\\.}, which is not that pin.s tool" \
  "^# gitleaks \"${NODE_CUR//./\\.}\"\$" "${M}" \
  sed -i -E "s/^node = /# gitleaks \"${NODE_CUR}\"\\n&/" "${M}"

mutant_rejected "mise comment counts pin sites in capitals" \
  'DTP-5: mise\.toml:2 comment counts pin sites \("6 workflows"\)' \
  '^# Pinned in 6 Workflows\.$' "${M}" \
  sed -i -E '1a # Pinned in 6 Workflows.' "${M}"

# The real Node and Python blocks: a correct attached comment is not a finding.
control_passes "mise Node and Python attached comments" \
  "node-version: \"${NODE_CUR//./\\.}\"@@python-version: \"${PY_CUR//./\\.}\"" "${M}" \
  true

control_passes "mise inline comment naming its pin" \
  "^node = \"${NODE_CUR//./\\.}\" # node-version: \"${NODE_CUR//./\\.}\"\$" "${M}" \
  sed -i -E "s/^node = \"[0-9.]+\"\$/node = \"${NODE_CUR}\" # node-version: \"${NODE_CUR}\"/" "${M}"

# Taskfile.yml: the comment joins the run of comments above HELM_VERSION (above its annotation).
control_passes "Taskfile comment attached to HELM_VERSION" \
  "^  # helm ${HELM_CUR//./\\.}\$" Taskfile.yml \
  sed -i -E "s#^  \\# renovate: datasource=github-releases depName=helm/helm\$#  \\# helm ${HELM_CUR}\\n&#" Taskfile.yml

mutant_rejected "Taskfile comment restates helm away from its pin" \
  'DTP-5: Taskfile\.yml:1 comment restates helm 3\.21\.4 away from its pin' \
  '^# helm v3\.21\.4$' Taskfile.yml \
  sed -i -E '1i # helm v3.21.4' Taskfile.yml

mutant_rejected "Taskfile helm comment attached to GIT_CLIFF_VERSION" \
  'DTP-5: Taskfile\.yml:[0-9]+ comment attached to GIT_CLIFF_VERSION = [0-9.]+ names helm 3\.21\.4, which is not that pin.s tool' \
  '^  # helm v3\.21\.4$' Taskfile.yml \
  sed -i -E 's#^  \# renovate: datasource=github-releases depName=orhun/git-cliff$#  \# helm v3.21.4\n&#' Taskfile.yml

mutant_rejected "Taskfile helm comment attached to HELM_VERSION gone stale" \
  'DTP-5: Taskfile\.yml:[0-9]+ comment attached to HELM_VERSION = [0-9.]+ says helm 3\.21\.4, not [0-9.]+ -- an attached comment gone stale' \
  '^  # helm v3\.21\.4$' Taskfile.yml \
  sed -i -E 's#^  \# renovate: datasource=github-releases depName=helm/helm$#  \# helm v3.21.4\n&#' Taskfile.yml

# --- DTP-1 Task ---
mutant_rejected "task one workflow site missed" \
  'task: \.github/workflows/sonarcloud\.yaml:[0-9]+ says 3\.51\.1, but the reference site \.github/workflows/ci\.yaml:[0-9]+ says' \
  '^[[:space:]]+version: 3\.51\.1$' .github/workflows/sonarcloud.yaml \
  sed -i -E '/go-task\/setup-task@/,/version:/ s/version: [0-9.]+$/version: 3.51.1/' .github/workflows/sonarcloud.yaml

# The whole set, mise.toml included; the sentinel only fires when every site agrees.
mutant_rejected "task set below floor" \
  'task: every site says 3\.51\.1, below the floor 3\.52\.0' \
  '^task = "3\.51\.1"$' "${M}" \
  bash -c 'grep -rl "go-task/setup-task@" .github | xargs sed -i -E "/go-task\/setup-task@/,/version:/ s/version: [0-9.]+\$/version: 3.51.1/" &&
    sed -i -E "s/^task = \"[0-9.]+\"/task = \"3.51.1\"/" mise.toml'

# A bot bump moves every site at once; an exact-value check would block it.
control_passes "task set moved to a newer release" \
  '^task = "99\.0\.0"$' "${M}" \
  bash -c 'grep -rl "go-task/setup-task@" .github | xargs sed -i -E "/go-task\/setup-task@/,/version:/ s/version: [0-9.]+\$/version: 99.0.0/" &&
    sed -i -E "s/^task = \"[0-9.]+\"/task = \"99.0.0\"/" mise.toml'

# The step stays and installs an unpinned Task: it must red, never be skipped (1.2 step-reset-off).
mutant_rejected "setup-task step without a version input" \
  "task: \\.github/workflows/docs\\.yaml:[0-9]+ says '<no version: input>', not a plain release version" \
  'go-task/setup-task[@]@@!^[[:space:]]+version: [0-9.]+$' .github/workflows/docs.yaml \
  sed -i -E '/go-task\/setup-task@/,/version:/ { /^[[:space:]]+version: [0-9.]+$/d }' .github/workflows/docs.yaml

mutant_rejected "task named site loses its setup-task step" \
  'task: no pin found in \.github/actions/kind-e2e-setup/action\.yml' \
  '!go-task/setup-task@' .github/actions/kind-e2e-setup/action.yml \
  sed -i -E 's#go-task/setup-task@#example/setup-task@#' .github/actions/kind-e2e-setup/action.yml

mutant_rejected "task empty scan" \
  'task: the scan found no go-task/setup-task step under \.github' \
  '!go-task/setup-task@' .github/workflows/ci.yaml \
  bash -c 'grep -rl "go-task/setup-task@" .github | xargs sed -i -E "s#go-task/setup-task@#example/setup-task@#" &&
    ! grep -rq "go-task/setup-task@" .github'

# --- DTP-2 govulncheck ---
mutant_rejected "govulncheck inline pin returns" \
  'govulncheck: Taskfile\.yml:[0-9]+ task vulncheck pins golang\.org/x/vuln/cmd/govulncheck@v1\.9\.0 inline' \
  'govulncheck@v1\.9\.0' Taskfile.yml \
  sed -i -E 's#(cmd/govulncheck)@\{\{\.GOVULNCHECK_VERSION\}\}#\1@v1.9.0#' Taskfile.yml

# Without its annotation the Taskfile manager no longer extracts the pin (DTP-4 is right too).
ALSO_RED='renovate: govulncheck pin [^ ]+ at Taskfile\.yml:[0-9]+ is extracted by no custom manager' \
mutant_rejected "govulncheck annotation deleted" \
  "govulncheck: Taskfile\\.yml:[0-9]+ GOVULNCHECK_VERSION has no '# renovate: datasource=go depName=golang\\.org/x/vuln' line directly above it" \
  '!depName=golang\.org/x/vuln' Taskfile.yml \
  sed -i -E '/^[[:space:]]*# renovate: datasource=go depName=golang\.org\/x\/vuln[[:space:]]*$/d' Taskfile.yml

# A github-releases annotation still extracts (DTP-4 would pass) while every lookup fails.
# Under another depName the site is a different dependency to DTP-4 (right too).
ALSO_RED='renovate: govulncheck pin [^ ]+ at Taskfile\.yml:[0-9]+ is extracted by no custom manager' \
mutant_rejected "govulncheck annotation on the wrong datasource" \
  "govulncheck: Taskfile\\.yml:[0-9]+ GOVULNCHECK_VERSION has no '# renovate: datasource=go depName=golang\\.org/x/vuln' line directly above it" \
  'datasource=github-releases depName=golang/vuln' Taskfile.yml \
  sed -i -E 's#^([[:space:]]*\# renovate: )datasource=go depName=golang\.org/x/vuln#\1datasource=github-releases depName=golang/vuln#' Taskfile.yml

# Same value, unannotated variable: the pin Renovate bumps is not the one that runs.
mutant_rejected "vulncheck reads another variable" \
  'govulncheck: Taskfile\.yml:[0-9]+ task vulncheck runs golang\.org/x/vuln/cmd/govulncheck@\{\{\.GOVULN_PIN\}\}, not \{\{\.GOVULNCHECK_VERSION\}\}' \
  'govulncheck@\{\{\.GOVULN_PIN\}\}@@^  GOVULN_PIN: v[0-9]@@^  GOVULNCHECK_VERSION: v[0-9]' Taskfile.yml \
  sed -i -E -e 's/^(  GOVULNCHECK_VERSION: (v[0-9.]+))$/\1\n  GOVULN_PIN: \2/' \
    -e 's#(cmd/govulncheck)@\{\{\.GOVULNCHECK_VERSION\}\}#\1@{{.GOVULN_PIN}}#' Taskfile.yml

mutant_rejected "GOVULNCHECK_VERSION definition removed" \
  'govulncheck: Taskfile\.yml defines no GOVULNCHECK_VERSION under vars:' \
  '!^  GOVULNCHECK_VERSION:@@govulncheck@\{\{\.GOVULNCHECK_VERSION\}\}' Taskfile.yml \
  sed -i -E '/^  GOVULNCHECK_VERSION:/d' Taskfile.yml

# The run moves to a task appended after vulncheck: only the end of the vulncheck block keeps it out.
mutant_rejected "vulncheck runs no govulncheck" \
  'govulncheck: Taskfile\.yml task vulncheck runs no golang\.org/x/vuln/cmd/govulncheck@\{\{\.GOVULNCHECK_VERSION\}\}' \
  '^  kollect-vuln-elsewhere:' Taskfile.yml \
  bash -c 'sed -i -E "/cmd\/govulncheck@/d" Taskfile.yml && ! grep -q "cmd/govulncheck@" Taskfile.yml &&
    printf "\n  kollect-vuln-elsewhere:\n    cmds:\n      - go run golang.org/x/vuln/cmd/govulncheck@{{.GOVULNCHECK_VERSION}} ./...\n" >> Taskfile.yml'

mutant_rejected "GOVULNCHECK_VERSION set to latest" \
  "govulncheck: Taskfile\\.yml:[0-9]+ GOVULNCHECK_VERSION is 'latest', not one plain release" \
  '^  GOVULNCHECK_VERSION: latest$' Taskfile.yml \
  sed -i -E 's/^(  GOVULNCHECK_VERSION: )v[0-9.]+$/\1latest/' Taskfile.yml

# Renovate's `depName=(\S+)\n` extracts nothing when a blank follows the name.
# The trailing blank also stops the Taskfile manager's regex (DTP-4 is right too).
ALSO_RED='renovate: govulncheck pin [^ ]+ at Taskfile\.yml:[0-9]+ is extracted by no custom manager' \
mutant_rejected "govulncheck annotation with a trailing blank" \
  "govulncheck: Taskfile\\.yml:[0-9]+ GOVULNCHECK_VERSION has no '# renovate: datasource=go depName=golang\\.org/x/vuln' line directly above it" \
  'depName=golang\.org/x/vuln $' Taskfile.yml \
  sed -i -E 's#(depName=golang\.org/x/vuln)$#\1 #' Taskfile.yml

# Task runs the task-level value; CFC-2 would still smoke-test the global one.
mutant_rejected "vulncheck overrides GOVULNCHECK_VERSION" \
  'govulncheck: Taskfile\.yml:[0-9]+ task vulncheck sets its own GOVULNCHECK_VERSION' \
  '^      GOVULNCHECK_VERSION: v1\.1\.4$' Taskfile.yml \
  sed -i -E 's/^  vulncheck:$/&\n    vars:\n      GOVULNCHECK_VERSION: v1.1.4/' Taskfile.yml

# Annotation and definition moved under env:; the in-command awk proves they sit there.
mutant_rejected "GOVULNCHECK_VERSION defined under env:" \
  'govulncheck: Taskfile\.yml defines no GOVULNCHECK_VERSION under vars:' \
  '^  GOVULNCHECK_VERSION: v[0-9]' Taskfile.yml \
  bash -c 'sed -i -E "/^  # renovate: datasource=go depName=golang\.org\/x\/vuln\$/d; /^  GOVULNCHECK_VERSION: /d" Taskfile.yml &&
    sed -i -E "s/^env:\$/&\n  # renovate: datasource=go depName=golang.org\/x\/vuln\n  GOVULNCHECK_VERSION: v1.9.0/" Taskfile.yml &&
    awk "/^env:/ { e = 1; next } /^[^[:space:]#]/ { e = 0 } e && /^  GOVULNCHECK_VERSION:/ { f = 1 } END { exit !f }" Taskfile.yml'

mutant_rejected "vulncheck task gone" \
  'govulncheck: Taskfile\.yml has no vulncheck task' \
  '!^  vulncheck:' Taskfile.yml \
  sed -i -E 's/^  vulncheck:/  vulnscan:/' Taskfile.yml

# --- DTP-4 Renovate managers ---
# rj <python statements>: edit the copy's renovate.json. In scope: cfg (the parsed file), mgr(dep)
# (the one custom manager whose depNameTemplate is dep), rule(group) (the index of the rule that
# sets that groupName) and drop(dep, needle) (remove the one file pattern of mgr(dep) containing
# needle, and tag the manager so the probe sees the edit landed).
rj() {
  python3 -c '
import json, sys
cfg = json.load(open("renovate.json"))
def mgr(dep):
    ms = [m for m in cfg["customManagers"] if m.get("depNameTemplate") == dep]
    assert len(ms) == 1, "no single manager for " + dep
    return ms[0]
def rule(group):
    return [i for i, r in enumerate(cfg["packageRules"]) if r.get("groupName") == group][0]
def drop(dep, needle):
    m = mgr(dep)
    keep = [p for p in m["managerFilePatterns"] if needle not in p]
    assert len(keep) == len(m["managerFilePatterns"]) - 1, "no single pattern with " + needle
    m["managerFilePatterns"] = keep
    m["description"] += " MUTANT-DROPPED " + needle
exec(sys.argv[1])
json.dump(cfg, open("renovate.json", "w"), indent=2)
' "$1"
}
RJ=renovate.json

# Today's (base) gitleaks and shellcheck managers, verbatim: they match ci.yaml by file pattern and
# extract nothing, because CI no longer downloads by a versioned URL. A file-pattern-only check
# would call them alive.
mutant_rejected "today's gitleaks download-URL manager" \
  'renovate: customManagers\[[0-9]+\] "Pinned gitleaks release download in CI" \(renovate\.json:[0-9]+\) matches 1 file\(s\) \(\.github/workflows/ci\.yaml\) but its matchStrings extract no dependency from them -- a dead manager' \
  'Pinned gitleaks release download in CI' "${RJ}" \
  rj 'cfg["customManagers"].append({"customType": "regex", "description": "Pinned gitleaks release download in CI", "managerFilePatterns": [r"/^\.github/workflows/ci\.yaml$/"], "matchStrings": [r"releases/download/v[0-9.]+/gitleaks_(?<currentValue>[0-9.]+)_linux_x64\.tar\.gz"], "datasourceTemplate": "github-releases", "depNameTemplate": "gitleaks/gitleaks", "autoReplaceStringTemplate": "releases/download/v{{{newValue}}}/gitleaks_{{{newValue}}}_linux_x64.tar.gz"})'

mutant_rejected "today's shellcheck download-URL manager" \
  'renovate: customManagers\[[0-9]+\] "Pinned ShellCheck release download in CI" \(renovate\.json:[0-9]+\) matches 1 file\(s\) \(\.github/workflows/ci\.yaml\) but its matchStrings extract no dependency from them -- a dead manager' \
  'Pinned ShellCheck release download in CI' "${RJ}" \
  rj 'cfg["customManagers"].append({"customType": "regex", "description": "Pinned ShellCheck release download in CI", "managerFilePatterns": [r"/^\.github/workflows/ci\.yaml$/"], "matchStrings": [r"releases/download/v[0-9.]+/shellcheck-v(?<currentValue>[0-9.]+)\.linux\.x86_64\.tar\.xz"], "datasourceTemplate": "github-releases", "depNameTemplate": "koalaman/shellcheck", "autoReplaceStringTemplate": "releases/download/v{{{newValue}}}/shellcheck-v{{{newValue}}}.linux.x86_64.tar.xz"})'

mutant_rejected "manager file pattern rotted" \
  'renovate: customManagers\[[0-9]+\] "golangci-lint[^"]*" \(renovate\.json:[0-9]+\): file pattern [^ ]*no-such-gcl[^ ]* matches no tracked file -- the pattern has rotted' \
  'no-such-gcl' "${RJ}" \
  rj 'mgr("golangci/golangci-lint")["managerFilePatterns"].append(r"/^hack/tooling/\.no-such-gcl\.yml$/")'

mutant_rejected "manager whose only pattern matches no file" \
  'renovate: customManagers\[[0-9]+\] "mutant-rotted" \(renovate\.json:[0-9]+\)(: file pattern [^ ]+ matches no tracked file -- the pattern has rotted| matches no tracked file -- a dead manager)$' \
  'mutant-rotted' "${RJ}" \
  rj 'cfg["customManagers"].append({"customType": "regex", "description": "mutant-rotted", "managerFilePatterns": [r"/^no-such-dir/x\.sh$/"], "matchStrings": [r"X_VERSION=(?<currentValue>\S+)"], "datasourceTemplate": "github-releases", "depNameTemplate": "example/x"})'

# Uncovered sites, one per case: the file leaves its manager (a manager with one file is removed).
uncovered() {
  mutant_rejected "no manager covers ${1} in ${2}" \
    "renovate: ${1} pin [^ ]+ at ${2//./\\.}:[0-9]+ is extracted by no custom manager" \
    "${3}" "${RJ}" rj "${4}"
}
uncovered gitleaks .github/workflows/ci.yaml 'MUTANT-DROPPED github' 'drop("gitleaks/gitleaks", "github")'
uncovered gitleaks hack/install-gitleaks.sh 'MUTANT-DROPPED install-gitleaks' 'drop("gitleaks/gitleaks", "install-gitleaks")'
uncovered gitleaks .pre-commit-config.yaml 'MUTANT-DROPPED pre-commit' 'drop("gitleaks/gitleaks", "pre-commit")'
uncovered shellcheck .github/workflows/ci.yaml 'MUTANT-DROPPED github' 'drop("koalaman/shellcheck", "github")'
uncovered shellcheck hack/install-shellcheck.sh 'MUTANT-DROPPED install-shellcheck' 'drop("koalaman/shellcheck", "install-shellcheck")'
uncovered shellcheck .pre-commit-config.yaml '!"depNameTemplate": "shellcheck-py/shellcheck-py"' 'cfg["customManagers"].remove(mgr("shellcheck-py/shellcheck-py"))'
uncovered git-cliff .github/workflows/release.yaml 'MUTANT-DROPPED github' 'drop("orhun/git-cliff", "github")'
uncovered git-cliff hack/verify-changelog.sh 'MUTANT-DROPPED verify-changelog' 'drop("orhun/git-cliff", "verify-changelog")'
uncovered golangci-lint Makefile 'MUTANT-DROPPED Makefile' 'drop("golangci/golangci-lint", "Makefile")'
uncovered golangci-lint hack/tooling/.custom-gcl.yml 'MUTANT-DROPPED custom-gcl' 'drop("golangci/golangci-lint", "custom-gcl")'
# Sites the generic loop covers beyond the task's ten: a Task site and DTP-2's govulncheck.
uncovered task mise.toml 'MUTANT-DROPPED mise' 'drop("go-task/task", "mise")'
uncovered govulncheck Taskfile.yml 'mutant-vuln-off' 'cfg["packageRules"].insert(rule("go toolchain"), {"description": "mutant-vuln-off", "matchDepNames": ["golang.org/x/vuln"], "enabled": False})'

# CFC-2 builds golangci-lint at any version both sites agree on; the manager extracts vN.N.N only.
# Both sites move, so the manager extracts nothing at all (DTP-4's dead-manager line is right too).
ALSO_RED='renovate: customManagers\[[0-9]+\] "golangci-lint[^"]*" \(renovate\.json:[0-9]+\) matches 2 file\(s\) .* -- a dead manager' \
mutant_rejected "golangci-lint pinned to a pre-release" \
  "golangci-lint: (Makefile|hack/tooling/\\.custom-gcl\\.yml):[0-9]+ says 'v2\\.99\\.0-rc\\.1', not a plain vN\\.N\\.N release" \
  'GOLANGCI_LINT_VERSION \?= v2\.99\.0-rc\.1' Makefile \
  bash -c 'sed -i -E "s/^(GOLANGCI_LINT_VERSION \?= )v[0-9.]+\$/\1v2.99.0-rc.1/" Makefile &&
    sed -i -E "s/^version: v[0-9.]+\$/version: v2.99.0-rc.1/" hack/tooling/.custom-gcl.yml'

# Coverage is per site line and value: the installer's usage comment (GITLEAKS_VERSION=<v>) still
# extracts from the same file when the `:-` default no longer does, and a capture that stops at
# N.N would make the bot rewrite part of the version.
mutant_rejected "gitleaks installer default not matched, its usage comment still is" \
  'renovate: gitleaks pin [^ ]+ at hack/install-gitleaks\.sh:[0-9]+ is extracted by no custom manager' \
  'MUTANT-NO-DEFAULT' "${RJ}" \
  rj 'm = mgr("gitleaks/gitleaks"); s = m["matchStrings"][0]; m["matchStrings"][0] = s.replace("|:-|", "|"); assert m["matchStrings"][0] != s; m["description"] += " MUTANT-NO-DEFAULT"'

mutant_rejected "gitleaks capture stops at N.N" \
  'renovate: gitleaks pin [^ ]+ at (\.github/workflows/ci\.yaml|hack/install-gitleaks\.sh):[0-9]+ is extracted by no custom manager' \
  'MUTANT-SHORT' "${RJ}" \
  rj 'm = mgr("gitleaks/gitleaks"); s = m["matchStrings"][0]; m["matchStrings"][0] = s.replace("[0-9]+\\.[0-9]+\\.[0-9]+", "[0-9]+\\.[0-9]+"); assert m["matchStrings"][0] != s; m["description"] += " MUTANT-SHORT"'

# Extracted under another name, the site is looked up and grouped as another tool.
mutant_rejected "gitleaks extracted under another depName" \
  'renovate: gitleaks pin [^ ]+ at [^ ]+ is extracted by no custom manager as gitleaks/gitleaks' \
  'zricethezav/gitleaks' "${RJ}" \
  rj 'mgr("gitleaks/gitleaks")["depNameTemplate"] = "zricethezav/gitleaks"'

mutant_rejected "golangci-lint disabled by a rule" \
  'renovate: golangci-lint pin [^ ]+ at [^ ]+ is extracted by no custom manager' \
  'mutant-disabled' "${RJ}" \
  rj 'cfg["packageRules"].insert(rule("go toolchain"), {"description": "mutant-disabled", "matchDepNames": ["golangci/golangci-lint"], "enabled": False})'

mutant_rejected "golangci-lint sites in different groups" \
  'renovate: golangci-lint sites are not in one group \(Makefile:[0-9]+ in .golangci split.; hack/tooling/\.custom-gcl\.yml:[0-9]+ in .pinned build tools.\)' \
  'golangci split' "${RJ}" \
  rj 'cfg["packageRules"].insert(rule("go toolchain"), {"matchFileNames": ["Makefile"], "groupName": "golangci split"})'

# No groupName at all: each depName gets its own branch, so shellcheck's two depNames split.
mutant_rejected "pinned build tools rule removed (no group)" \
  "renovate: [a-z-]+ sites are not in one group \\([^)]*in '-'" \
  '!"groupName": "pinned build tools"' "${RJ}" \
  rj 'del cfg["packageRules"][rule("pinned build tools")]'

mutant_rejected "shellcheck group rule removed" \
  "renovate: shellcheck shares the group 'pinned build tools' with " \
  '!"groupName": "shellcheck"' "${RJ}" \
  rj 'del cfg["packageRules"][rule("shellcheck")]'

# Order: the shellcheck rule before "pinned build tools" loses (the last matching groupName wins).
mutant_rejected "shellcheck group rule before pinned build tools" \
  "renovate: shellcheck shares the group 'pinned build tools' with " \
  'MUTANT-MOVED' "${RJ}" \
  rj 'r = cfg["packageRules"].pop(rule("shellcheck")); r["description"] = "MUTANT-MOVED"; cfg["packageRules"].insert(rule("pinned build tools"), r)'

# Shapes the model cannot evaluate fail closed.
mutant_rejected "group rule that depends on the update type" \
  'renovate: renovate\.json packageRules\[[0-9]+\] cannot be evaluated for golangci/golangci-lint .*depends on the update type' \
  'golangci majors' "${RJ}" \
  rj 'cfg["packageRules"].insert(rule("go toolchain"), {"matchDepNames": ["golangci/golangci-lint"], "matchUpdateTypes": ["major"], "groupName": "golangci majors"})'

mutant_rejected "matcher the model does not implement" \
  'renovate: renovate\.json packageRules\[[0-9]+\] cannot be evaluated for .*matcher matchCurrentVersion is not modelled' \
  'matchCurrentVersion' "${RJ}" \
  rj 'cfg["packageRules"].insert(rule("go toolchain"), {"matchCurrentVersion": "<1.0.0", "groupName": "old"})'

mutant_rejected "manager key the model does not implement" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: keys fileMatch are not modelled' \
  '"fileMatch"' "${RJ}" \
  rj 'mgr("helm/helm")["fileMatch"] = ["x"]'

mutant_rejected "lookbehind in a matchString (not RE2)" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: lookaround .\(\?<=. is not RE2' \
  '\(\?<=HELM\)' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "(?<=HELM)" + m["matchStrings"][0]'

mutant_rejected "lookahead in a matchString (not RE2)" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: lookaround .\(\?=v. is not RE2' \
  '\(\?=v' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "(?=v)" + m["matchStrings"][0]'

mutant_rejected "backreference in a matchString (not RE2)" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: backreference .\\\\1. is not RE2' \
  'MUTANT-BACKREF' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "(H)\\1?" + m["matchStrings"][0]; m["description"] += " MUTANT-BACKREF"'

mutant_rejected "named backreference in a matchString (not RE2)" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: construct .\(\?P=. is not RE2' \
  'MUTANT-NAMED-BACKREF' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "(?P<h>H)?(?P=h)?" + m["matchStrings"][0]; m["description"] += " MUTANT-NAMED-BACKREF"'

mutant_rejected "atomic group in a matchString (not RE2)" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: construct .\(\?>. is not RE2' \
  'MUTANT-ATOMIC' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "(?>H)?" + m["matchStrings"][0]; m["description"] += " MUTANT-ATOMIC"'

mutant_rejected "possessive quantifier in a matchString (not RE2)" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: possessive quantifier .\?\+. is not RE2' \
  'MUTANT-POSSESSIVE' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "H?+" + m["matchStrings"][0]; m["description"] += " MUTANT-POSSESSIVE"'

mutant_rejected "template helper in autoReplaceStringTemplate" \
  'renovate: customManagers\[[0-9]+\] "Helm release versions[^"]*" \(renovate\.json:[0-9]+\) cannot be evaluated: autoReplaceStringTemplate template .* uses a helper' \
  'MUTANT-HELPER' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["autoReplaceStringTemplate"] = "{{#if depName}}x{{/if}}"; m["description"] += " MUTANT-HELPER"'

# `+` after `*` inside a character class is a literal, not a possessive quantifier.
control_passes "character class holding quantifier characters" \
  'MUTANT-CLASS' "${RJ}" \
  rj 'm = mgr("helm/helm"); m["matchStrings"][0] = "[*+]?" + m["matchStrings"][0]; m["description"] += " MUTANT-CLASS"'

# A renovate.json the model cannot walk (customManagers as an object) fails with the model's
# error, not as thirty uncovered sites.
mutant_rejected "renovate.json the model cannot walk" \
  'renovate: the renovate\.json model failed \(.*Error' \
  '"customManagers": \{' "${RJ}" \
  rj 'cfg["customManagers"] = {"x": cfg["customManagers"][0]}'

mutant_rejected "custom managers left out of enabledManagers" \
  'renovate: renovate\.json enabledManagers leaves out custom\.regex' \
  '"enabledManagers"' "${RJ}" \
  rj 'cfg["enabledManagers"] = ["gomod"]'

# renovate.json writes a newline in a matchString both ways (the regex escape `\\n` at the go-task
# manager's first string, a JSON newline in its second); the model reads both as Renovate does.
control_passes "matchString newline written as a JSON newline" \
  'MUTANT-JSON-NEWLINE' "${RJ}" \
  rj 'm = mgr("go-task/task"); m["matchStrings"][0] = m["matchStrings"][0].replace("\\n", "\n"); m["description"] += " MUTANT-JSON-NEWLINE"; assert "\\n" not in m["matchStrings"][0]'

# --- DTP-7 wiring: the meta-test in the copy must notice an unwired guard ---
GATE="hack/test/ci_workflow_security_test.sh"
G="hack/test/consistency_toolchain_pins_test.sh"
mutant_rejected "self-test step removed from lint" \
  "CWS-6: guard script ${G} runs in CI but its '--self-test' mode is not invoked" \
  "!run: bash ${G} --self-test" "${CI}" \
  drop_step "${CI}" "run: bash ${G} --self-test\$"

mutant_rejected "plain step removed from lint" \
  "CWS-6: guard script ${G} runs in CI but its 'bare' mode is not invoked" \
  "!run: bash ${G}\$" "${CI}" \
  drop_step "${CI}" "run: bash ${G}\$"

mutant_rejected "both steps removed from lint" \
  "CFC-6: guard ${G} is not run in its 'bare' mode" \
  "!${G}" "${CI}" \
  drop_step "${CI}" "run: bash ${G}( --self-test)?\$"
GATE="${SELF}"

echo "All toolchain-pin consistency self-tests passed."
