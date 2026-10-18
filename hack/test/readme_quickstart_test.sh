#!/usr/bin/env bash
# The README quick start names things that exist (TDS-3, openspec change test-depth-signals).
#
# Takes the README section whose `##` heading starts with "Quick start" (today
# `## Quick start (MVP)`) and extracts the lines of its fenced `sh` blocks, `#` comments included
# (`# or: task dev-up` offers `task dev-up` to users, so it counts). Prose, other sections and
# fences of another language are not read. It fails when:
#   section  no `##` heading starts with "Quick start";
#   extract  the section holds no `sh` fence line, or a fence in it is never closed;
#   task     a `task <name>` is not a task of Taskfile.yml or of a Taskfile it includes (an
#            included task is `<namespace>:<name>`);
#   apply    a `kubectl apply -k <path>` path does not exist in the tree;
#   kubectl  a flag comes before the kubectl verb (its value would hide the verb);
#   get      a `kubectl get` resource name (each comma-separated name, and the type part of
#            every operand once one is TYPE/NAME) is not a `shortNames`, `singular` or `plural`
#            entry of a CRD in config/crd/bases. Fail closed: built-in resources (pods, ...) are
#            not quick-start material.
# Whether demo-up/demo-down delegate to the hero harness is demo_task_aliases_test.sh's, not
# repeated here. Static: no Docker, no cluster, no task binary.
#
# Usage:
#   hack/test/readme_quickstart_test.sh              # check the repository tree
#   hack/test/readme_quickstart_test.sh --self-test  # mutate copies, assert each case
#   hack/test/readme_quickstart_test.sh <tree-root>  # check a tree (README, Taskfiles, config/)
set -euo pipefail

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
REPO="$(cd "$(dirname "${SELF}")/../.." && pwd -P)"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi
ROOT="$(cd "${1:-${REPO}}" && pwd -P)"

FAILED=0
# bad <clause> <message>: one violation; the check reports all of them, then exits 1.
bad() {
  echo "FAIL: $1: $2" >&2
  FAILED=1
}
die() {
  echo "FAIL: $1: $2" >&2
  exit 1
}

# taskfile_tasks <Taskfile> <prefix>: print every task name of the Taskfile, then recurse into its
# `includes:` with `<prefix><namespace>:`. Keys are read at the two-space indent of the top-level
# `tasks:` / `includes:` maps; a task key may hold colons (`coverage:race:`).
taskfile_tasks() {
  local file="$1" prefix="$2" ns path dir
  [[ -f "${file}" ]] || die input "${file#"${ROOT}"/} is missing"
  awk -v p="${prefix}" '
    /^[^[:space:]#]/ { intasks = ($0 ~ /^tasks:[[:space:]]*$/) }
    intasks && /^  [A-Za-z0-9_.:-]+:([[:space:]]|$)/ {
      k = substr($0, 3); sub(/:([[:space:]].*)?$/, "", k); print p k
    }
  ' "${file}"
  dir="$(dirname "${file}")"
  while IFS=$'\t' read -r ns path; do
    [[ -n "${ns}" ]] || continue
    [[ -n "${path}" ]] || die input "${file#"${ROOT}"/} includes '${ns}' without a taskfile path"
    path="${dir}/${path#./}"
    [[ -d "${path}" ]] && path="${path}/Taskfile.yml"
    taskfile_tasks "${path}" "${prefix}${ns}:"
  done < <(awk '
    /^[^[:space:]#]/ { flush(); inc = ($0 ~ /^includes:[[:space:]]*$/); next }
    !inc { next }
    /^  [A-Za-z0-9_-]+:/ {
      flush(); ns = substr($0, 3); val = ns
      sub(/:.*/, "", ns); sub(/^[^:]*:[[:space:]]*/, "", val); sub(/[[:space:]]+#.*/, "", val)
      path = val; next
    }
    ns != "" && /^    taskfile:/ { path = $0; sub(/^    taskfile:[[:space:]]*/, "", path); sub(/[[:space:]]+#.*/, "", path) }
    function flush() { if (ns != "") { gsub(/["'\'']/, "", path); print ns "\t" path }; ns = ""; path = "" }
    END { flush() }
  ' "${file}")
}

# crd_names: every spec.names plural, singular and shortNames entry under config/crd/bases.
crd_names() {
  local crds=("${ROOT}"/config/crd/bases/*.yaml)
  [[ -f "${crds[0]}" ]] || die input "no CRD in config/crd/bases"
  awk '
    FNR == 1 { innames = 0; inshort = 0 }
    /^  names:[[:space:]]*$/ { innames = 1; next }
    innames && /^([^ ]| [^ ]|  [^ ]|   [^ ])/ { innames = 0 }
    !innames { next }
    /^    (plural|singular):/ { v = $0; sub(/^[^:]*:[[:space:]]*/, "", v); print v; inshort = 0; next }
    /^    shortNames:[[:space:]]*$/ { inshort = 1; next }
    inshort && /^    - / { v = $0; sub(/^    - /, "", v); print v; next }
    { inshort = 0 }
  ' "${crds[@]}"
}

# quickstart_lines <README>: `<line>\t<text>` for each non-blank line of an `sh` fence in the
# section; `SECTION-MISSING` / `UNCLOSED <line>` mark the two structural failures. Fences are
# tracked before headings, so a `# comment` inside a fence never ends the section; a fence that
# reaches a `## ` line (the next section's heading) or the end of the file is unclosed. A `# `
# line inside a fence stays content: it cannot be told from a shell comment. A line ending in `\`
# is joined with the next, as the shell does. An indented or `~~~` fence opener in the section
# (outside a fence) is `UNSUPPORTED <line>`: it would hide its commands from this reader.
quickstart_lines() {
  awk '
    !insec && /^## Quick start/ { insec = 1; found = 1; next }
    !insec { next }
    /^```/ {
      if (!infence) { infence = 1; open = NR; lang = substr($0, 4); sub(/[[:space:]].*/, "", lang); next }
      if ($0 ~ /^```[[:space:]]*$/) { if (buf != "") print at "\t" buf; buf = ""; at = 0; infence = 0; next }
    }
    infence && /^## / { print "UNCLOSED\t" open; infence = 0; exit }
    infence && lang == "sh" && /\\$/ { if (!at) at = NR; buf = buf substr($0, 1, length($0) - 1) " "; next }
    infence { line = buf $0; if (!at) at = NR; buf = ""; if (lang == "sh" && line ~ /[^[:space:]]/) print at "\t" line; at = 0; next }
    /^[[:space:]]+(```|~~~)|^~~~/ { print "UNSUPPORTED\t" NR; exit }
    /^##?[[:space:]]/ { exit }
    END { if (!found) print "SECTION-MISSING"; else if (infence) print "UNCLOSED\t" open }
  ' "$1"
}

check_tree() {
  local readme="${ROOT}/README.md" lines tasks names n text seg i w verb j res r path op slashed
  local -a ops
  [[ -f "${readme}" ]] || die input "README.md is missing"
  tasks="$(taskfile_tasks "${ROOT}/Taskfile.yml" "")"
  names="$(crd_names)"
  [[ -n "${names}" ]] || die input "config/crd/bases names no resource"
  lines="$(quickstart_lines "${readme}")"
  case "${lines}" in
    SECTION-MISSING) die section "README.md has no '## Quick start' heading (a heading starting with \"Quick start\")" ;;
    *UNSUPPORTED*) die extract "the fence opened at README line ${lines##*UNSUPPORTED$'\t'} is indented or uses ~~~; write sh fences as column-0 backticks" ;;
    *UNCLOSED*) die extract "the fence opened at README line ${lines##*UNCLOSED$'\t'} is never closed" ;;
    "") die extract "the Quick start section holds no sh fence line: nothing to check" ;;
  esac
  while IFS=$'\t' read -r n text; do
    # One segment per command: split at shell separators and subshell/substitution brackets,
    # and at `#` starting a word (a comment is its own segment, so `# or: task dev-up` is
    # checked like code). awk, not sed: BSD sed writes `\n` in a replacement as a literal n.
    text="$(awk '{ gsub(/(^|[[:space:]])#/, "\n"); gsub(/&&|\|\||[;|&()`]/, "\n"); print }' <<<"${text}")"
    while IFS= read -r seg; do
      read -ra w <<<"${seg}"
      for ((i = 0; i < ${#w[@]}; i++)); do
        case "${w[i]}" in
          task)
            res=0
            for ((j = i + 1; j < ${#w[@]}; j++)); do
              [[ "${w[j]}" == -* || "${w[j]}" == *=* ]] && continue
              res=1
              grep -Fx -- "${w[j]}" <<<"${tasks}" >/dev/null ||
                bad task "'${w[j]}' (README line ${n}) is not a task of Taskfile.yml or a Taskfile it includes"
            done
            ((res)) || grep -Fx default <<<"${tasks}" >/dev/null ||
              bad task "bare 'task' (README line ${n}) but Taskfile.yml has no default task"
            break
            ;;
          kubectl)
            verb="" path="" ops=()
            # Fail closed on a flag before the verb (`kubectl --context x get ...`): its value
            # would be read as the verb and nothing checked.
            if [[ "${w[i + 1]:-}" == -* ]]; then
              bad kubectl "'kubectl ${w[i + 1]}' (README line ${n}): a flag before the verb is not supported here; put the verb first"
              break
            fi
            for ((j = i + 1; j < ${#w[@]}; j++)); do
              case "${w[j]}" in
                -n | --namespace | -o | --output | -l | --selector) ((j += 1)) ;;
                -k | --kustomize) ((j += 1)) && path="${w[j]:-}" ;;
                -k=* | --kustomize=*) path="${w[j]#*=}" ;;
                -*) ;;
                *) if [[ -z "${verb}" ]]; then verb="${w[j]}"; else ops+=("${w[j]}"); fi ;;
              esac
            done
            if [[ "${verb}" == get ]]; then
              ((${#ops[@]})) || bad get "'kubectl get' (README line ${n}) names no resource"
              # `get TYPE[,TYPE...] [NAME...]` names its types in the first operand only. Once any
              # operand is TYPE/NAME, every operand must be (kubectl rejects the mix, so the
              # guard does too) and the type part of each is checked.
              slashed=0
              for op in "${ops[@]}"; do [[ "${op}" != */* ]] || slashed=1; done
              ((slashed)) || ops=("${ops[@]:0:1}")
              for op in "${ops[@]}"; do
                if ((slashed)) && [[ "${op}" != */* ]]; then
                  bad get "'${op}' (README line ${n}) is a plain operand next to a TYPE/NAME one; kubectl rejects the mix"
                  continue
                fi
                IFS=',' read -ra r <<<"${op%%/*}"
                for res in "${r[@]}"; do
                  grep -Fx -- "${res}" <<<"${names}" >/dev/null ||
                    bad get "'${res}' (README line ${n}) is not a shortNames, singular or plural name of a CRD in config/crd/bases"
                done
              done
            elif [[ "${verb}" == apply && "${seg}" =~ (^|[[:space:]])(-k|--kustomize)([=[:space:]]|$) ]]; then
              [[ -n "${path}" && -e "${ROOT}/${path}" ]] ||
                bad apply "kubectl apply -k path '${path}' (README line ${n}) does not exist"
            fi
            break
            ;;
        esac
      done
    done <<<"${text}"
  done <<<"${lines}"
}

if [[ "${MODE}" == "check" ]]; then
  check_tree
  ((FAILED == 0)) || exit 1
  echo "ok - README quick start names only existing tasks, sample paths and Kollect resources"
  exit 0
fi

# ---------------------------------------------------------------------------
# --self-test: each case mutates a fresh copy of the inputs and runs the check on it.
# ---------------------------------------------------------------------------
ST_BAD=0
COPY=""
trap 'rm -rf ${COPY} 2>/dev/null' EXIT

st_fail() {
  echo "FAIL: self-test: $*" >&2
  ST_BAD=$((ST_BAD + 1))
}

new_copy() {
  rm -rf "${COPY:-/nonexistent}"
  COPY="$(mktemp -d)"
  mkdir -p "${COPY}/hack"
  cp "${REPO}/README.md" "${REPO}/Taskfile.yml" "${COPY}/"
  cp -R "${REPO}/hack/task" "${COPY}/hack/task"
  cp -R "${REPO}/config" "${COPY}/config"
}

# replace_once <file> <old> <new>: an exactly-once literal replace; a vanished or repeated anchor
# fails the case instead of testing nothing.
replace_once() {
  local file="$1" old="$2" new="$3" content rest
  content="$(cat "${file}")"
  [[ "${content}" == *"${old}"* ]] || return 1
  rest="${content#*"${old}"}"
  [[ "${rest}" != *"${old}"* ]] || return 1
  printf '%s\n' "${content%%"${old}"*}${new}${rest}" >"${file}"
}

# apply_edits <label> <rel> <old> <new> [<old> <new>...]: returns 1 (after st_fail) when an
# anchor does not apply.
apply_edits() {
  local label="$1" rel="$2"
  shift 2
  while [[ "$#" -ge 2 ]]; do
    if ! replace_once "${COPY}/${rel}" "$1" "$2"; then
      st_fail "${label}: mutant did not apply (anchor missing or not unique in ${rel}) -- nothing was tested"
      return 1
    fi
    shift 2
  done
}

# expect_red <label> <clause> <needle>: the check on the current copy must exit non-zero with a
# `FAIL: <clause>:` line that contains <needle>. Leaves the output in OUT.
expect_red() {
  local label="$1" clause="$2" needle="$3" rc
  OUT="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  if ((rc == 0)); then
    st_fail "${label}: the check accepted the mutant (exit 0)"
  elif ! grep "^FAIL: ${clause}:" <<<"${OUT}" | grep -F -- "${needle}" >/dev/null; then
    st_fail "${label}: no 'FAIL: ${clause}:' line naming ${needle}; got: $(grep '^FAIL' <<<"${OUT}" | head -3 | tr '\n' ' ')"
  else
    echo "ok - self-test: ${label} rejected (${clause}: ${needle})"
  fi
}

# reject <label> <clause> <needle> <rel> <old> <new>...: a fresh copy, the edits, expect_red.
reject() {
  local label="$1" clause="$2" needle="$3" rel="$4"
  shift 4
  new_copy
  apply_edits "${label}" "${rel}" "$@" || return 0
  expect_red "${label}" "${clause}" "${needle}"
}

# accept <label> <rel> <old> <new>...: the check must pass on the mutated copy.
accept() {
  local label="$1" rel="$2"
  shift 2
  new_copy
  apply_edits "${label}" "${rel}" "$@" || return 0
  if OUT="$(bash "${SELF}" "${COPY}" 2>&1)"; then
    echo "ok - self-test: ${label} accepted"
  else
    st_fail "${label}: the check rejected a valid tree: $(grep '^FAIL' <<<"${OUT}" | head -3 | tr '\n' ' ')"
  fi
}

R=README.md
CLONE='git clone https://github.com/platformrelay/kollect.git && cd kollect'
FENCE1='```sh
git clone https://github.com/platformrelay/kollect.git && cd kollect
task demo-up                      # preferred: kind + Forgejo + Ready Git inventory
# or: task dev-up                 # same hero path after build
kubectl get kinv,ktgt,ksnap -A    # watch Ready + ConnectionVerified
```'
FENCE2='```sh
kubectl apply -k config/samples/advanced/
```'

# R7 control: the real tree, copied unchanged, passes.
accept "R7 no-op control" "${R}"

reject "R1 demo-up renamed in Taskfile.yml" task "'demo-up'" Taskfile.yml \
  '  demo-up:' '  demo-start:'
reject "R2 dev-up renamed (commented alternative)" task "'dev-up'" Taskfile.yml \
  '  dev-up:' '  dev-start:'
reject "R3 unknown short name" get "'foo'" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl get kinv,foo -A'
reject "R3b built-in resource next to a CRD plural" get "'pods'" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl get kollectprofiles,pods -A'
if grep '^FAIL' <<<"${OUT}" | grep -F kollectprofiles >/dev/null; then
  st_fail "R3b: the CRD plural kollectprofiles was reported: $(grep '^FAIL' <<<"${OUT}" | tr '\n' ' ')"
fi
reject "R4 missing apply -k path" apply "config/samples/nosuch/" "${R}" \
  'kubectl apply -k config/samples/advanced/' 'kubectl apply -k config/samples/nosuch/'
reject "R5 no Quick start heading" section "Quick start" "${R}" \
  '## Quick start (MVP)' '## Getting started (MVP)'
accept "R6 included (namespaced) tasks" "${R}" \
  "${CLONE}" "${CLONE}
task test:run
task test:coverage:race"
reject "R8 section without sh fences" extract "no sh fence" "${R}" \
  "${FENCE1}" '' "${FENCE2}" ''
reject "R9 unknown namespaced task" task "'test:nosuch'" "${R}" \
  "${CLONE}" "${CLONE}
task test:nosuch"
accept "R10 invalid commands only in prose, another section and a text fence" "${R}" \
  'samples are an explicit opt-in:' 'samples are an explicit opt-in (or run `task nosuch`, then
`kubectl get foo`):

```text
task nosuch
kubectl get foo
```' \
  '## How it works' '## Not the quick start

```sh
task nosuch
kubectl get foo
```

## How it works'

# Clause and input controls beyond R1-R10.
reject "X1 short name read from the CRD, not a list" get "'kinv'" \
  config/crd/bases/kollect.dev_kollectinventories.yaml '    - kinv' '    - kinvx'
new_copy
if apply_edits "X2 include namespace read from Taskfile.yml" "${R}" "${CLONE}" "${CLONE}
task test:run" &&
  apply_edits "X2 include namespace read from Taskfile.yml" Taskfile.yml '  test:
    taskfile:' '  tst:
    taskfile:'; then
  expect_red "X2 include namespace read from Taskfile.yml" task "'test:run'"
fi
reject "X3 unterminated sh fence" extract "never closed" "${R}" \
  'kubectl apply -k config/samples/advanced/
```' 'kubectl apply -k config/samples/advanced/'
reject "X6 second command after a separator" task "'nosuch'" "${R}" \
  "${CLONE}" "${CLONE}
kubectl get kinv -A && task nosuch"
reject "X7 second command after a single &" task "'nosuch'" "${R}" \
  "${CLONE}" "${CLONE}
kubectl get kinv -A & task nosuch"
reject "X8 command inside a substitution" task "'nosuch'" "${R}" \
  "${CLONE}" "${CLONE}
echo \"\$(task nosuch)\""
reject "X9 global flag before the kubectl verb" kubectl "--context" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl --context kind-x get kinv,foo -A'
reject "X10 text fence left open into the next section" extract "never closed" "${R}" \
  "${FENCE2}" "${FENCE2}

\`\`\`text
open"
# X11: an sh fence open at the end of the file (README cut after FENCE2's opener).
new_copy
head -n "$(grep -n -F 'kubectl apply -k config/samples/advanced/' "${COPY}/${R}" | cut -d: -f1)" "${COPY}/${R}" >"${COPY}/cut.md"
mv "${COPY}/cut.md" "${COPY}/${R}"
expect_red "X11 sh fence open at the end of the file" extract "never closed"
reject "X15 second TYPE/NAME operand of kubectl get" get "'foo'" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl get kinv/a foo/b -A'
reject "X16 plain operand after a TYPE/NAME operand" get "'ktgt'" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl get kinv/a ktgt -A'
accept "X17 TYPE/NAME operands of Kollect resources" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl get kinv/a ktgt/b kollectprofiles/c -A'
accept "X18 plain NAME operands after the type are not types" "${R}" \
  'kubectl get kinv,ktgt,ksnap -A' 'kubectl get kinv my-inventory other -n demo'
reject "X12 backslash-continued kubectl get" get "'foo'" "${R}" \
  "${CLONE}" "${CLONE}
kubectl \\
  get kinv,foo"
reject "X14 continued line closing a fence" get "'foo'" "${R}" \
  'kubectl apply -k config/samples/advanced/
```' 'kubectl get kinv,foo -A \
```'
reject "X13 indented sh fence in the section" extract "indented" "${R}" \
  "${FENCE2}" "${FENCE2}

- a step:

  \`\`\`sh
  kubectl get foo
  \`\`\`"
new_copy
rm -f "${COPY}/Taskfile.yml"
expect_red "X4 Taskfile.yml missing" input "Taskfile.yml"
new_copy
rm -f "${COPY}"/config/crd/bases/*.yaml
expect_red "X5 no CRD in config/crd/bases" input "config/crd/bases"

if ((ST_BAD > 0)); then
  echo "FAIL: self-test: ${ST_BAD} case(s) failed" >&2
  exit 1
fi
echo "All README quick-start self-tests passed."
