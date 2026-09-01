#!/usr/bin/env bash
# DIST-AH-02: release workflow pushes Artifact Hub metadata via oras and keeps DR-FIND-07 guard.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/release.yaml"
REPO_YML="${ROOT}/artifacthub-repo.yml"

fail() {
  printf 'dist artifacthub release: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${WORKFLOW}" ]] || fail "${WORKFLOW} is missing"
[[ -f "${REPO_YML}" ]] || fail "${REPO_YML} is missing"

# repositoryID must be a well-formed UUID. Both the pre-registration placeholder
# (00000000-0000-4000-8000-000000000000) and the real ID pasted from the Artifact
# Hub control panel after registration are valid — this gate must never block its
# own remediation. The "registration still pending" fact is documented, with a
# date, in artifacthub-repo.yml itself and in ADR-0708.
REPOSITORY_ID="$(grep -E '^repositoryID:[[:space:]]*' "${REPO_YML}" | head -1 |
  sed -E 's/^repositoryID:[[:space:]]*//; s/[[:space:]]*$//; s/^["'"'"']//; s/["'"'"']$//')"
[[ -n "${REPOSITORY_ID}" ]] || fail "artifacthub-repo.yml has no repositoryID"
[[ "${REPOSITORY_ID}" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]] ||
  fail "artifacthub-repo.yml repositoryID '${REPOSITORY_ID}' is not a well-formed UUID"

pass "artifacthub-repo.yml repositoryID is a well-formed UUID"

# The five checks below are deliberately TEXT-LEVEL whole-file greps, and that is safe here in a
# way it is not for the ordering assertion further down. They are PRESENCE checks: they answer "does
# this workflow still mention X at all", and a comment mentioning X can only make such a check pass
# for the wrong reason -- never fail for the wrong reason. The cost of that is bounded (a workflow
# that lost `oras push` but kept a comment about it would slip through) and the structural
# assertions below close it: `Push Artifact Hub metadata` is resolved by step name, its index is
# compared, and its `run:` is required to actually invoke `oras push`. So text-level here buys
# readable failure messages against a real risk of the whole wiring being deleted, while nothing
# depends on a line number.
#
# The distinction that matters: a whole-file grep may back an assertion whose meaning is "this
# string exists", never one whose meaning is "this string is in a particular PLACE". Anything
# positional must be resolved through the parsed step graph -- see the ordering block below.
grep -Fq 'setup-oras' "${WORKFLOW}" ||
  fail "release workflow must set up oras"

grep -Fq 'oras push' "${WORKFLOW}" ||
  fail "release workflow must oras push Artifact Hub metadata"

grep -Fq ':artifacthub.io' "${WORKFLOW}" ||
  fail "release workflow must push the artifacthub.io metadata tag on the chart repo"

grep -Fq 'artifacthub-repo.yml' "${WORKFLOW}" ||
  fail "release workflow must reference artifacthub-repo.yml"

grep -Fq 'Guard against image/chart GHCR tag collision (DR-FIND-07)' "${WORKFLOW}" ||
  fail "release workflow must retain DR-FIND-07 guard"

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the release job step graph"

step_index() {
  yq eval ".jobs.release.steps | to_entries | map(select(.value.name == \"$1\")) | .[0].key" "${WORKFLOW}"
}

step_soft_fail() {
  yq eval ".jobs.release.steps[] | select(.name == \"$1\") | .[\"continue-on-error\"]" "${WORKFLOW}"
}

# True when the named step's `run:` script actually invokes the given command at the start of a
# script line. Scoped to one step and anchored, so neither a step name nor a comment elsewhere in
# the workflow -- nor a `#` comment inside this step's own script -- can stand in for the command.
step_runs() {
  yq eval ".jobs.release.steps[] | select(.name == \"$1\") | (.run // \"\") | test(\"(^|\n)[[:space:]]*$2\")" "${WORKFLOW}"
}

ORAS_STEP="Set up oras"
PUSH_STEP="Push Artifact Hub metadata"
REPORT_STEP="Report Artifact Hub metadata push outcome"
PUBLISH_STEP="Publish GitHub Release"
GUARD_STEP="Guard against image/chart GHCR tag collision (DR-FIND-07)"

oras_idx="$(step_index "${ORAS_STEP}")"
push_idx="$(step_index "${PUSH_STEP}")"
report_idx="$(step_index "${REPORT_STEP}")"
publish_idx="$(step_index "${PUBLISH_STEP}")"
guard_idx="$(step_index "${GUARD_STEP}")"

for pair in "${ORAS_STEP}:${oras_idx}" "${PUSH_STEP}:${push_idx}" "${REPORT_STEP}:${report_idx}" "${PUBLISH_STEP}:${publish_idx}" "${GUARD_STEP}:${guard_idx}"; do
  name="${pair%:*}"
  idx="${pair##*:}"
  [[ -n "${idx}" && "${idx}" != "null" ]] ||
    fail "release job has no step named '${name}' (step-graph assertions below would pass vacuously)"
done

# The Artifact Hub metadata push must run AFTER the DR-FIND-07 collision guard: the guard is what
# proves the chart and the image did not land on the same coordinate, and publishing repository
# metadata for a repository in that state advertises a broken artifact.
#
# Resolved through the parsed step graph -- step INDEX by step NAME, and then the resolved step's
# own `run:` script -- deliberately.
#
# The previous implementation was `grep -n 'oras push' "${WORKFLOW}" | head -1` compared against the
# guard's grepped line number. That reds on any earlier line anywhere in the file that contains the
# words, and it did: a lane added an explanatory comment mentioning `oras push` to the chart
# publishing step, nowhere near the command, and this gate failed claiming the metadata push ran
# before the guard -- a failure message naming a problem that did not exist, which costs more than
# a miss. A line number is not a step order; it only correlates with one until someone writes a
# sentence. Do not reintroduce a positional whole-file grep here.
#
# The `run:` match is anchored at the start of a script line (`(^|\n)[[:space:]]*`), so a `#`
# comment inside that step's own script -- comments inside a block scalar are script text, not YAML
# comments, and yq therefore hands them to `test()` -- cannot satisfy it either.
[[ "${guard_idx}" -lt "${push_idx}" ]] ||
  fail "'${PUSH_STEP}' (index ${push_idx}) must run AFTER '${GUARD_STEP}' (index ${guard_idx}) -- Artifact Hub metadata must not be published for a repository whose image/chart collision guard has not passed"

[[ "$(step_runs "${PUSH_STEP}" 'oras push')" == "true" ]] ||
  fail "'${PUSH_STEP}' must actually invoke \`oras push\` in its run script -- otherwise the ordering assertion above is about a step that no longer pushes anything, and the whole-file presence grep for 'oras push' near the top could be satisfied by a comment alone"

pass "release workflow wires oras push after DR-FIND-07 guard"

# ADR-0708: "soft-fail hub jobs preserve tag-release success". Artifact Hub
# repository metadata is discoverability only, so it must never be able to abort
# the release job after the images and chart are already pushed and cosign-signed
# (which would leave a tag with signed GHCR artifacts but no GitHub Release).
[[ "${publish_idx}" -lt "${oras_idx}" ]] ||
  fail "'${ORAS_STEP}' (index ${oras_idx}) must run AFTER '${PUBLISH_STEP}' (index ${publish_idx})"
[[ "${publish_idx}" -lt "${push_idx}" ]] ||
  fail "'${PUSH_STEP}' (index ${push_idx}) must run AFTER '${PUBLISH_STEP}' (index ${publish_idx})"
[[ "${publish_idx}" -lt "${report_idx}" ]] ||
  fail "'${REPORT_STEP}' (index ${report_idx}) must run AFTER '${PUBLISH_STEP}' (index ${publish_idx})"

# Every step placed after the signed publish must be soft-fail, the reporting
# step included -- a hard-failing reporter would reintroduce exactly the failure
# mode this ordering exists to prevent.
for name in "${ORAS_STEP}" "${PUSH_STEP}" "${REPORT_STEP}"; do
  [[ "$(step_soft_fail "${name}")" == "true" ]] ||
    fail "'${name}' must declare continue-on-error: true so an Artifact Hub failure cannot fail the release job"
done

pass "Artifact Hub steps run after the GitHub Release publish and are soft-fail"

# Soft-fail must not be silent. `continue-on-error` pins steps.<id>.conclusion to
# "success", so the reporting step has to read steps.<id>.outcome.
grep -Fq 'steps.push-artifacthub-metadata.outcome' "${WORKFLOW}" ||
  fail "release workflow must read steps.push-artifacthub-metadata.outcome (conclusion is always 'success' under continue-on-error)"
grep -Fq '::warning title=Artifact Hub metadata not published' "${WORKFLOW}" ||
  fail "a failed Artifact Hub push must emit a ::warning:: annotation"
grep -Fq 'Artifact Hub repository metadata was NOT published' "${WORKFLOW}" ||
  fail "a failed Artifact Hub push must write a GITHUB_STEP_SUMMARY line"

pass "failed Artifact Hub push is reported via ::warning:: and job summary"

# DIST-AH-03 / ADR-0709: the `ignore` list is DELETED, and its absence is now the contract.
#
# It used to suppress the v-prefixed image tags and the broken 0.9.0-0.13.0 charts that shared the
# old single-repository path. At ghcr.io/platformrelay/charts/kollect the regex would match nothing:
# no `v*` tags are pushed there, and 0.9.0-0.13.0 are deleted from the registry and must never be
# republished (they hardcode `image.tag: latest`, a tag that was never pushed, so they cannot
# install). A regex that matches nothing is not a safety net, it is a decoy -- it reads as if it
# were suppressing something, which is precisely how it was tuned twice for a job it cannot do.
#
# Asserted as ABSENCE OF THE KEY rather than as "the regex is empty", because the failure mode is
# someone re-adding an entry to silence a future `error preparing package` mail. That cannot work:
# Artifact Hub loads every semver-ish tag BEFORE applying the ignore list, so ignore filters
# indexing and never loading (ADR-0709, "Why no Artifact Hub setting fixes this").
has_ignore="$(yq eval 'has("ignore")' "${REPO_YML}")"
[[ "${has_ignore}" == "false" ]] ||
  fail "artifacthub-repo.yml must NOT carry an \`ignore\` key (ADR-0709): at ghcr.io/platformrelay/charts/kollect it would match nothing, and it cannot suppress \`error preparing package\` mail because Artifact Hub applies ignore to INDEXING, after every tag has already been loaded"

pass "artifacthub-repo.yml carries no ignore list (ADR-0709: it would match nothing and cannot silence load errors)"

# Verified Publisher requires BOTH repositoryID (asserted as a UUID above) and an owner whose email
# matches the Artifact Hub account. Deleting `ignore` must not take `owners` with it -- both live in
# the same small file and the same edit, and losing owners silently drops the Verified badge with no
# other signal. Asserted through yq on the parsed document, so a commented-out owners block reds.
OWNER_NAME="$(yq eval '.owners[0].name // ""' "${REPO_YML}")"
OWNER_EMAIL="$(yq eval '.owners[0].email // ""' "${REPO_YML}")"
[[ -n "${OWNER_NAME}" ]] ||
  fail "artifacthub-repo.yml must keep an owners[0].name -- Verified Publisher needs the owners block"
[[ "${OWNER_EMAIL}" == *@*.* ]] ||
  fail "artifacthub-repo.yml owners[0].email ('${OWNER_EMAIL}') must be an email address matching the Artifact Hub account that owns the repository -- Artifact Hub compares the two directly and silently drops Verified Publisher when they differ"

pass "artifacthub-repo.yml keeps the owners block Verified Publisher requires"

# The three facts a reader needs before touching this file again. They are asserted, not merely
# written, because deleting `ignore` removes the only place those facts were recorded, and the
# expensive mistake here is a re-learned one: two earlier sessions tuned the regex against errors it
# structurally cannot suppress. Each pattern is anchored to a COMMENT line (`^#`), so moving a fact
# into a YAML value or dropping it into a commit message does not satisfy the gate.
#
# Deliberately phrase-level, not word-level: rewording the file is fine, but a rewrite that loses
# "indexing, never loading" has lost the only sentence that stops the next tuning attempt.
grep -Eiq '^#.*charts only' "${REPO_YML}" ||
  fail "artifacthub-repo.yml must state in a comment that this repository path holds CHARTS ONLY -- that separation is the whole point of ADR-0709"
grep -Eiq '^#.*DR-FIND-07' "${REPO_YML}" ||
  fail "artifacthub-repo.yml must name DR-FIND-07 in a comment: the image/chart tag collision still exists, permanently, at the OTHER path"
grep -Eq '^#.*ghcr\.io/platformrelay/kollect([^/]|$)' "${REPO_YML}" ||
  fail "artifacthub-repo.yml must name the other path (ghcr.io/platformrelay/kollect) in a comment, so the next reader knows which coordinate the DR-FIND-07 collision lives at and that the controller image never moved"
grep -Eiq '^#.*indexing,? never loading' "${REPO_YML}" ||
  fail "artifacthub-repo.yml must record, in a comment, that \`ignore\` filters INDEXING and never LOADING -- without that sentence the next \`error preparing package\` mail gets answered with another ignore regex, which cannot work"

pass "artifacthub-repo.yml records why ignore is gone and why re-adding one cannot work"

echo "All dist Artifact Hub release tests passed."
