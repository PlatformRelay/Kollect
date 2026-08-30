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
# working tree is how a local-only failure mode returns. Once it is committed it is
# in the scanned set like everything else (the Docs workflow is path-filtered and is
# not a required check, so "CI catches it" is a weaker promise than it sounds -- this
# gate is the check). The forbidden-path checks below deliberately still probe the
# WORKING TREE: a re-created `ui/` must red whether or not anyone staged it.
#
# The unstated half of that change: main's twelve pruned directories are gone, so the
# scan now WIDENS BY ITSELF. `agent-context/`, `references/` and `.claude/` are
# untracked but NOT gitignored today; the day any of them is committed it enters the
# scan set like every other tracked file. That is the contract, not an accident --
# tracked means scanned -- and the fix for a resulting failure is to clean the file (or
# gitignore the directory if it was never meant to be committed). Re-adding a directory
# denylist is the one thing not to do: that denylist IS the defect described above.
#
# A scan that silently covers nothing is worse than the bug above, so a missing git,
# a non-repository root, an empty scan set and a scan set that lost its `mkdocs.yml`
# anchor are all hard failures. The self-test at the bottom builds throwaway fixture
# repos and re-proves every one of those directions on each run.
set -euo pipefail

# F1 (data loss): every git invocation below -- the scan's own `ls-files` and the
# self-test's throwaway `git init`/`git add` fixtures -- inherits the ambient git
# environment. `GIT_INDEX_FILE` is exported in every pre-commit, commit-msg and
# prepare-commit-msg hook and by `git commit --interactive`, so a maintainer running
# `task docs:verify` from a hook context would have the fixtures stage themselves
# into the REAL repository's index, against objects in a temp store this script then
# deletes -- an unrecoverable staging area, and the gate would still exit 0.
# `GIT_DIR`/`GIT_WORK_TREE` are the read-side twin (see check_ui_removal).
# GIT_OBJECT_DIRECTORY is not optional here: without it a sanitised index can still
# be written against foreign object storage.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR \
  GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
readonly repo_root

fail() {
  printf 'ui removal reference: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

# The tripwire for the `unset` above. It is unreachable while that line is present --
# that is the point: if the unset is ever dropped, a run inherited from a git hook
# dies here, loudly, BEFORE any git command, instead of staging fixture blobs into the
# real repository's index and exiting 0. Verified by ablation, not assumed.
for poisoning_var in GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR \
  GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES; do
  [[ -z "${!poisoning_var-}" ]] ||
    fail "${poisoning_var} is set (${!poisoning_var}); every git command below -- including the self-test's throwaway repositories -- would run against a foreign index or object store and destroy it. Sanitise the git environment before running this gate."
done

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
  # F2: repeated here, not just at the top of the script, because this is the unit
  # the self-test exercises -- and because the failure mode is silent. With `GIT_DIR`
  # exported, `ls-files` reads a FOREIGN index while `rev-parse --show-toplevel`
  # answers with the cwd, so the toplevel-equality guard below would certify a scan
  # of somebody else's repository as a pass (measured on this repo: 601 files -> 2,
  # exit 0, with a tracked kollect-ui reference left unseen).
  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR \
    GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES

  command -v git >/dev/null 2>&1 ||
    fail "git is required to derive the content-scan set from tracked files; refusing to scan nothing"

  local toplevel
  toplevel="$(git -C "${root}" rev-parse --show-toplevel 2>/dev/null)" ||
    fail "${root} is not a git repository, so no tracked-file set can be derived; refusing to scan nothing"
  toplevel="$(cd "${toplevel}" && pwd -P)"
  # Equality, not `--is-inside-work-tree`: that walks UPWARD, so a plain directory
  # nested inside a checkout resolves to the PARENT's toplevel. `ls-files` is
  # prefix-scoped, so the scan would come back empty rather than showing the parent's
  # files -- either way the set no longer describes the directory under test. Failing
  # on the mismatch names the real problem instead of leaving it to the empty-set
  # guard to report something misleading.
  [[ "${toplevel}" == "${root}" ]] ||
    fail "${root} is not the root of its git repository (toplevel is ${toplevel}); refusing to scan a set derived from somewhere else"

  # F4: `git ls-files` piped straight into the loop hides its exit status -- a
  # truncated or aborted listing is indistinguishable from a short repository, and
  # the files it never emitted are never scanned. Capture it first so rc is testable.
  local listing ls_rc=0
  listing="$(mktemp)" ||
    fail "could not create a temporary file for the tracked-file listing"
  git -C "${root}" ls-files -z >"${listing}" || ls_rc=$?
  if ((ls_rc != 0)); then
    rm -f "${listing}"
    fail "git ls-files failed in ${root} (rc=${ls_rc}); refusing to scan a partial file set"
  fi

  local scan_files=() f
  # Redirection from a file, never a pipe: a pipeline runs the loop in a subshell and
  # leaves scan_files empty -- the silent zero-file scan this gate exists to prevent.
  while IFS= read -r -d '' f; do
    scannable "${f}" || continue
    # Tracked but deleted from the working tree: skipping keeps rg/grep from
    # aborting the whole scan (rc=2) over a file nobody can read.
    [[ -f "${root}/${f}" ]] || continue
    scan_files+=("${f}")
  done <"${listing}"
  rm -f "${listing}"

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

  # A listing truncated to its first entry satisfies both guards above on its own, so
  # require the anchor AND something else. The floor is deliberately 2 rather than a
  # repo-sized number because the self-test fixtures are minimal repositories; the rc
  # check on `ls-files` above is the real defence against a truncated listing.
  ((${#scan_files[@]} >= 2)) ||
    fail "the content scan set is just the mkdocs.yml anchor (${#scan_files[@]} file); a listing truncated to one entry must not read as a scanned repository"

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
  # The lib/ui.sh rule below is LIVE, and it is not merely a second copy of the
  # scannable() exclusion: `case` patterns are ANCHORED, the awk regex is not. So
  # `hack/demo/*/lib/ui.sh` in scannable() covers only helpers at the repo root --
  # today just hack/demo/kind-wide-scope/lib/ui.sh -- while this rule additionally
  # exempts any NESTED `*/hack/demo/*/lib/ui.sh`. A tracked
  # `vendor/hack/demo/x/lib/ui.sh` carrying `kollect-ui` therefore enters the scan set
  # and is exempted here; that is the residual reach of the allowlist, and it is wider
  # than the header sentence above suggests. The two together are pinned by the
  # demo-helper fixture in the self-test: removing EITHER still leaves the exemption
  # standing, so no assertion can separate them, but removing BOTH reds it.
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
require_ignored_noise() {
  local root="$1" rel="$2"

  git -C "${root}" check-ignore -q "${rel}" ||
    fail "self-test: ${rel} is not gitignored in the fixture, so it proves nothing about ignored noise"
  grep -Eq -e "${pattern}" "${root}/${rel}" ||
    fail "self-test: ${rel} carries none of the forbidden literals, so accepting it proves nothing"
}

for ignored_fixture in ignored/kollect-lab/notes.md .cursor/skills/skill.md; do
  require_ignored_noise "${fixture}" "${ignored_fixture}"
done
gate_accepts "${fixture}" 'untracked, gitignored files carrying the forbidden literals'

# A fixture guard nobody has watched reject anything is decoration, and this one sits
# between a green line and a vacuous one. Prove BOTH halves fire.
printf 'nothing interesting here\n' >"${fixture}/ignored/plain.md"
if (require_ignored_noise "${fixture}" README.md) >/dev/null 2>&1; then
  fail "self-test: require_ignored_noise accepted README.md, which is tracked and not ignored; the fixture guard is decorative"
fi
if (require_ignored_noise "${fixture}" ignored/plain.md) >/dev/null 2>&1; then
  fail "self-test: require_ignored_noise accepted an ignored file carrying none of the forbidden literals; the fixture guard is decorative"
fi
rm -f "${fixture}/ignored/plain.md"
pass "self-test: the ignored-noise fixture guard rejects both a non-ignored file and a literal-free one"

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

# A file can be tracked and still absent from the working tree -- a sparse checkout is
# the realistic case, an interrupted `rm` the mundane one. Handing such a path to
# rg/grep aborts the WHOLE scan with rc=2, so the gate would fail for a reason that has
# nothing to do with UI residue and everything to do with the checkout. Removing the
# `[[ -f ]]` skip reds this line.
deleted_repo="$(new_fixture_repo deleted)"
[[ -n "${deleted_repo}" && -d "${deleted_repo}" ]] ||
  fail "self-test: the tracked-but-deleted fixture repository was not created"
# A third scannable file first: the scan-set floor is 2 and one of the fixture's two is
# about to leave the working tree, so without this the anchor-only guard would fire and
# the assertion would prove something else entirely.
printf '# still on disk\n' >"${deleted_repo}/present.md"
git -C "${deleted_repo}" add present.md ||
  fail "self-test: could not stage the tracked-but-deleted fixture"
rm -f "${deleted_repo}/README.md"
# Both halves, or the fixture proves nothing: still in the index, no longer on disk.
git -C "${deleted_repo}" ls-files --error-unmatch README.md >/dev/null 2>&1 ||
  fail "self-test: README.md is not tracked in the deleted fixture, so it never reaches the scan set"
[[ ! -e "${deleted_repo}/README.md" ]] ||
  fail "self-test: README.md is still on disk in the deleted fixture, so it does not exercise the skip"
gate_accepts "${deleted_repo}" 'a tracked file is missing from the working tree'

# Direction 4: an unusable scan set must fail loudly instead of passing vacuously.
empty_repo="${FIXTURES}/empty"
mkdir -p "${empty_repo}"
git init -q "${empty_repo}" >/dev/null 2>&1 ||
  fail "self-test: could not git init the empty-scan-set fixture"
printf 'not a scannable type\n' >"${empty_repo}/notes.txt"
git -C "${empty_repo}" add notes.txt ||
  fail "self-test: could not stage the empty-scan-set fixture"
gate_rejects "${empty_repo}" 'the tracked-file set contains nothing scannable' \
  'derived an empty file set from tracked files'

anchorless_repo="${FIXTURES}/anchorless"
mkdir -p "${anchorless_repo}"
git init -q "${anchorless_repo}" >/dev/null 2>&1 ||
  fail "self-test: could not git init the anchorless fixture"
printf '# no mkdocs here\n' >"${anchorless_repo}/README.md"
git -C "${anchorless_repo}" add README.md ||
  fail "self-test: could not stage the anchorless fixture"
gate_rejects "${anchorless_repo}" 'the scan set is non-empty but has lost its mkdocs.yml anchor' \
  'does not contain mkdocs.yml'

# F4: a listing truncated to the anchor alone satisfies both the non-empty and the
# anchor guard, so the floor has to reject it on its own.
anchor_only_repo="${FIXTURES}/anchor-only"
mkdir -p "${anchor_only_repo}"
git init -q "${anchor_only_repo}" >/dev/null 2>&1 ||
  fail "self-test: could not git init the anchor-only fixture"
printf 'site_name: fixture\n' >"${anchor_only_repo}/mkdocs.yml"
git -C "${anchor_only_repo}" add mkdocs.yml ||
  fail "self-test: could not stage the anchor-only fixture"
gate_rejects "${anchor_only_repo}" 'the scan set is nothing but the mkdocs.yml anchor' \
  'is just the mkdocs.yml anchor'

# F4 again, the other half: every guard above inspects the LISTING, so none of them
# notices a listing that was never produced -- only the rc check on `ls-files` does.
# A corrupt index is the mutation that reaches it, and it has to be that one:
# `rev-parse --show-toplevel` does not read the index, so the toplevel-equality guard
# is still satisfied and `ls-files` is the first command to fail (rc=128). Most other
# ways to break a repository trip an earlier guard and prove nothing about this branch.
corrupt_index_repo="$(new_fixture_repo corrupt-index)"
# Re-checked before the redirect below, not merely for symmetry with the clean
# fixture: new_fixture_repo's `fail` can only exit its command substitution, and an
# empty root would turn `>"${root}/.git/index"` into a write at /.git/index -- the
# exact class of accident this gate's own data-loss finding was about.
[[ -n "${corrupt_index_repo}" && -d "${corrupt_index_repo}/.git" ]] ||
  fail "self-test: the corrupt-index fixture repository was not created"
printf 'garbage' >"${corrupt_index_repo}/.git/index"
gate_rejects "${corrupt_index_repo}" 'the tracked-file listing could not be produced' \
  'git ls-files failed'

plain_dir="${FIXTURES}/not-a-repo"
mkdir -p "${plain_dir}"
gate_rejects "${plain_dir}" 'the scan root is not a git repository' \
  'is not a git repository'

# gate_rejects' own guard rail. `${expect}` is what stops every assertion above from
# degenerating into "something exited nonzero", which a broken fixture also does, so
# prove on every run that a rejection carrying the WRONG message is still refused.
# Deleting the ${expect} comparison in gate_rejects must red this.
self_test_guard_holds() {
  local root="$1" label="$2"

  if (gate_rejects "${root}" "${label}" 'unreachable-expected-message') >/dev/null 2>&1; then
    fail "self-test: gate_rejects counted ${label} as proof of the intended check; it is tautological"
  fi
  pass "self-test: gate_rejects refuses a rejection carrying the wrong message"
}
self_test_guard_holds "${plain_dir}" 'a root that fails for an unrelated reason'

# The status comparisons in gate_accepts and gate_rejects looked redundant with the
# message assertions beside them -- deleting either left every assertion above green,
# because no fixture separated the two. They are not redundant, and the separating
# case is not exotic: the gate copies MATCHING LINES from the scanned files to stderr
# before it fails, so the scanned content chooses part of the captured output. A
# tracked file can therefore print the success line on a failing run. Prove each half.
smuggle_repo="$(new_fixture_repo smuggle)"
[[ -n "${smuggle_repo}" && -d "${smuggle_repo}" ]] ||
  fail "self-test: the ok-smuggling fixture repository was not created"
printf 'image ghcr.io/platformrelay/kollect-ui:v1 -- ui removal reference: ok\n' \
  >"${smuggle_repo}/smuggle.md"
git -C "${smuggle_repo}" add smuggle.md ||
  fail "self-test: could not stage the ok-smuggling mutant"
if (gate_accepts "${smuggle_repo}" 'a repository that forges the ok line') >/dev/null 2>&1; then
  fail "self-test: gate_accepts accepted a FAILING run whose scan output merely echoed the ok line; its exit-status assertion is decorative"
fi
pass "self-test: gate_accepts refuses a failing run that only echoes the ok line"

# The mirror image: a run that PASSES cannot be counted as a rejection just because the
# expected message turns up in its output. `ui removal reference: ok` is the shortest
# way to say that -- a clean repository always prints it.
passing_repo="$(new_fixture_repo passing)"
[[ -n "${passing_repo}" && -d "${passing_repo}" ]] ||
  fail "self-test: the passing fixture repository was not created"
if (gate_rejects "${passing_repo}" 'a clean repository' 'ui removal reference: ok') >/dev/null 2>&1; then
  fail "self-test: gate_rejects counted a PASSING run as a rejection because the expected message appeared in its ok output; its exit-status assertion is decorative"
fi
pass "self-test: gate_rejects refuses a passing run whose output contains the expected message"

# The upward walk `git rev-parse` performs by default is the trap here: this directory
# IS inside a work tree, just not at its root, and without the equality check the gate
# would happily scan the parent fixture's files and call that a pass.
nested_dir="${fixture}/not-the-root"
mkdir -p "${nested_dir}"
gate_rejects "${nested_dir}" 'the scan root is a subdirectory of a repository, not its root' \
  'is not the root of its git repository'

# F2/F3: the read-side twin of the data-loss hazard, and the reason the `unset` is
# repeated inside check_ui_removal. With `GIT_DIR` or `GIT_INDEX_FILE` exported,
# `ls-files` reads a FOREIGN index while `rev-parse --show-toplevel` still answers
# with the cwd -- so the toplevel-equality guard does not merely miss this case, it
# affirmatively certifies it. Remove either `unset` and these two assertions red.
poisoned_repo="$(new_fixture_repo poisoned)"
[[ -n "${poisoned_repo}" && -d "${poisoned_repo}" ]] ||
  fail "self-test: the poisoned-environment fixture repository was not created"
printf 'image: ghcr.io/platformrelay/kollect-ui:v1\n' >"${poisoned_repo}/bad.md"
git -C "${poisoned_repo}" add bad.md ||
  fail "self-test: could not stage the poisoned-environment mutant"
# Unpoisoned this fixture must already red, otherwise the two assertions below would
# be satisfied by a repository that has nothing to find.
gate_rejects "${poisoned_repo}" 'the poisoned-environment fixture tracks UI wiring' \
  'residual product UI references remain'

gate_rejects_under_git_env() {
  local root="$1" var="$2" value="$3" output status=0

  output="$( (export "${var}=${value}" && check_ui_removal "${root}") 2>&1 )" || status=$?
  [[ ${status} -ne 0 ]] ||
    fail "self-test: the gate passed on a repository that tracks UI wiring while ${var} pointed at a different repository; it scanned a foreign index and reported that as a clean scan"
  [[ "${output}" == *'residual product UI references remain'* ]] ||
    fail "self-test: the gate failed under a hostile ${var} but not for the intended reason; expected the residual-reference failure, got: ${output}"
  pass "self-test: gate ignores a hostile ${var} and scans the repository actually under test"
}
gate_rejects_under_git_env "${poisoned_repo}" GIT_DIR "${fixture}/.git"
gate_rejects_under_git_env "${poisoned_repo}" GIT_INDEX_FILE "${fixture}/.git/index"

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

# Two checks older than this self-test and never exercised by it: the mkdocs nav check
# and the webhook line of the allowlist awk. Both survived mutation because nothing
# reached them -- on the real tree the content scan finds no hits at all, so the awk
# filters see an empty stream and the nav regex sees a clean mkdocs.yml.

# The nav check runs AFTER the content scan, so the obvious fixture (a nav entry for
# `operator-manual/ui.md`) reds one line earlier and proves nothing about the nav check.
# A bare ADR number is the discriminating shape: the nav regex matches on `0408-`
# alone, while the content pattern needs `0408-read-api-ui`.
nav_repo="$(new_fixture_repo nav)"
[[ -n "${nav_repo}" && -d "${nav_repo}" ]] ||
  fail "self-test: the mkdocs-nav fixture repository was not created"
printf 'site_name: fixture\nnav:\n  - Legacy: adr/0408-kept-out-of-the-content-pattern.md\n' \
  >"${nav_repo}/mkdocs.yml"
git -C "${nav_repo}" add mkdocs.yml ||
  fail "self-test: could not stage the mkdocs-nav mutant"
# Not vacuous: if the content pattern matched this line too, the rejection below would
# come from the scan and the nav check would still be untested.
! grep -Eq -e "${pattern}" "${nav_repo}/mkdocs.yml" ||
  fail "self-test: the nav fixture also matches the content pattern, so the rejection would not come from the nav check"
gate_rejects "${nav_repo}" 'mkdocs.yml navigates to a removed UI ADR' \
  'still navigates to removed UI pages'

# The webhook allowlist, stated as behaviour rather than left implicit: a hit line that
# mentions a webhook and none of the three product-image literals is dropped. That is a
# deliberate false negative -- `task ui-` next to `webhook` passes -- and pinning it
# here is the point: the exemption is now visible and any widening of it reds.
webhook_repo="$(new_fixture_repo webhook)"
[[ -n "${webhook_repo}" && -d "${webhook_repo}" ]] ||
  fail "self-test: the webhook-allowlist fixture repository was not created"
printf 'The webhook certificate job used to be driven by task ui-build.\n' \
  >"${webhook_repo}/webhook-notes.md"
git -C "${webhook_repo}" add webhook-notes.md ||
  fail "self-test: could not stage the webhook-allowlist fixture"
# Not vacuous: the line has to be a real scan hit, otherwise accepting it says nothing
# about the awk rule -- it would just be a file the pattern never matched.
grep -Eq -e "${pattern}" "${webhook_repo}/webhook-notes.md" ||
  fail "self-test: the webhook fixture line is not a scan hit, so accepting it proves nothing about the allowlist"
gate_accepts "${webhook_repo}" 'a hit line mentioning a webhook and no product-image literal'

printf 'ui removal reference: self-test ok\n'
