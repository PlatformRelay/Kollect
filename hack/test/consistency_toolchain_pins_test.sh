#!/usr/bin/env bash
# DTP-3, DTP-6 and DTP-7 (openspec change developer-toolchain-pins): the developer and CI tools
# that are pinned in more than one file agree, meet their floor, and can be installed.
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
#   * versions compare with a leading `v` stripped and must be plain releases (N.N.N); a
#     variable, `latest` or an empty value is a failure, never skipped;
#   * an empty scan for any tool fails, never passes;
#   * DTP-6: UNVERIFIED_BY_DESIGN in hack/test/ci_fetch_lib_hardening_test.sh (installers exempt
#     from its checksum scan) is defined once, on one line, and otherwise only read (a closed list
#     of read forms; any other mention is a write), and lists nothing outside
#     UNVERIFIED_LIMIT: the list may only shrink.
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
# The exemptions UNVERIFIED_BY_DESIGN may hold. Shrink-only: removing an entry here and from the
# hardening gate is the only allowed edit.
UNVERIFIED_LIMIT=("install-git-cliff.sh" "install-helm-docs.sh")
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

# cliff_action_sites <root>: the `with: version:` input of every orhun/git-cliff-action step, or
# "<no version: input>" when the step has none. A step is read whole (key order does not matter):
# it opens at a `- ` line and ends at the next `- ` line at its indent or a line left of it. Only
# a direct child of the step's `with:` is the input; a `version:` under `env:` or another map is
# not, and must not stand in for it.
cliff_action_sites() {
  local root="$1" f
  while IFS= read -r f; do
    awk -v f="${f}" '
      function indent(s) { match(s, /^[[:space:]]*/); return RLENGTH }
      function flush() {
        if (open && cliff) print f ":" (ver == "" ? at "\t<no version: input>" : vat "\t" ver)
        open = 0; cliff = 0; ver = ""; w = -1; wc = -1
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
          if (k ~ /^uses:[[:space:]]*["\047]?orhun\/git-cliff-action@/) { cliff = 1; at = NR }
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

check_all() {
  local root="$1" sites
  BROKEN=0

  sites="$({
    yaml_env_sites "${root}" GITLEAKS_VERSION
    default_sites "${root}" GITLEAKS_VERSION
    precommit_rev_sites "${root}" 'github\.com/gitleaks/gitleaks(\.git)?$'
  } | with_compare plain)"
  check_tool gitleaks "${GITLEAKS_FLOOR}" hack/install-gitleaks.sh "${sites}" \
    .github/workflows/ci.yaml hack/install-gitleaks.sh .pre-commit-config.yaml

  sites="$({
    taskfile_sites "${root}" GIT_CLIFF_VERSION
    cliff_action_sites "${root}"
    cliff_install_sites "${root}"
  } | with_compare plain)"
  check_tool git-cliff "${GIT_CLIFF_FLOOR}" Taskfile.yml "${sites}" \
    Taskfile.yml .github/workflows/release.yaml hack/verify-changelog.sh

  sites="$({
    yaml_env_sites "${root}" SHELLCHECK_VERSION | with_compare plain
    default_sites "${root}" SHELLCHECK_VERSION | with_compare plain
    precommit_rev_sites "${root}" 'github\.com/shellcheck-py/shellcheck-py(\.git)?$' | with_compare first3
  })"
  check_tool shellcheck - hack/install-shellcheck.sh "${sites}" \
    .github/workflows/ci.yaml hack/install-shellcheck.sh .pre-commit-config.yaml
  if [[ -n "${AGREED}" ]]; then
    check_shellcheck_digests "${root}" "${AGREED}"
  fi

  check_unverified_limit "${root}"
  return "${BROKEN}"
}

if [[ "${MODE}" == "check" ]]; then
  if check_all "${ROOT}"; then
    pass "gitleaks, git-cliff and shellcheck sites agree and meet their floors, shellcheck has digests for its pin, and UNVERIFIED_BY_DESIGN is within its limit"
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
new_copy() {
  COPY="$(mktemp -d "${WORKROOT}/XXXXXXXX")"
  (cd "${ROOT}" && git ls-files -z | tar --null --ignore-failed-read -T - -cf - 2>/dev/null) |
    tar -xf - -C "${COPY}"
}

# probe <label> <probe> <file>: the mutation landed. A probe starting with `!` must NOT match;
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
    sed -i -E "s/install-git-cliff\.sh v/install-cliff.sh v/" '"${VC}"

mutant_rejected "git-cliff-action step without a version input" \
  "git-cliff: \\.github/workflows/release\\.yaml:[0-9]+ says '<no version: input>', not a plain release version" \
  '!^[[:space:]]+version: v2\.13\.1' "${REL}" \
  sed -i -E '/orhun\/git-cliff-action@/,/version:/ { /^[[:space:]]+version: v[0-9.]+/d }' "${REL}"

# env: goes before and after the with: block, so neither a first-wins nor a last-wins reader passes.
mutant_rejected "git-cliff env version masks a drifted input" \
  'git-cliff: \.github/workflows/release\.yaml:[0-9]+ says v2\.12\.0, but the reference site Taskfile\.yml' \
  'version: v2\.13\.1 # mutant-env-before@@version: v2\.13\.1 # mutant-env-after@@version: v2\.12\.0' "${REL}" \
  bash -c 'sed -i -E "/orhun\/git-cliff-action@/,/version:/ s/version: v[0-9.]+/version: v2.12.0/" '"${REL}"' &&
    sed -i -E "/orhun\/git-cliff-action@/{n;s/^([[:space:]]+)with:\$/\1env:\n\1  version: v2.13.1 # mutant-env-before\n\1with:/}" '"${REL}"' &&
    sed -i -E "/orhun\/git-cliff-action@/,/github_token:/ s/^([[:space:]]+github_token:.*)\$/\1\n        env:\n          version: v2.13.1 # mutant-env-after/" '"${REL}"

# Key order: the with: block before uses: is the same step and the same input.
control_passes "git-cliff with: before uses:" \
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
    sed -i -E "s#github.com/shellcheck-py/shellcheck-py#github.com/example/sc#" '"${PC}"

mutant_rejected "shellcheck platforms unreadable" \
  'shellcheck: cannot read the supported platforms of hack/install-shellcheck\.sh' \
  'OSNAME="linux"' "${SI}" \
  sed -i -E 's/\)[[:space:]]*OS="([a-z]+)"/) OSNAME="\1"/' "${SI}"

# --- DTP-6 exceptions list ---
FG="${FETCH_GATE}"
mutant_rejected "UNVERIFIED_BY_DESIGN gains a third entry" \
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
