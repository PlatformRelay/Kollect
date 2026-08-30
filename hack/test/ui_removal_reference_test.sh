#!/usr/bin/env bash
# Reference check: frozen kollect-ui product surface must be gone (UI-REMOVE-01).
# Fails while SPA/chart/CI/docs paths or product wiring remain. Allowlists
# Charm Gum demo helper lib/ui.sh, webhook paths, and CHANGELOG history.
#
# GATE-UIREF-01: the content scan used to walk the WORKING TREE with `find` and a
# hardcoded prune list. That list is a denylist of remembered paths, not a statement
# of what the repo contains, so any gitignored directory nobody thought to add --
# `kollect-repos/` (a nested kollect-lab checkout), `.cursor/` -- put untracked files
# carrying the literal `kollect-ui` into the scan and redded the gate. A fresh CI
# clone has neither directory, so the failure was invisible on CI and fired only on a
# maintainer's machine, which is exactly where it destroys trust in the local gate
# matrix. The scan set is now derived from TRACKED files (`git ls-files`), which is
# the same set CI sees. `--others --exclude-standard` is deliberately NOT added: an
# unstaged file is not yet part of the repository, and widening the set back to the
# working tree is how a local-only failure mode returns. CI reds it the moment it is
# committed. The forbidden-path checks below deliberately still probe the
# WORKING TREE: a re-created `ui/` must red whether or not anyone staged it.
#
# A scan that silently covers nothing is worse than the bug above, so a missing git,
# a non-repository root, an empty scan set and a scan set that lost its `mkdocs.yml`
# anchor are all hard failures. The self-test at the bottom builds throwaway fixture
# repos and re-proves every one of those directions on each run.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
readonly repo_root

fail() {
  printf 'ui removal reference: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

# --- Paths that must not exist -------------------------------------------------
forbidden_paths=(
  ui
  charts/kollect-ui
  charts/kollect/charts/kollect-ui-0.1.0.tgz
  .github/workflows/ui-ci.yaml
  docs/operator-manual/ui.md
  docs/examples/ui-local-development.md
  docs/assets/ui-inventory-placeholder.png
  docs/assets/ui-inventory-placeholder.svg
  docs/adr/0408-read-api-ui-architecture.md
  docs/adr/0409-kollect-ui-deployment.md
  docs/adr/0410-ui-engineering-and-quality-gates.md
  docs/adr/0411-read-api-extensions-for-ui.md
  docs/adr/0412-mock-read-api-for-ui-development.md
  hack/ci/ui-verify.sh
  hack/verify-ui-headers.sh
  hack/add-ui-headers.sh
  hack/verify-ui-mock.sh
  hack/ui-e2e-docker.sh
  hack/test/sonar_ko_04_ui_automount_test.sh
)
readonly forbidden_paths

pattern='kollect-ui|UI_IMAGE_|sbom-ui|ui-playwright-msw|ui-ci\.yaml|charts/kollect-ui|operator-manual/ui\.md|ui-local-development|0408-read-api-ui|0409-kollect-ui|0410-ui-engineering|0411-read-api-extensions|0412-mock-read-api|task ui-|build-ui|ghcr\.io/.*/kollect-ui'
readonly pattern

# The scan-set filter, ported from the old `find` expression. Two of its three
# exclusions change meaning if copied mechanically, so they are spelled out here:
#   * `CHANGELOG.md` was matched by BASENAME at any depth, not as a top-level path;
#   * the demo helper and this script were `-path './...'` patterns -- `git ls-files`
#     emits repo-relative paths with no `./` prefix, so the leading dot must go.
# Dropping the self-exclusion is the interesting failure: this file carries every
# literal in ${pattern}, so the gate would red on itself.
scannable() {
  local f="$1"

  case "${f}" in
  hack/test/ui_removal_reference_test.sh | hack/demo/*/lib/ui.sh) return 1 ;;
  esac

  # The old `find` name list also spelled out Taskfile.yml, mkdocs.yml,
  # .go-arch-lint.yml, SECURITY.md, GOVERNANCE.md and CONTRIBUTING.md; every one of
  # them is already matched by *.yml or *.md, so listing them again only earns an
  # SC2221/SC2222 warning. Nothing is dropped from the scan -- only the three names
  # below fall outside the globs and still need spelling out.
  case "${f##*/}" in
  CHANGELOG.md) return 1 ;;
  *.yaml | *.yml | *.md | *.sh | *.gotmpl | \
    renovate.json | sonar-project.properties | Chart.lock) return 0 ;;
  esac

  return 1
}

check_ui_removal() {
  local root
  root="$(cd "$1" 2>/dev/null && pwd -P)" ||
    fail "scan root does not exist: $1"
  cd "${root}"

  local path
  for path in "${forbidden_paths[@]}"; do
    # Working-tree probe on purpose: a re-created ui/ reds even while untracked.
    if [[ -e "${path}" ]]; then
      fail "forbidden path still present: ${path}"
    fi
  done

  # --- Product wiring strings (tracked files; CHANGELOG + lib/ui.sh excluded) ---
  command -v git >/dev/null 2>&1 ||
    fail "git is required to derive the content-scan set from tracked files; refusing to scan nothing"

  local toplevel
  toplevel="$(git -C "${root}" rev-parse --show-toplevel 2>/dev/null)" ||
    fail "${root} is not a git repository, so no tracked-file set can be derived; refusing to scan nothing"
  toplevel="$(cd "${toplevel}" && pwd -P)"
  # Equality, not `--is-inside-work-tree`: that walks UPWARD, so a non-repository
  # directory nested inside a checkout would silently scan the parent's files and
  # report a pass that proves nothing about the directory actually under test.
  [[ "${toplevel}" == "${root}" ]] ||
    fail "${root} is not the root of its git repository (toplevel is ${toplevel}); refusing to scan a set derived from somewhere else"

  local scan_files=() f
  # Process substitution, never a pipe: a pipeline runs the loop in a subshell and
  # leaves scan_files empty -- the silent zero-file scan this gate exists to prevent.
  while IFS= read -r -d '' f; do
    scannable "${f}" || continue
    # Tracked but deleted from the working tree: skipping keeps rg/grep from
    # aborting the whole scan (rc=2) over a file nobody can read.
    [[ -f "${root}/${f}" ]] || continue
    scan_files+=("${f}")
  done < <(git -C "${root}" ls-files -z)

  ((${#scan_files[@]} > 0)) ||
    fail "the content scan derived an empty file set from tracked files; a gate that scans nothing passes vacuously"

  # Non-empty is satisfiable by one stray file. mkdocs.yml is tracked, matches the
  # type filter, and is read again below, so anchor on it: if it is missing the scan
  # set has lost its shape and every assertion downstream is decoration.
  local anchored=0
  for f in "${scan_files[@]}"; do
    if [[ "${f}" == "mkdocs.yml" ]]; then
      anchored=1
      break
    fi
  done
  [[ "${anchored}" -eq 1 ]] ||
    fail "the content scan set does not contain mkdocs.yml; the tracked-file derivation is broken and the scan would pass on a set that is not this repository"

  # Exit codes (rg and GNU/BSD grep -E): 0=matches, 1=no matches, 2+=error.
  # Prefer ripgrep; fall back to grep so Docs CI (no rg preinstall) still scans.
  # Capture rc outside `if !` — bash sets $? to 0 after a successful `!` negation,
  # which would mis-classify "no matches" (exit 1) as a fatal error.
  local hits rc
  set +e
  if command -v rg >/dev/null 2>&1; then
    hits="$(rg -n --no-heading -e "${pattern}" -- "${scan_files[@]}" 2>/dev/null)"
    rc=$?
  else
    hits="$(grep -EHn -e "${pattern}" -- "${scan_files[@]}" 2>/dev/null)"
    rc=$?
  fi
  set -e
  if [[ ${rc} -gt 1 ]]; then
    fail "content scan failed while looking for residual UI references (rc=${rc})"
  fi
  if [[ ${rc} -eq 1 ]]; then
    hits=""
  fi

  # Drop allowlisted false positives (webhook "ui", Charm Gum helper mentions).
  local filtered
  filtered="$(
    printf '%s\n' "${hits}" | awk '
      NF == 0 { next }
      /hack\/demo\/.*\/lib\/ui\.sh/ { next }
      /webhook/ && !/kollect-ui/ && !/UI_IMAGE_/ && !/sbom-ui/ { next }
      { print }
    '
  )"

  if [[ -n "${filtered}" ]]; then
    printf '%s\n' "${filtered}" >&2
    fail "residual product UI references remain (see above)"
  fi

  # Nav must not advertise removed pages.
  if grep -Eq 'operator-manual/ui\.md|examples/ui-local-development\.md|0408-|0409-|0410-|0411-|0412-' mkdocs.yml; then
    fail "mkdocs.yml still navigates to removed UI pages or ADRs"
  fi

  printf 'ui removal reference: ok\n'
}

check_ui_removal "${repo_root}"

# --- Self-test -----------------------------------------------------------------
# A gate that only ever runs against one clean tree is not evidence, and this one is
# structurally unable to observe its own regression there: a CI clone (and a fresh
# worktree) contains none of the gitignored directories that broke it. So build the
# missing shapes as throwaway fixture repositories and re-prove all four directions
# on every run -- untracked/gitignored noise is ignored, tracked wiring still reds, a
# working-tree-only forbidden path still reds, and an unusable scan set fails loudly.
FIXTURES="$(mktemp -d)"
trap 'rm -rf "${FIXTURES}"' EXIT

# A minimal repository the gate accepts: mkdocs.yml (the scan-set anchor and the nav
# source), one more scannable tracked file, and a .gitignore for the noise fixtures.
# `git add` alone is enough -- `git ls-files` reads the index, so no commit and no
# user identity are needed.
new_fixture_repo() {
  local name="$1" root="${FIXTURES}/$1"

  mkdir -p "${root}"
  git init -q "${root}" >/dev/null 2>&1 ||
    fail "self-test: could not git init the ${name} fixture"
  printf 'site_name: fixture\nnav:\n  - Home: index.md\n' >"${root}/mkdocs.yml"
  printf '# fixture\n\nNothing to see here.\n' >"${root}/README.md"
  printf 'ignored/\n.cursor/\n' >"${root}/.gitignore"
  git -C "${root}" add mkdocs.yml README.md .gitignore ||
    fail "self-test: could not stage the ${name} fixture"

  printf '%s\n' "${root}"
}

# Subshell: `fail`'s exit and check_ui_removal's `cd` must not escape into the parent.
gate_accepts() {
  local root="$1" label="$2" output status=0

  output="$( (check_ui_removal "${root}") 2>&1 )" || status=$?
  [[ ${status} -eq 0 ]] ||
    fail "self-test: the gate failed although ${label}; output: ${output}"
  [[ "${output}" == *"ui removal reference: ok"* ]] ||
    fail "self-test: the gate exited 0 for ${label} without reporting ok; output: ${output}"
  pass "self-test: gate accepts ${label}"
}

# `${expect}` is the load-bearing half. Any nonzero exit proves nothing about WHICH
# check fired -- a broken fixture also exits nonzero.
gate_rejects() {
  local root="$1" label="$2" expect="$3" output status=0

  output="$( (check_ui_removal "${root}") 2>&1 )" || status=$?
  [[ ${status} -ne 0 ]] ||
    fail "self-test: the gate still passed although ${label}; it is vacuous"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason; expected the failure to mention '${expect}', got: ${output}"
  pass "self-test: gate rejects ${label}"
}

fixture="$(new_fixture_repo clean)"
# new_fixture_repo's `fail` can only exit the command substitution, so re-check here:
# an empty root would otherwise turn every assertion below into a different failure.
[[ -n "${fixture}" && -d "${fixture}" ]] ||
  fail "self-test: the clean fixture repository was not created"
gate_accepts "${fixture}" 'a clean fixture repository'

# Direction 1 (the GATE-UIREF-01 regression): untracked, gitignored files carrying the
# forbidden literals -- a nested checkout and an editor skill directory -- are noise on
# a maintainer's machine and absent from CI. They must not red the gate.
mkdir -p "${fixture}/ignored/kollect-lab" "${fixture}/.cursor/skills"
printf 'deploys charts/kollect-ui from the lab\n' >"${fixture}/ignored/kollect-lab/notes.md"
printf 'UI_IMAGE_TAG and kollect-ui notes\n' >"${fixture}/.cursor/skills/skill.md"
# Two guards against a vacuous pass: the files must really be gitignored (an untracked
# file git does NOT ignore would prove a weaker property), and they must really carry
# text the scan looks for (otherwise the old find-based scan would have passed too).
for ignored_fixture in ignored/kollect-lab/notes.md .cursor/skills/skill.md; do
  git -C "${fixture}" check-ignore -q "${ignored_fixture}" ||
    fail "self-test: ${ignored_fixture} is not gitignored in the fixture, so it proves nothing about ignored noise"
  grep -Eq -e "${pattern}" "${fixture}/${ignored_fixture}" ||
    fail "self-test: ${ignored_fixture} carries none of the forbidden literals, so accepting it proves nothing"
done
gate_accepts "${fixture}" 'untracked, gitignored files carrying the forbidden literals'

# Direction 2: a TRACKED file reintroducing product-UI wiring must still red, and the
# failure must name the file.
printf 'image: ghcr.io/platformrelay/kollect-ui:v1\n' >"${fixture}/docs-regression.md"
git -C "${fixture}" add docs-regression.md ||
  fail "self-test: could not stage the tracked-wiring mutant"
gate_rejects "${fixture}" 'a tracked file reintroduces product-UI wiring' \
  'residual product UI references remain'
gate_rejects "${fixture}" 'a tracked file reintroduces product-UI wiring (failure names the file)' \
  'docs-regression.md'
git -C "${fixture}" rm -q --cached docs-regression.md >/dev/null
rm -f "${fixture}/docs-regression.md"
gate_accepts "${fixture}" 'the tracked-wiring mutant after it is removed again'

# Direction 3: forbidden_paths probes the WORKING TREE. A re-created ui/ that was never
# staged is invisible to `git ls-files`, and must still red.
mkdir -p "${fixture}/ui/src"
printf 'console.log("spa")\n' >"${fixture}/ui/src/main.ts"
if git -C "${fixture}" check-ignore -q ui/src/main.ts; then
  fail "self-test: the ui/ mutant must be untracked-but-not-ignored, otherwise it does not test the working-tree probe"
fi
gate_rejects "${fixture}" 'an untracked ui/ directory is re-created' \
  'forbidden path still present: ui'
rm -rf "${fixture}/ui"
gate_accepts "${fixture}" 'the fixture after the ui/ directory is removed again'

# Direction 4: an unusable scan set must fail loudly instead of passing vacuously.
empty_repo="${FIXTURES}/empty"
mkdir -p "${empty_repo}"
git init -q "${empty_repo}" >/dev/null 2>&1
printf 'not a scannable type\n' >"${empty_repo}/notes.txt"
git -C "${empty_repo}" add notes.txt
gate_rejects "${empty_repo}" 'the tracked-file set contains nothing scannable' \
  'derived an empty file set from tracked files'

anchorless_repo="${FIXTURES}/anchorless"
mkdir -p "${anchorless_repo}"
git init -q "${anchorless_repo}" >/dev/null 2>&1
printf '# no mkdocs here\n' >"${anchorless_repo}/README.md"
git -C "${anchorless_repo}" add README.md
gate_rejects "${anchorless_repo}" 'the scan set is non-empty but has lost its mkdocs.yml anchor' \
  'does not contain mkdocs.yml'

plain_dir="${FIXTURES}/not-a-repo"
mkdir -p "${plain_dir}"
gate_rejects "${plain_dir}" 'the scan root is not a git repository' \
  'is not a git repository'

# The upward walk `git rev-parse` performs by default is the trap here: this directory
# IS inside a work tree, just not at its root, and without the equality check the gate
# would happily scan the parent fixture's files and call that a pass.
nested_dir="${fixture}/not-the-root"
mkdir -p "${nested_dir}"
gate_rejects "${nested_dir}" 'the scan root is a subdirectory of a repository, not its root' \
  'is not the root of its git repository'

gate_rejects_without_git() {
  local root="$1" output status=0

  # shellcheck disable=SC2123  # clobbering PATH is the mutation: it is how "git is
  # unavailable" is simulated, and the subshell keeps it out of the parent.
  output="$( (PATH=/nonexistent; export PATH; check_ui_removal "${root}") 2>&1 )" || status=$?
  [[ ${status} -ne 0 ]] ||
    fail "self-test: the gate still passed with no git on PATH; it would scan nothing"
  [[ "${output}" == *'git is required'* ]] ||
    fail "self-test: the gate failed without git but not for the intended reason; got: ${output}"
  pass "self-test: gate rejects a run with no git available"
}
gate_rejects_without_git "${fixture}"

printf 'ui removal reference: self-test ok\n'
