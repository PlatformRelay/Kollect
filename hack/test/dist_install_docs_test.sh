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
# registration completed 2026-08-18 (repo `kollect`, oci://ghcr.io/platformrelay/kollect,
# Verified Publisher active), so its badge is unambiguously legitimate and is asserted PRESENT.
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

grep -Fq "${OLM_DOCS_DESTINATION}" "${README}" ||
  fail "${README} must point its OLM/OperatorHub badge at a live destination (${OLM_DOCS_DESTINATION}); the operatorhub.io listing does not exist yet, so the badge cannot link to it"

# The fragment above is a heading slug on the install page, and nothing else checks it: the badge is
# raw HTML, which hack/test/repo_root_links_test.sh skips by design; `mkdocs build --strict` never
# reads README.md; and markdownlint's MD051 (link-fragments) is disabled repo-wide. Rename the
# heading and the badge would still resolve -- to the top of the page instead of the section it
# promises. Tie the two together so the rename fails here instead of rotting silently.
grep -Fq '## Discoverability on package hubs' "${INSTALL}" ||
  fail "${INSTALL} must keep the '## Discoverability on package hubs' heading that the README OLM badge targets (${OLM_DOCS_DESTINATION}); renaming it breaks the badge fragment"

for file in "${INSTALL}" "${README}"; do
  ! grep -Fq "${OPERATORHUB_PACKAGE_URL}" "${file}" ||
    fail "${file} links to the OperatorHub.io package page, which answers HTTP 200 and then renders \"can't find package kollect\" -- k8s-operatorhub/community-operators#9070 is still waiting on a maintainer to approve its action_required workflow runs. Restore this link only once the listing is actually live."
done

pass "install docs describe hub paths without premature listing URLs"

echo "All dist install doc tests passed."
