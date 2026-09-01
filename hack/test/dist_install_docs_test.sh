#!/usr/bin/env bash
# DIST-DOC-01: install docs mention hub paths without live 404 badge URLs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INSTALL="${ROOT}/docs/getting-started/install.md"
README="${ROOT}/README.md"

fail() {
  printf 'dist install docs: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${INSTALL}" ]] || fail "${INSTALL} is missing"
[[ -f "${README}" ]] || fail "${README} is missing"

for file in "${INSTALL}" "${README}"; do
  grep -Eiq 'artifact hub|artifacthub' "${file}" ||
    fail "${file} must mention Artifact Hub discoverability"
  grep -Eiq 'operatorhub|operator hub' "${file}" ||
    fail "${file} must mention OperatorHub discoverability"
done

# ADR-0708 forbids shipping a hub URL that does not resolve to a live listing. Artifact Hub
# registration completed 2026-08-18 (repo `kollect`; chart coordinate moved to
# oci://ghcr.io/platformrelay/charts/kollect by ADR-0709), Verified Publisher active, so its
# badge is unambiguously legitimate and is asserted PRESENT.
#
# DIST-OH-05: the OperatorHub.io package deep link is NOT legitimate, and the rationale that
# shipped it has been falsified. It went out ahead of the listing on the premise that
# operatorhub.io "soft-404s" -- serving HTTP 200 with a generic landing page for unknown
# operators. It does not. The package URL answers HTTP 200 and then CLIENT-RENDERS
# "can't find package kollect": a broken-looking dead end, which is exactly the user-visible
# outcome ADR-0708 set out to prevent. HTTP status is not evidence that a listing exists.
#
# Why the listing does not exist, recorded here so the next person does not re-litigate it: the
# upstream submission k8s-operatorhub/community-operators#9070 ("operator [N] [CI] kollect
# (0.18.0)", head 821cf2da) has been OPEN since 2026-08-18 with its "Operator test" and "DCO
# test" workflow runs at conclusion=action_required. That is GitHub's first-time-contributor
# "Approve and run" gate; only a k8s-operatorhub MAINTAINER can clear it. Everything on our side
# is green (operator-ci, operator-automerge-enabled, DCO), the authorized-changes / new-operator
# / allow-operator-recreate labels are applied, and the PR timeline carries no bot request, no
# review and no maintainer activity since 2026-08-19. Nothing is being asked of us, and
# resubmitting does not clear an action_required run.
#
# The OpenShift sibling redhat-openshift-ecosystem/community-operators-prod#10889 (0.18.0) IS
# merged, so the OLM bundle is genuinely live in that community catalog. That is why the badge
# is RE-POINTED rather than deleted: the OLM install path is real; only the operatorhub.io
# listing is not.
#
# So this is a two-sided contract, not a deletion check:
#   * README must keep an OLM/OperatorHub badge pointed at a destination that is live today --
#     the install page's hub section on the published docs site;
#   * neither README nor the install page may carry the operatorhub.io package deep link until
#     #9070 merges.
# When it merges: re-point the badge at the listing and turn the absence assertion below back
# into a presence assertion.
#
# Scope note: only these two files are scanned. They are the ones this gate owns and the ones a
# user actually follows; CHANGELOG.md and ADR-0708 may name the URL as history or as evidence.
OPERATORHUB_PACKAGE_URL='operatorhub.io/operator/kollect'
OLM_DOCS_DESTINATION='getting-started/install/#discoverability-on-package-hubs'

grep -Fq 'artifacthub.io/badge/repository/kollect' "${README}" ||
  fail "${README} must carry the Artifact Hub badge (repository is registered)"

# Anchored on the badge ANCHOR MARKUP, not on the bare string. An earlier revision only required
# the destination to appear somewhere in README.md, so deleting the badge outright and pasting the
# URL as prose passed a check whose message claimed to be about the badge -- an assertion claiming
# more than it checked. Every badge in that header block is a one-line `<a href="..."><img ...>`, so
# matching that shape checks the thing the message names. Case-SENSITIVE on purpose: GitHub Pages
# paths and HTML fragment ids are both case-sensitive, so a case variant here is a genuinely broken
# destination and must red.
grep -Eq "<a href=\"[^\"]*${OLM_DOCS_DESTINATION}\"><img " "${README}" ||
  fail "${README} must carry the OLM/OperatorHub BADGE (an <a href=...><img ...> in the badge header) pointing at a live destination (${OLM_DOCS_DESTINATION}); the operatorhub.io listing does not exist yet, so the badge cannot link to it"

# The fragment above is a heading slug on the install page, and nothing else checks it: the badge is
# raw HTML, which hack/test/repo_root_links_test.sh skips by design; `mkdocs build --strict` never
# reads README.md; and markdownlint's MD051 (link-fragments) is disabled repo-wide. Rename the
# heading and the badge would still resolve -- to the top of the page instead of the section it
# promises. Tie the two together so the rename fails here instead of rotting silently.
# ANCHORED to the whole heading line, because the slug is computed from the whole heading line. An
# unanchored substring match let a SUFFIX through -- `## Discoverability on package hubs (OLM)` and
# `... and registries` both contain the string, both passed, and both change the slug, killing the
# badge fragment while this gate stayed green. `^#+ ` keeps the level free (## or ###: the slug does
# not depend on it) and `[[:space:]]*$` forbids anything trailing. Case-INSENSITIVE because
# python-markdown's slugifier lowercases, so ALLCAPS or Title Case yield the same slug and must not
# red. Net: this assertion pins the heading to the exact text whose slug the badge fragment depends
# on -- green iff the slug is preserved, red on every edit that moves it.
grep -Eiq '^#+ Discoverability on package hubs[[:space:]]*$' "${INSTALL}" ||
  fail "${INSTALL} must keep a heading that reads exactly 'Discoverability on package hubs', with nothing appended -- the README OLM badge targets its slug (${OLM_DOCS_DESTINATION}), and any renaming or suffix changes that slug and silently drops the reader at the top of the page"

for file in "${INSTALL}" "${README}"; do
  # Case-INSENSITIVE, and that is load-bearing rather than cosmetic: host names are
  # case-insensitive, so `https://OperatorHub.io/operator/kollect` is the SAME dead page. A
  # case-sensitive absence check is a one-keystroke bypass of the whole gate -- proven by a review
  # mutation that re-added the dead link with a capital O/H and passed GREEN.
  ! grep -Fiq "${OPERATORHUB_PACKAGE_URL}" "${file}" ||
    fail "${file} links to the OperatorHub.io package page, which answers HTTP 200 and then renders \"can't find package kollect\" -- k8s-operatorhub/community-operators#9070 is still waiting on a maintainer to approve its action_required workflow runs. Restore this link only once the listing is actually live."
done

pass "install docs describe hub paths without premature listing URLs"

# DIST-AH-03 / ADR-0709: the Helm CHART lives at oci://ghcr.io/platformrelay/charts/kollect.
# The controller IMAGE does NOT move -- it stays at ghcr.io/platformrelay/kollect with v-prefixed
# tags, because its digest is pinned immutably in OLM bundles already merged into the community
# catalogs. Until this gate existed the install coordinate was named only in a comment and never
# asserted, so every doc could drift to a coordinate that does not resolve and nothing would red.
#
# The whole difficulty is that both artifacts share the string `ghcr.io/platformrelay/kollect`, and
# the docs below carry MANY legitimate controller-image references to it (image.repository defaults,
# the DR-FIND-07 tag table in RELEASE.md, the lab-registry mirror notes). So every assertion here
# anchors on the `oci://` scheme, which only ever prefixes a CHART reference: `helm install`,
# `helm upgrade`, `helm pull`, `helm show`. An unanchored grep for the bare repo path would red on
# the image references it must leave alone -- which is exactly the blind-sed defect this pair of
# assertions is built to catch, from both directions.
CHART_OCI='oci://ghcr.io/platformrelay/charts/kollect'
CHART_OCI_RE='oci://ghcr\.io/platformrelay/charts/kollect'
CHART_OCI_OLD='oci://ghcr.io/platformrelay/kollect'
CONTROLLER_IMAGE_REPO='ghcr.io/platformrelay/kollect'
# The controller image path as its own repository: `platformrelay/kollect` followed by a delimiter,
# never by another path segment. The trailing class excludes `/` (so `platformrelay/charts/kollect`
# cannot match), `-` and alphanumerics (so `kollect-pipeline` cannot), and allows `:`, `@`, a
# backtick, whitespace or end-of-line -- every way these docs actually terminate the reference.
CONTROLLER_IMAGE_RE='ghcr\.io/platformrelay/kollect([^/[:alnum:]-]|$)'

# Every doc that hands a reader an OCI chart reference. Scoped deliberately: docs/adr/*.md quote the
# OLD coordinate as history and as a verbatim Artifact Hub error message, and CHANGELOG.md records
# it as shipped fact -- rewriting either would falsify the record, so neither is scanned.
#
# Split by what each file OWES the reader, because a single presence check cannot be honest about
# both kinds. INSTALL_COMMAND_DOCS put a runnable `helm install`/`helm upgrade` line in front of the
# reader; MENTION_DOCS name the coordinate as a fact (README's install-paths sentence, RELEASE.md's
# published-outputs table) and have no command to run.
INSTALL_COMMAND_DOCS=(
  "${INSTALL}"
  "${ROOT}/docs/COMMAND-REFERENCE.md"
  "${ROOT}/docs/operator-manual/index.md"
  "${ROOT}/docs/operator-manual/upgrading.md"
  "${ROOT}/docs/operator-manual/load-test-runbook.md"
)
MENTION_DOCS=(
  "${README}"
  "${ROOT}/docs/RELEASE.md"
)
COORDINATE_DOCS=("${INSTALL_COMMAND_DOCS[@]}" "${MENTION_DOCS[@]}")

# PRESENCE, before the absence check, so "the old coordinate is gone" cannot be satisfied by
# deleting the coordinate outright.
#
# Anchored to the COMMAND, not to the bare string, and that anchoring is the whole point. Three of
# these files name the new coordinate twice -- once in a `helm install`/`helm upgrade` snippet and
# once in prose explaining the move -- so a bare `grep -F` for the coordinate stayed green after the
# runnable command was deleted, leaving only the prose. Proven by mutation on install.md,
# operator-manual/index.md and upgrading.md. `helm (install|upgrade)` and the coordinate must sit on
# ONE line, which every snippet in these files does (continuations break after the coordinate).
for file in "${INSTALL_COMMAND_DOCS[@]}"; do
  [[ -f "${file}" ]] ||
    fail "${file} is missing (the chart-coordinate assertions below would pass vacuously)"
  grep -Eq "helm (install|upgrade)[^[:space:]]*.*${CHART_OCI_RE}" "${file}" ||
    fail "${file} must hand the reader a runnable \`helm install\`/\`helm upgrade\` line against ${CHART_OCI} (ADR-0709) -- a prose mention of the coordinate is not an install path, and this file is listed because it carries a command today"
done

# The mention-only docs owe the string, not a command: README's install-paths sentence and
# RELEASE.md's "what CI publishes" table row. Kept as a separate list so the assertion above can
# stay strict rather than being weakened to accommodate two files that legitimately have no snippet.
for file in "${MENTION_DOCS[@]}"; do
  [[ -f "${file}" ]] ||
    fail "${file} is missing (the chart-coordinate assertions below would pass vacuously)"
  grep -Fq "${CHART_OCI}" "${file}" ||
    fail "${file} must name the chart coordinate ${CHART_OCI} (ADR-0709)"
done

for file in "${COORDINATE_DOCS[@]}"; do
  # ABSENCE. Case-INSENSITIVE because registry hosts are case-insensitive, so
  # `OCI://GHCR.IO/platformrelay/kollect` is the same stale coordinate; a case-sensitive check here
  # would be a one-keystroke bypass. Note the new coordinate does NOT contain the old one as a
  # substring (`charts/` sits between owner and chart name), so this is exact: it reds only on a
  # genuinely stale chart reference. It also catches the pre-ADR-0709 typo
  # `oci://ghcr.io/platformrelay/kollect/charts/kollect`, a path that has never existed.
  #
  # Consequence for prose, and it is deliberate: these docs explain the migration WITHOUT
  # reproducing the old coordinate under an `oci://` scheme -- they write it as
  # "ghcr.io/platformrelay/kollect (no `charts/` segment)", which is the same path the controller
  # image is named by anyway. A copy-pasteable `oci://` line is an instruction whatever sentence
  # surrounds it, and a reader skimming for the command does not read the caveat. The history
  # belongs in ADR-0709 and CHANGELOG.md, which this gate does not scan.
  ! grep -Fiq "${CHART_OCI_OLD}" "${file}" ||
    fail "${file} still installs the chart from ${CHART_OCI_OLD}; ADR-0709 moved it to ${CHART_OCI} because Artifact Hub indexes one chart per repository and the old path also serves the controller image"

  # The other direction, and the reason this gate is a pair rather than a single grep: a blind
  # `sed s#platformrelay/kollect#platformrelay/charts/kollect#` over these files would satisfy both
  # assertions above while silently repointing the controller image at a path that holds no images.
  # A v-prefixed tag and an `image.repository` value are image-only, so either under `charts/` is
  # that defect on sight.
  #
  # The `image.repository` half is a LOOSE proximity match, not a `key = value` match, because the
  # docs state it in prose as often as in a snippet -- upgrading.md reads
  # "`image.repository` stays `ghcr.io/platformrelay/kollect`". The earlier version required `=` or
  # `:` immediately after the key and therefore missed exactly that sentence: a sed over
  # upgrading.md alone passed green while corrupting the image coordinate. 40 characters is enough
  # slack for the prose forms in use and short enough that an unrelated mention of the chart path
  # later on the same line does not collide.
  ! grep -Eiq "ghcr\.io/platformrelay/charts/kollect:v[0-9]|image\.repository.{0,40}ghcr\.io/platformrelay/charts/kollect" "${file}" ||
    fail "${file} points the CONTROLLER IMAGE at ghcr.io/platformrelay/charts/kollect; ADR-0709 moves only the chart -- the image stays at ${CONTROLLER_IMAGE_REPO} with v-prefixed tags because its digest is pinned immutably in already-merged OLM bundles"
done

# The general backstop, and the one that does not depend on guessing which shapes a blind sed
# produces: every doc that names the controller image today must STILL name it as its own
# repository. A `sed s#platformrelay/kollect#platformrelay/charts/kollect#g` rewrites every
# occurrence in the file, so this reds on the mutation even where no v-tag and no
# `image.repository` happens to sit near the corrupted string -- the case that let a sed over
# operator-manual/index.md alone pass green.
#
# load-test-runbook.md is deliberately absent: it references the chart only, so it has no image
# coordinate to protect and requiring one would be a lie about its content.
IMAGE_PATH_DOCS=(
  "${README}"
  "${INSTALL}"
  "${ROOT}/docs/COMMAND-REFERENCE.md"
  "${ROOT}/docs/operator-manual/index.md"
  "${ROOT}/docs/operator-manual/upgrading.md"
  "${ROOT}/docs/RELEASE.md"
)
for file in "${IMAGE_PATH_DOCS[@]}"; do
  grep -Eq "${CONTROLLER_IMAGE_RE}" "${file}" ||
    fail "${file} no longer names the controller image repository ${CONTROLLER_IMAGE_REPO} as its own path -- ADR-0709 moves only the chart, so a rewrite that took the image with it (typically a blind sed over platformrelay/kollect) has corrupted this file"
done

# Narrower, and kept because it pins the TAG CONVENTION rather than the path: RELEASE.md is where
# the v-prefix is stated as a published fact, and DR-FIND-07 is entirely about that prefix.
grep -Fq "${CONTROLLER_IMAGE_REPO}:v" "${ROOT}/docs/RELEASE.md" ||
  fail "${ROOT}/docs/RELEASE.md must keep naming the controller image as ${CONTROLLER_IMAGE_REPO}:v<version> -- the image does not move under ADR-0709"

pass "docs install the chart from ${CHART_OCI} and leave the controller image at ${CONTROLLER_IMAGE_REPO}"

echo "All dist install doc tests passed."
