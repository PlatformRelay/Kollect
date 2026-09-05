#!/usr/bin/env bash
# DIST-AH-03: static + offline-behavioural gate on hack/migrate-chart-path.sh.
#
# WHY THIS GATE IS SHAPED THE WAY IT IS
# -------------------------------------
# The script under test performs live writes to GHCR. It cannot run in CI: it needs a
# GitHub token carrying `write:packages`, plus cosign/crane/oras on PATH, and its whole
# purpose is to mutate a public registry. So there is no end-to-end test to be had here
# and pretending otherwise would be worse than useless.
#
# What CAN be gated, and what this file gates, is the script's STRUCTURE and its SAFETY
# PROPERTIES -- the invariants whose violation would be discovered only by a maintainer
# running a hand-driven migration against a production registry, i.e. exactly the place
# where a mistake is unrecoverable. Concretely:
#
#   * the never-republish exclusion (charts 0.9.0-0.13.0) still holds;
#   * nothing in the script can remove anything from any registry;
#   * the V1 signature-portability gate runs BEFORE the bulk copy, so a broken
#     assumption stops after one chart rather than after six;
#   * the visibility check runs BEFORE the metadata push, so the migration cannot end
#     "successful" while the package is private and the hub sees nothing;
#   * the cosign identity regexp matches the one the repo publishes in docs/RELEASE.md;
#   * the acceptance-criterion verifier cannot report PASS from a single sample.
#
# Where a property is observable without a network or a credential, this file asserts it
# BEHAVIOURALLY by executing the script in an offline mode rather than by grepping for a
# string -- a grep for `0.13.0` proves nothing about whether 0.13.0 is a copy target.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT}/hack/migrate-chart-path.sh"
RELEASE_DOC="${ROOT}/docs/RELEASE.md"

fail() {
  printf 'dist chart path migration: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${SCRIPT}" ]] || fail "${SCRIPT} is missing"

# Vacuity floor. Every assertion below is a grep or a mode invocation against this one
# file; if the file were emptied or truncated to a stub, most greps would simply find
# nothing and a naive `! grep` style check would go green. Establish up front that there
# is a real script here to assert against.
script_lines="$(wc -l <"${SCRIPT}")"
(( script_lines >= 200 )) ||
  fail "${SCRIPT} is only ${script_lines} lines -- too short to be the real migration script; every assertion below would be asserting against a stub"

head -1 "${SCRIPT}" | grep -Fq '#!/usr/bin/env bash' ||
  fail "${SCRIPT} must start with the repo-standard bash shebang"
grep -Fq 'set -euo pipefail' "${SCRIPT}" ||
  fail "${SCRIPT} must run under 'set -euo pipefail'"

pass "migration script exists and uses the repo-standard bash preamble"

# ---------------------------------------------------------------------------
# 1. Offline modes must work with no credentials, no network and no registry.
# ---------------------------------------------------------------------------
# Both `--help` and `--plan` are the seams the rest of this file uses to test behaviour
# instead of text. If either of them ever starts requiring a token or touching the
# network, this gate would start failing in CI for the right reason -- so assert it
# directly rather than discovering it as a mysterious CI red.
#
# Run with no registry token in scope. (An earlier revision of this comment also claimed
# PATH was reduced so a stray cosign/crane/oras/curl call would fail loudly. It was not --
# the code only unset the token variables. Section 8 below does put a controlled PATH in
# front of the script, with recording stubs, which is a stronger version of that idea; this
# section is credential-free only, and now says so.)
help_out=""
if ! help_out="$(env -u GHCR_TOKEN -u GITHUB_TOKEN -u CR_PAT bash "${SCRIPT}" --help 2>&1)"; then
  fail "'--help' must exit 0 without credentials; got:"$'\n'"${help_out}"
fi
grep -Fq -- '--apply' <<<"${help_out}" ||
  fail "'--help' must document the --apply flag"
grep -Fq -- '--dry-run' <<<"${help_out}" ||
  fail "'--help' must document the --dry-run flag"
grep -Fq -- '--verify-ac1' <<<"${help_out}" ||
  fail "'--help' must document the --verify-ac1 acceptance-criterion mode"

# The default mode is the single most consequential decision in the script: an operator
# who types `bash hack/migrate-chart-path.sh` with no flags must get a rehearsal, not a
# live migration. This asserts the DOCUMENTED default only -- `--plan` says nothing about
# APPLY. The behaviour (a bare invocation executes zero mutating commands) is asserted in
# section 8, which is where the `APPLY=0` -> `APPLY=1` mutation is actually caught.
grep -Eiq 'default[^\n]*dry.run|dry.run[^\n]*default' <<<"${help_out}" ||
  fail "'--help' must state which mode is the default; an operator cannot be expected to guess whether a bare invocation writes to GHCR"

# An unrecognised flag must be rejected, not ignored. Catches the classic argument-parser
# mutation where the `*)` case falls through -- under which `--dryrun` (a plausible typo
# for --dry-run) would be silently swallowed and the script would run in whatever mode
# the default happened to be.
if env -u GHCR_TOKEN -u GITHUB_TOKEN -u CR_PAT bash "${SCRIPT}" --this-flag-does-not-exist >/dev/null 2>&1; then
  fail "an unrecognised flag must be rejected with a non-zero exit, not silently ignored"
fi

pass "--help works without credentials, documents the mode default, and unknown flags are rejected"

# ---------------------------------------------------------------------------
# 2. The version plan: what gets copied, and what must NEVER get copied.
# ---------------------------------------------------------------------------
# ADR-0709 / artifacthub-repo.yml: charts 0.9.0-0.13.0 shipped `image.tag: latest`, a tag
# that was never pushed to GHCR. They cannot install -- the manager pod sits in
# ImagePullBackOff -- and they were taken out of the registry on purpose. Republishing
# them to the new path would resurrect five uninstallable listings AND the recurring
# Artifact Hub scan-failure mail that ADR-0709 exists to end.
#
# This is asserted against the script's OWN rendering of its plan, not against its source
# text, because the mutation being defended against is a well-meaning maintainer widening
# a loop bound ("0.9.0 through 0.19.0, tidier") -- a change that a source-level grep for
# the literal string `0.13.0` would not notice, since 0.13.0 already appears in the file
# as an exclusion.
plan_out=""
if ! plan_out="$(env -u GHCR_TOKEN -u GITHUB_TOKEN -u CR_PAT bash "${SCRIPT}" --plan 2>&1)"; then
  fail "'--plan' must exit 0 without credentials or network; got:"$'\n'"${plan_out}"
fi

mapfile -t copy_lines < <(grep -E '^plan: copy ' <<<"${plan_out}" || true)
mapfile -t excluded_lines < <(grep -E '^plan: excluded ' <<<"${plan_out}" || true)

# Non-vacuity: pin the COUNTS before testing membership. Without this, a `--plan` that
# printed nothing at all would satisfy every "0.13.0 is not a copy target" assertion.
(( ${#copy_lines[@]} == 6 )) ||
  fail "--plan must list exactly 6 copy targets (0.14.0 V1 gate + 0.15.0-0.19.0); found ${#copy_lines[@]}"
(( ${#excluded_lines[@]} == 5 )) ||
  fail "--plan must list exactly 5 excluded versions (0.9.0-0.13.0); found ${#excluded_lines[@]}"

for want in 0.14.0 0.15.0 0.16.0 0.17.0 0.18.0 0.19.0; do
  grep -Eq "^plan: copy ${want//./\\.}([^0-9]|$)" <<<"${plan_out}" ||
    fail "--plan must schedule chart ${want} for copy"
done

# The load-bearing half: none of the five broken charts may appear as a copy target. The
# check is against the copy lines only, so the exclusion list itself (which legitimately
# names all five) cannot make it pass.
for banned in 0.9.0 0.10.0 0.11.0 0.12.0 0.13.0; do
  for line in "${copy_lines[@]}"; do
    if grep -Eq "(^|[^0-9.])${banned//./\\.}([^0-9]|$)" <<<"${line}"; then
      fail "chart ${banned} appears as a COPY TARGET in --plan ('${line}') -- 0.9.0-0.13.0 hardcode image.tag=latest, cannot install, and must never be republished"
    fi
  done
  grep -Eq "^plan: excluded ${banned//./\\.}([^0-9]|$)" <<<"${plan_out}" ||
    fail "--plan must name ${banned} explicitly as excluded, with its reason -- an exclusion that is implicit in a loop bound is one refactor away from disappearing"
done

# ADR-0709: same exclusion, TWO different reasons, and the operator-facing text has to say
# so. 0.9.0/0.10.0/0.11.0/0.13.0 ARE charts that hardcode `image.tag: latest`. 0.12.0 is
# not a chart at all -- that release pushed the controller IMAGE to the chart's bare tag
# (artifacthub-repo.yml records it in the same words), and its GHCR package version is the
# protected 1087932920. A blanket "hardcodes image.tag: latest" across all five is a
# falsifiable claim about 0.12.0: a maintainer who checks its values.yaml, finds no such
# line and concludes the note is stale is exactly the reader the exclusion exists to stop.
reason_for() { grep -E "^plan: excluded ${1//./\\.} " <<<"${plan_out}" | sed -E "s/^plan: excluded [0-9.]+[[:space:]]+//"; }
r_0_9="$(reason_for 0.9.0)"
r_0_12="$(reason_for 0.12.0)"
[[ -n "${r_0_9}" && -n "${r_0_12}" ]] ||
  fail "could not extract exclusion reasons from --plan; the comparison below would pass vacuously"
[[ "${r_0_9}" != "${r_0_12}" ]] ||
  fail "--plan gives 0.12.0 the same exclusion reason as 0.9.0 ('${r_0_12}'). It is not the same reason: 0.12.0 is not a chart at all, it is the controller IMAGE published to the chart's bare tag. ADR-0709: same exclusion, two different reasons."
grep -Eiq 'image' <<<"${r_0_12}" ||
  fail "0.12.0's exclusion reason must say it is the controller IMAGE at the chart's bare tag, not a broken chart; got: '${r_0_12}'"
if grep -Fiq 'image.tag=latest' <<<"${r_0_12}"; then
  fail "0.12.0's exclusion reason repeats the 'image.tag=latest' claim, which is false for 0.12.0: it is not a chart, so it has no values.yaml in the registry at all"
fi
grep -Fiq 'latest' <<<"${r_0_9}" ||
  fail "0.9.0's exclusion reason must state the image.tag=latest cause; got: '${r_0_9}'"

# 0.14.0 is not just "the first copy": it is the V1 gate, the one chart whose copied
# signature is verified before five more are copied on the strength of that result. If the
# plan stops distinguishing it, the gate has been flattened into a plain bulk copy.
grep -Eq '^plan: copy 0\.14\.0.*(V1|v1)' <<<"${plan_out}" ||
  fail "--plan must mark 0.14.0 as the V1 gate chart, distinct from the bulk copies"

# Direction of travel AS RENDERED IN THE PLAN. This inspects `--plan` text; it does not
# reach the `cosign copy` call, so on its own it would NOT catch a transposed
# source/destination in the copy itself -- an earlier version of this comment claimed it
# did. The transposition mutation (which with --apply would push the chart path back over
# the OLM-pinned controller image repository) is caught behaviourally in section 8, by
# asserting the argv of the recorded `cosign copy`. Both are worth having: this one keeps
# the operator-facing rehearsal honest, that one keeps the write honest.
NEW_PATH='ghcr.io/platformrelay/charts/kollect'
OLD_PATH='ghcr.io/platformrelay/kollect'
for line in "${copy_lines[@]}"; do
  grep -Fq "${NEW_PATH}:" <<<"${line}" ||
    fail "copy plan line does not name the new chart path ${NEW_PATH}: '${line}'"
  # The arrow must point at the new path, never at the controller image repository. The
  # arrow itself is required: without it '${line##*-> }' would silently degrade to the
  # whole line, and the destination assertion would be re-testing what was just tested.
  grep -Fq -- '-> ' <<<"${line}" ||
    fail "copy plan line must render 'src -> dst' so the direction of the copy is legible: '${line}'"
  dest="${line##*-> }"
  grep -Fq "${NEW_PATH}:" <<<"${dest}" ||
    fail "copy destination must be ${NEW_PATH}, not '${dest}' -- ADR-0709 keeps the controller image at ${OLD_PATH} and its OLM-pinned digests must not be disturbed"
done
grep -Fq "${OLD_PATH}:" <<<"${plan_out}" ||
  fail "--plan must name the OLD path ${OLD_PATH} as the copy source"

pass "--plan copies 0.14.0 (V1 gate) + 0.15.0-0.19.0 into ${NEW_PATH} and never republishes 0.9.0-0.13.0"

# ---------------------------------------------------------------------------
# 3. Nothing in this script may take anything away.
# ---------------------------------------------------------------------------
# A migration script holding a write:packages token is one typo away from being a deletion
# script. GHCR package-version deletion is not reversible past its restore window, and the
# blast radius includes artifacts this migration must not touch at all -- most sharply
# package version 1087932920, which carries the v0.12.0 CONTROLLER IMAGE and its cosign
# signature, sitting in the same GHCR package as the charts being copied.
#
# The scan runs over EXECUTABLE lines. Comment-only lines are stripped (the script has to
# be able to say "this must never delete anything" in prose), and so is the body of the
# single quoted handoff heredoc, which is printed text rather than code. Both exemptions
# are narrow and both are proven non-vacuous below.
code_only="$(awk '
  /^[[:space:]]*#/ { next }                  # comment-only lines: prose, not code
  /<<[[:space:]]*.?HANDOFF.?/ { inhd = 1 }   # opener of the quoted handoff heredoc
  inhd == 1 { if ($0 ~ /^HANDOFF[[:space:]]*$/) { inhd = 0 }; next }
  { print }
' "${SCRIPT}")"

code_lines_n="$(printf '%s\n' "${code_only}" | grep -cvE '^[[:space:]]*$' || true)"
(( code_lines_n >= 100 )) ||
  fail "the comment/heredoc stripper left only ${code_lines_n} executable lines -- it is over-matching, and the destructive-verb scan below would be passing vacuously"

# Broad scan: any destructive word as a whole word on an executable line. Whole-word
# matching is deliberate -- a substring scan would fire on 'confirm', 'perform', 'term'.
if printf '%s\n' "${code_only}" | grep -nE '(^|[^[:alnum:]_.-])(delete|DELETE|Delete|rm)([^[:alnum:]_.-]|$)'; then
  fail "a destructive verb appears on an executable line of ${SCRIPT} (listed above) -- this script must never remove a tag, a package version, a signature, or a local file"
fi

# Narrow scan: the specific invocations that would actually take something out of a
# registry, spelled out so the failure message names the real hazard even if the broad
# scan above is ever relaxed.
if printf '%s\n' "${code_only}" | grep -nEi 'crane[[:space:]]+delete|oras[[:space:]]+(manifest|blob|tag|repo)[[:space:]]+delete|cosign[[:space:]]+clean|skopeo[[:space:]]+delete|-X[[:space:]]*(DELETE|PUT|PATCH|POST)|--request[[:space:]]+(DELETE|PUT|PATCH|POST)|helm[[:space:]]+registry[[:space:]]+logout'; then
  fail "${SCRIPT} invokes a destructive or state-changing registry/API verb (listed above); every GitHub API call this script makes must be a plain GET"
fi

# The protected package version must be named -- so a reader knows it exists and why it is
# off limits -- but only ever as an inert constant or in prose. If it ever appears on a
# line that also invokes a registry or API client, it has stopped being a guard.
grep -Fq '1087932920' "${SCRIPT}" ||
  fail "${SCRIPT} must name package version 1087932920 (v0.12.0 controller image + its cosign signature) as explicitly out of scope, so nobody later assumes the 0.12.0 coordinate is a stale chart to tidy up"
if printf '%s\n' "${code_only}" | grep -nE '1087932920' | grep -Ei 'crane|oras|cosign|curl|gh[[:space:]]+api|api\.github\.com'; then
  fail "package version 1087932920 is passed to a registry or API client (listed above) -- it carries the v0.12.0 controller image and must not be touched by any verb, read or write"
fi

pass "no destructive registry verb on any executable line; package version 1087932920 is an inert guard only"

# ---------------------------------------------------------------------------
# 4. Phase order, asserted at the CALL SITES.
# ---------------------------------------------------------------------------
# Ordering is checked on the dispatcher's invocation order, not on the order the functions
# happen to be defined in. Definition order is cosmetic in bash; call order is the thing
# that decides whether five charts get copied on the back of an unverified assumption.
call_line() {
  local fn="$1" n
  # A bare, indented call: `  phase1_v1_gate`. Excludes the `fn() {` definition line and
  # any mention inside a comment or string.
  mapfile -t n < <(grep -nE "^[[:space:]]+${fn}([[:space:]]*)$" "${SCRIPT}" | cut -d: -f1)
  (( ${#n[@]} == 1 )) ||
    fail "expected exactly one call site for ${fn}(); found ${#n[@]} -- ordering assertions are meaningless with zero or several"
  printf '%s' "${n[0]}"
}

p0="$(call_line phase0_preconditions)"
p1="$(call_line phase1_v1_gate)"
p2="$(call_line phase2_copy_remaining)"
p3="$(call_line phase3_visibility)"
p4="$(call_line phase4_metadata)"
ph="$(call_line print_handoff)"

(( p0 < p1 )) || fail "phase0_preconditions must be called before phase1_v1_gate (line ${p0} vs ${p1})"
# The V1 gate exists so that a signature-portability failure costs one chart, not six. If
# the bulk copy ran first, the gate would be reporting on a migration that already happened.
(( p1 < p2 )) || fail "phase1_v1_gate must be called before phase2_copy_remaining (line ${p1} vs ${p2}) -- the whole point of the gate is that it fails before five more charts are copied"
# Visibility before metadata: GHCR creates new packages PRIVATE. Pushing artifacthub.io
# metadata to a private package produces a completely silent dead end -- every command
# succeeds and Artifact Hub sees nothing. The check has to come first so the operator
# learns about it while they are still at the keyboard.
(( p3 < p4 )) || fail "phase3_visibility must be called before phase4_metadata (line ${p3} vs ${p4}) -- a metadata push onto a private package succeeds locally and is invisible to Artifact Hub"
(( p2 < p3 )) || fail "phase2_copy_remaining must be called before phase3_visibility (line ${p2} vs ${p3})"
(( p4 < ph )) || fail "print_handoff must come last (line ${p4} vs ${ph})"

# Order in the dispatcher is not enough on its own: a maintainer resuming a partial run
# might call a phase directly. The bulk copy must therefore also refuse to run at RUNTIME
# unless the V1 gate has actually passed in this process.
awk '/^phase2_copy_remaining\(\)/,/^}/' "${SCRIPT}" | grep -Fq 'V1_GATE_PASSED' ||
  fail "phase2_copy_remaining() must check the V1_GATE_PASSED guard at runtime -- dispatcher order alone does not survive someone invoking the phase directly while resuming a partial migration"

pass "phases are called in order (0 -> 1 -> 2 -> 3 -> 4 -> handoff) and the bulk copy is runtime-gated on the V1 result"

# ---------------------------------------------------------------------------
# 5. The cosign verify shape must be the one the repo publishes.
# ---------------------------------------------------------------------------
# A verification that uses a looser identity regexp than the published one would pass on
# signatures that a user following docs/RELEASE.md would reject -- i.e. the migration
# would certify chart signatures as good and the hub's readers would still see them fail.
# The expected value is cross-checked against docs/RELEASE.md so the two cannot drift
# apart silently in either direction.
IDENTITY_REGEXP='^https://github\.com/[Pp]latform[Rr]elay/[Kk]ollect/.+'
OIDC_ISSUER='https://token.actions.githubusercontent.com'

grep -Fq -- "--certificate-identity-regexp '${IDENTITY_REGEXP}'" "${SCRIPT}" ||
  grep -Fq -- "${IDENTITY_REGEXP}" "${SCRIPT}" ||
  fail "${SCRIPT} must verify with the published identity regexp ${IDENTITY_REGEXP}"
grep -Fq -- "${OIDC_ISSUER}" "${SCRIPT}" ||
  fail "${SCRIPT} must pin --certificate-oidc-issuer ${OIDC_ISSUER}"

if [[ -f "${RELEASE_DOC}" ]]; then
  doc_re="$(grep -oE -- "--certificate-identity-regexp '[^']+'" "${RELEASE_DOC}" | head -1 |
    sed -E "s/^--certificate-identity-regexp '//; s/'$//")"
  # Non-vacuity: if the extraction returned nothing, the comparison below would compare
  # the empty string against itself-ish and pass. Demand a real value first.
  [[ -n "${doc_re}" ]] ||
    fail "could not extract a --certificate-identity-regexp value from ${RELEASE_DOC}; this cross-check would pass vacuously"
  [[ "${doc_re}" == "${IDENTITY_REGEXP}" ]] ||
    fail "the identity regexp published in ${RELEASE_DOC} ('${doc_re}') differs from the one this gate pins ('${IDENTITY_REGEXP}'); the migration script and the published verify command must agree"
  grep -Fq -- "${doc_re}" "${SCRIPT}" ||
    fail "${SCRIPT} does not use the identity regexp that ${RELEASE_DOC} publishes ('${doc_re}')"
fi

# Gate 3 of the V1 gate. Digest equality plus a passing `cosign verify` is NOT sufficient:
# a signature minted at migration time from the maintainer's own laptop would also verify
# under a loose enough reading, while presenting an identity no published verify command
# accepts. The gate must therefore inspect the certificate identity that verification
# returns and require the ORIGINAL release-time workflow identity.
grep -Fq 'release.yaml@refs/tags/v0.14.0' "${SCRIPT}" ||
  fail "the V1 gate must assert the certificate identity is the ORIGINAL release-time identity (.../release.yaml@refs/tags/v0.14.0), not merely that some signature verifies"

pass "cosign verify uses the published issuer/identity shape and pins the original release-time identity"

# ---------------------------------------------------------------------------
# 6. Handoff warnings.
# ---------------------------------------------------------------------------
# The two failure modes below are both irreversible-in-practice and both are things a
# maintainer would plausibly do on their own initiative between running this script and
# finishing the migration. They must be printed, not left in a doc.
handoff_out="$(env -u GHCR_TOKEN -u GITHUB_TOKEN -u CR_PAT bash "${SCRIPT}" --handoff 2>&1)" ||
  fail "'--handoff' must exit 0 without credentials so the operator can re-read the closing instructions at any time"

# Artifact Hub's Manager.Update keys on repository NAME: an in-place URL edit preserves
# repository_id, stars and Verified Publisher; a delete-and-recreate loses all three, and
# Verified Publisher in particular is not trivially re-earned.
grep -Eiq 'in.place|do not (delete|re-?create)|never (delete|re-?create)' <<<"${handoff_out}" ||
  fail "--handoff must warn against deleting and re-creating the Artifact Hub repository (an in-place URL edit preserves repository_id, stars and Verified Publisher; a re-create loses all three)"
grep -Eiq 'verified publisher' <<<"${handoff_out}" ||
  fail "--handoff must name Verified Publisher as one of the things a re-create destroys"
# Between the workflow lane landing and the URL repoint, a release publishes the chart to
# the new path only, while Artifact Hub still tracks the old one -- so the release is
# invisible on the hub and adds one more permanent tracking error to the list.
grep -Eiq 'release' <<<"${handoff_out}" ||
  fail "--handoff must warn against cutting a release mid-migration"
grep -Eiq 'mid.migration|during the migration|until the (url|repoint)|before the repoint' <<<"${handoff_out}" ||
  fail "--handoff must state that the no-release window runs until the Artifact Hub URL repoint is done"

pass "--handoff prints the in-place-edit warning and the no-release-mid-migration warning"

# ---------------------------------------------------------------------------
# 7. --verify-ac1: the sampling rule is the mode's entire reason to exist.
# ---------------------------------------------------------------------------
# Artifact Hub's public repository search endpoint exposes last_tracking_errors and
# last_tracking_ts with no API key, which makes the acceptance criterion scriptable. But
# last_tracking_errors is a SAMPLE, not a census: observed 2026-09-01, the registry held
# eleven v-prefixed image tags (v0.9.0-v0.19.0), every one of them a load-as-chart
# candidate, and the endpoint reported only six errors. So an empty error list from ONE
# read is not evidence of anything -- and reading the same tracking run twice is the
# easiest way to manufacture a green result by accident.
grep -Fq -- '--verify-ac1' "${SCRIPT}" ||
  fail "${SCRIPT} must provide a --verify-ac1 mode"
grep -Fq -- '--since' "${SCRIPT}" ||
  fail "--verify-ac1 must take a --since <unix-ts> baseline argument (the baseline lives in gitignored agent-context/, so it cannot be read from the repo)"
grep -Fq 'INCONCLUSIVE' "${SCRIPT}" ||
  fail "--verify-ac1 must be able to report INCONCLUSIVE; a mode that can only say PASS or FAIL can express neither 'the tracking run did not advance' nor 'only one run postdates the repoint'"
grep -Fq 'last_tracking_ts' "${SCRIPT}" ||
  fail "--verify-ac1 must read last_tracking_ts"
grep -Fq 'last_tracking_errors' "${SCRIPT}" ||
  fail "--verify-ac1 must read last_tracking_errors"
grep -Fq 'verified_publisher' "${SCRIPT}" ||
  fail "--verify-ac1 must assert verified_publisher is still true -- the URL repoint can silently destroy it"

# Behavioural. The script exposes a documented fixture seam so this property can be tested
# offline; without it the only available assertion would be a grep, which cannot tell the
# difference between a script that enforces the rule and one that mentions it.
fixture_dir="$(mktemp -d)"
stub_root="$(mktemp -d)"
trap 'rm -rf "${fixture_dir}" "${stub_root}"' EXIT
write_fixture() {
  # $1 = path, $2 = last_tracking_ts, $3 = last_tracking_errors (JSON string body), $4 = url, $5 = verified_publisher
  cat >"$1" <<JSON
[{"repository_id":"cb3be9a6-8e3b-4419-9de5-1184fe349c29","name":"kollect","url":"$4","kind":0,"verified_publisher":$5,"last_tracking_ts":$2,"last_tracking_errors":$3}]
JSON
}

NEW_URL="oci://${NEW_PATH}"
write_fixture "${fixture_dir}/clean_a.json" 1788190801 'null' "${NEW_URL}" true
write_fixture "${fixture_dir}/clean_b.json" 1788194401 'null' "${NEW_URL}" true
write_fixture "${fixture_dir}/clean_c.json" 1788198001 'null' "${NEW_URL}" true
write_fixture "${fixture_dir}/errors_b.json" 1788194401 '"error preparing package: error loading chart (oci://ghcr.io/platformrelay/charts/kollect:v0.19.0): layer not found"' "${NEW_URL}" true
write_fixture "${fixture_dir}/oldurl_b.json" 1788194401 'null' "oci://${OLD_PATH}" true
write_fixture "${fixture_dir}/unverified_b.json" 1788194401 'null' "${NEW_URL}" false

# A repository-search response with a SIBLING row FIRST and the real one second. This is
# the shape that tells the three candidate selectors apart; a single-row fixture cannot,
# because there `repository_id`, exact-name and `.[0]` all return the same object.
#
# The sibling is named `kollect-staging` on purpose: it CONTAINS "kollect" as a substring,
# which is exactly the confusion ADR-0709 warns about when it names `.[0]` as the latent
# wrong-repository bug. Exact-name equality cannot match it; `.[0]` picks it every time.
# It also carries a different repository_id, a stale timestamp, errors, the OLD url and
# verified_publisher=false -- so a run that reads it produces a visibly wrong verdict
# rather than a subtly wrong one.
write_sibling_fixture() {
  # $1 path, $2 real row's last_tracking_ts, $3 real row's errors (JSON), $4 url, $5 vp
  cat >"$1" <<JSON
[{"repository_id":"11111111-2222-3333-4444-555555555555","name":"kollect-staging","url":"oci://ghcr.io/platformrelay/kollect","kind":0,"verified_publisher":false,"last_tracking_ts":1700000000,"last_tracking_errors":"error preparing package: sibling repository, not ours"},
 {"repository_id":"cb3be9a6-8e3b-4419-9de5-1184fe349c29","name":"kollect","url":"$4","kind":0,"verified_publisher":$5,"last_tracking_ts":$2,"last_tracking_errors":$3}]
JSON
}

# Same two rows, but the real one's repository_id has changed -- what a delete-and-
# re-create looks like. The exact-name fallback must still find it, and must still not be
# fooled by the substring sibling sitting in front of it.
write_sibling_fixture_no_id() {
  cat >"$1" <<JSON
[{"repository_id":"11111111-2222-3333-4444-555555555555","name":"kollect-staging","url":"oci://ghcr.io/platformrelay/kollect","kind":0,"verified_publisher":false,"last_tracking_ts":1700000000,"last_tracking_errors":"error preparing package: sibling repository, not ours"},
 {"repository_id":"99999999-8888-7777-6666-555555555555","name":"kollect","url":"$4","kind":0,"verified_publisher":$5,"last_tracking_ts":$2,"last_tracking_errors":$3}]
JSON
}

verify_ac1() {
  # $1 = colon-separated fixture list; remaining args passed through.
  local fixtures="$1"
  shift
  # --timeout is generous on purpose. Fixture reads never sleep and the loop is bounded by
  # the fixture count, so a large timeout cannot slow this down -- but a tight one CAN end
  # the loop early on a slow runner, turning a three-fixture PASS case into a flaky
  # INCONCLUSIVE. Wall clock must not decide the verdict in a fixture-driven test.
  MIGRATE_AH_FIXTURES="${fixtures}" bash "${SCRIPT}" --verify-ac1 --interval 0 --timeout 600 "$@" 2>&1
}

# (a) The control: two reads whose timestamp genuinely advanced, no errors, new URL,
#     verified publisher still true. This MUST be able to reach PASS -- otherwise every
#     negative case below is satisfied by a mode that can never say PASS at all, which is
#     the classic way a safety assertion becomes vacuous.
out_pass="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/clean_b.json" --since 1788190800 || true)"
grep -Eq '^(RESULT: )?PASS' <<<"${out_pass}" ||
  fail "--verify-ac1 must report PASS for TWO reads that both postdate the baseline, both clean, both on the new URL with verified_publisher true; got:"$'\n'"${out_pass}"

# (b) The rule itself: same timestamp in both reads means the same tracking run was
#     sampled twice. That is not two samples, and it must never be PASS.
out_same="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/clean_a.json" --since 1788190800 || true)"
grep -Fq 'INCONCLUSIVE' <<<"${out_same}" ||
  fail "--verify-ac1 must report INCONCLUSIVE when last_tracking_ts did NOT advance between reads; got:"$'\n'"${out_same}"
if grep -Eq '^(RESULT: )?PASS' <<<"${out_same}"; then
  fail "--verify-ac1 reported PASS for two samples of the SAME tracking run -- last_tracking_errors is a sample, not a census, and one run proves nothing"
fi

# (c) A genuinely advanced run that still carries errors is a FAIL, with a non-zero exit.
set +e
out_err="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/errors_b.json" --since 1788190800)"
rc_err=$?
set -e
grep -Fq 'FAIL' <<<"${out_err}" ||
  fail "--verify-ac1 must report FAIL when an advanced tracking run still reports errors; got:"$'\n'"${out_err}"
(( rc_err != 0 )) ||
  fail "--verify-ac1 must exit non-zero on FAIL (exit was ${rc_err}) -- a FAIL that exits 0 cannot gate anything"

# (d) The two things the control-panel repoint can silently destroy.
out_url="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/oldurl_b.json" --since 1788190800 || true)"
grep -Fq 'FAIL' <<<"${out_url}" ||
  fail "--verify-ac1 must FAIL while Artifact Hub still tracks the OLD url; got:"$'\n'"${out_url}"
out_vp="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/unverified_b.json" --since 1788190800 || true)"
grep -Fq 'FAIL' <<<"${out_vp}" ||
  fail "--verify-ac1 must FAIL when verified_publisher is no longer true; got:"$'\n'"${out_vp}"

# (e) Without a baseline there is nothing tying an observed tracking run to the repoint,
#     so a PASS would be unfounded. The mode must refuse to claim one and must print the
#     timestamp to use as the baseline for a later run.
out_nosince="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/clean_b.json" || true)"
if grep -Eq '^(RESULT: )?PASS' <<<"${out_nosince}"; then
  fail "--verify-ac1 must not report PASS without a --since baseline; an error list from a run that predates the repoint says nothing about the repoint"
fi
grep -Fq '1788194401' <<<"${out_nosince}" ||
  fail "--verify-ac1 must print the observed last_tracking_ts to use as the --since baseline for a later run"

# (f) A run that predates the baseline is INCONCLUSIVE, not PASS: the errors it lists were
#     collected before the repoint could possibly have taken effect.
out_stale="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/clean_b.json" --since 1799999999 || true)"
if grep -Eq '^(RESULT: )?PASS' <<<"${out_stale}"; then
  fail "--verify-ac1 must not report PASS when the observed tracking run is older than the --since baseline"
fi

pass "--verify-ac1 enforces two genuinely distinct tracking runs and cannot report PASS from a single sample"

# (g) ADR-0709 states AC1 as TWO reads, BOTH taken after the repoint, BOTH empty. All three
#     of those words are load-bearing and each has its own way of being got wrong.
write_fixture "${fixture_dir}/dirty_a.json" 1788190801 '"error preparing package: pre-repoint (package: kollect version: v0.9.0)"' "${NEW_URL}" true

# BOTH EMPTY: a baseline older than both reads makes read 1 post-repoint evidence, so its
# errors are a FAIL rather than something to be quietly discarded.
out_both="$(verify_ac1 "${fixture_dir}/dirty_a.json:${fixture_dir}/clean_b.json" --since 1788190000 || true)"
grep -Fq 'FAIL' <<<"${out_both}" ||
  fail "--verify-ac1 must FAIL when read 1 ALSO postdates the baseline and reported errors -- AC1 requires both post-repoint runs clean; got:"$'\n'"${out_both}"

# BOTH AFTER THE REPOINT. This is the case the first implementation got wrong, and it is
# the COMMON case: the operator pastes back the timestamp this mode printed, so read 1 is
# the baseline run itself -- pre-repoint, and expected to carry errors. Not failing on it
# was right; calling the result PASS was not. One clean post-repoint run is exactly the
# "single empty read proves nothing" situation this mode exists to refuse. The verdict is
# INCONCLUSIVE, and the loop keeps polling for the second post-repoint run.
out_pre="$(verify_ac1 "${fixture_dir}/dirty_a.json:${fixture_dir}/clean_b.json" --since 1788190801 || true)"
if grep -Eq '^(RESULT: )?PASS' <<<"${out_pre}"; then
  fail "--verify-ac1 reported PASS from ONE post-repoint run (read 1 was the baseline run itself). ADR-0709 requires two reads both taken after the repoint; got:"$'\n'"${out_pre}"
fi
grep -Fq 'INCONCLUSIVE' <<<"${out_pre}" ||
  fail "--verify-ac1 must report INCONCLUSIVE when only one observed run postdates the baseline; got:"$'\n'"${out_pre}"
# A pre-repoint read carrying errors must not be counted AGAINST the repoint either -- that
# would red the tool when it is used exactly as the handoff instructs.
grep -Fq 'FAIL' <<<"${out_pre}" &&
  fail "--verify-ac1 must not FAIL on a read that predates the baseline: those errors are expected pre-repoint output, not evidence about the repoint; got:"$'\n'"${out_pre}"

# ...and the polling actually gets there. Same dirty pre-repoint read 1, then TWO clean
# post-repoint runs -> PASS. Without this, every assertion above is satisfied by a mode
# that can no longer reach PASS at all, which is the way a strictness fix becomes a bug.
out_two_post="$(verify_ac1 "${fixture_dir}/dirty_a.json:${fixture_dir}/clean_b.json:${fixture_dir}/clean_c.json" --since 1788190801 || true)"
grep -Eq '^(RESULT: )?PASS' <<<"${out_two_post}" ||
  fail "--verify-ac1 must reach PASS once TWO runs postdating the baseline have both been observed clean; got:"$'\n'"${out_two_post}"

pass "--verify-ac1 requires two runs both taken after the repoint and both empty, per ADR-0709"

# (h) WHICH repository the mode reads. ADR-0709 names `.[0]` as the latent wrong-repository
#     bug -- the search endpoint returns a list, and the first element is whatever the
#     search ranked first. Every fixture above is a single row carrying both the right id
#     and the right name, so `repository_id`, exact-name and `.[0]` are indistinguishable
#     to them: the selector had no discriminating coverage at all. These two rows do have
#     it, and the sibling is the substring case the ADR actually warns about.
write_sibling_fixture "${fixture_dir}/sib_a.json" 1788190801 'null' "${NEW_URL}" true
write_sibling_fixture "${fixture_dir}/sib_b.json" 1788194401 'null' "${NEW_URL}" true
out_sibling="$(verify_ac1 "${fixture_dir}/sib_a.json:${fixture_dir}/sib_b.json" --since 1788190800 || true)"
grep -Eq '^(RESULT: )?PASS' <<<"${out_sibling}" ||
  fail "--verify-ac1 must select the kollect repository by repository_id even when a sibling row (kollect-staging) is returned FIRST by the search. Selecting .[0] reads the wrong repository -- the failure mode ADR-0709 names explicitly; got:"$'\n'"${out_sibling}"
# The sibling's own values must not appear anywhere in the verdict: it is stale, dirty, on
# the old url and not a verified publisher, so if any of it leaked in, the result is being
# computed from the wrong row.
if grep -Fq 'kollect-staging' <<<"${out_sibling}"; then
  fail "the sibling repository's data reached the output; the selector is reading the wrong row:"$'\n'"${out_sibling}"
fi
if grep -Fq '1700000000' <<<"${out_sibling}"; then
  fail "the sibling repository's last_tracking_ts (1700000000) reached the output; .[0] is being read instead of the row with repository_id cb3be9a6-...:"$'\n'"${out_sibling}"
fi

# The exact-name fallback: still finds the right row when the id has changed, still is not
# fooled by the substring sibling in front of it, and says out loud that the id missed --
# an id that changed is what a delete-and-re-create looks like, which the handoff warns
# against and which normally also clears verified_publisher.
write_sibling_fixture_no_id "${fixture_dir}/noid_a.json" 1788190801 'null' "${NEW_URL}" true
write_sibling_fixture_no_id "${fixture_dir}/noid_b.json" 1788194401 'null' "${NEW_URL}" true
out_noid="$(verify_ac1 "${fixture_dir}/noid_a.json:${fixture_dir}/noid_b.json" --since 1788190800 || true)"
grep -Eq '^(RESULT: )?PASS' <<<"${out_noid}" ||
  fail "the exact-name fallback must still find the kollect row when repository_id has changed; got:"$'\n'"${out_noid}"
if grep -Fq '1700000000' <<<"${out_noid}"; then
  fail "the exact-name fallback matched the substring sibling kollect-staging; equality, not substring, is the point:"$'\n'"${out_noid}"
fi
grep -Fiq 'repository_id' <<<"${out_noid}" ||
  fail "when the fallback fires the operator must be told the repository_id no longer matches -- that is what a delete-and-re-create looks like; got:"$'\n'"${out_noid}"

pass "--verify-ac1 selects by repository_id, falls back to exact name, and is not fooled by a sibling row ranked first"

# ---------------------------------------------------------------------------
# 8. Behavioural: the --apply path, driven through recording stubs.
# ---------------------------------------------------------------------------
# Sections 1-7 are static or exercise only the credential-free modes. That leaves the most
# consequential code in the file -- everything gated on APPLY -- asserted by nothing, and a
# mutation review proved it: flipping the APPLY default, making run_mutating execute
# unconditionally, transposing the arguments of `cosign copy`, and removing the V1 gate's
# guard, the anti-clobber refusal, the identity check and `cosign verify` outright ALL
# survived a green run of this file.
#
# None of that needs a registry. It needs cosign/crane/oras/curl to be observable, which is
# what this section does: a controlled PATH in front of the script, holding stubs that
# record their argv and answer from a scripted registry state. Every assertion below is
# about what the script CALLED and in what order -- the same evidence a live run would
# produce, minus the live registry.
stub_bin="${stub_root}/bin"
mkdir -p "${stub_bin}"

# crane: `digest` answers from a two-column state file. Absent tags produce GHCR's real
# MANIFEST_UNKNOWN wording, and STUB_CRANE_FLAKE produces a transient error instead --
# the distinction the script must not collapse.
cat >"${stub_bin}/crane" <<'STUB'
#!/usr/bin/env bash
printf 'crane %s\n' "$*" >>"${STUB_LOG}"
if [ "${1:-}" = "auth" ] && [ "${2:-}" = "login" ]; then cat >>"${STUB_LOGIN_STDIN}"; exit 0; fi
if [ "${1:-}" = "digest" ]; then
  if [ -n "${STUB_CRANE_FLAKE:-}" ] && [ "$2" = "${STUB_CRANE_FLAKE}" ]; then
    printf 'Error: fetching manifest %s: GET https://ghcr.io/v2/token: TOOMANYREQUESTS: retry later\n' "$2" >&2
    exit 1
  fi
  d="$(awk -v r="$2" '$1==r{print $2; exit}' "${STUB_DIGESTS}" 2>/dev/null)"
  if [ -n "${d}" ]; then printf '%s\n' "${d}"; exit 0; fi
  printf 'Error: fetching manifest %s: GET https://ghcr.io/v2/: MANIFEST_UNKNOWN: manifest unknown\n' "$2" >&2
  exit 1
fi
exit 0
STUB

# cosign: `copy` lands the subject and its .sig at the destination (so a resume sees the
# state a real partial run would leave); `verify` answers with a certificate Subject built
# from the tag it was asked about, so each version gets its own correct identity unless the
# scenario deliberately supplies a wrong template.
cat >"${stub_bin}/cosign" <<'STUB'
#!/usr/bin/env bash
printf 'cosign %s\n' "$*" >>"${STUB_LOG}"
case "${1:-}" in
  copy)
    src="$3"; dst="$4"
    if [ "${STUB_COPY_CREATES:-1}" = "1" ]; then
      d="$(awk -v r="${src}" '$1==r{print $2; exit}' "${STUB_DIGESTS}")"
      printf '%s %s\n' "${dst}" "${d}" >>"${STUB_DIGESTS}"
      printf '%s:sha256-%s.sig sha256:5165000000000000000000000000000000000000000000000000000000000000\n' \
        "${dst%:*}" "${d#sha256:}" >>"${STUB_DIGESTS}"
    fi
    ;;
  login)
    cat >>"${STUB_LOGIN_STDIN}"
    ;;
  verify)
    if [ "${STUB_VERIFY_FAIL:-0}" = "1" ]; then
      printf 'Error: no matching signatures\n' >&2
      exit 1
    fi
    ref=""
    for a in "$@"; do case "$a" in *@sha256:*) ref="$a" ;; esac; done
    tag="${ref%@*}"; tag="${tag##*:}"
    subj="${STUB_SUBJECT_TEMPLATE}"
    printf '[{"optional":{"Subject":"%s"}}]\n' "${subj//__TAG__/${tag}}"
    ;;
esac
exit 0
STUB

cat >"${stub_bin}/oras" <<'STUB'
#!/usr/bin/env bash
printf 'oras %s\n' "$*" >>"${STUB_LOG}"
if [ "${1:-}" = "login" ]; then cat >>"${STUB_LOGIN_STDIN}"; fi
exit 0
STUB

# curl: records argv to the same log and, separately, whatever arrives on stdin. The split
# is the point -- it is what lets the credential-exposure assertion below distinguish
# "the token was passed safely" from "the token was passed at all".
cat >"${stub_bin}/curl" <<'STUB'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >>"${STUB_LOG}"
case " $* " in *" -K "*) cat >>"${STUB_CURL_STDIN}" ;; esac
for a in "$@"; do
  case "$a" in
    https://api.github.com/*)
      printf '{"visibility":"%s"}\n%s\n' "${STUB_PKG_VISIBILITY:-public}" "${STUB_PKG_HTTP:-200}"
      exit 0
      ;;
    https://ghcr.io/token*)
      printf '{"token":"%s"}\n' "${STUB_GHCR_JWT}"
      exit 0
      ;;
    https://artifacthub.io/*)
      # Counted, so a test can measure how hard the poll loop hits a third-party API.
      printf 'x\n' >>"${STUB_AH_COUNT:-/dev/null}"
      code="${STUB_AH_HTTP:-200}"
      # STUB_AH_OK_FIRST=N: answer the first N requests normally and fail after that, so a
      # scenario can put the failure on a read OTHER than the first.
      if [ -n "${STUB_AH_OK_FIRST:-}" ] && [ -f "${STUB_AH_COUNT:-/dev/null}" ]; then
        if [ "$(wc -l <"${STUB_AH_COUNT}")" -le "${STUB_AH_OK_FIRST}" ]; then code=200; fi
      fi
      if [ "${code}" != "200" ]; then
        printf 'curl: (22) The requested URL returned error: %s\n' "${code}" >&2
        exit 22
      fi
      cat "${STUB_AH_JSON}"
      exit 0
      ;;
  esac
done
exit 0
STUB
chmod +x "${stub_bin}/crane" "${stub_bin}/cosign" "${stub_bin}/oras" "${stub_bin}/curl"

b64url() { base64 | tr -d '\n' | tr '+/' '-_' | tr -d '='; }
PUSH_JWT="hdr.$(printf '%s' '{"access":[{"type":"repository","name":"platformrelay/charts/kollect","actions":["pull","push"]}]}' | b64url).sig"
PULL_JWT="hdr.$(printf '%s' '{"access":[{"type":"repository","name":"platformrelay/charts/kollect","actions":["pull"]}]}' | b64url).sig"
GOOD_SUBJECT_TEMPLATE='https://github.com/platformrelay/kollect/.github/workflows/release.yaml@refs/tags/v__TAG__'
LAPTOP_SUBJECT_TEMPLATE='https://github.com/platformrelay/kollect/laptop@refs/heads/main'
# Accepted by the script's token-format check and distinctive enough that finding it in a
# log is unambiguous -- but deliberately NOT shaped like a real PAT. gitleaks runs with
# useDefault = true and its `github-pat` rule is ghp_[0-9a-zA-Z]{36}; the underscores here
# break that run, the same trick hack/test/lab_report_redaction_meta_test.sh uses. A test
# fixture must not be the thing that reds the secret scanner.
TEST_TOKEN='ghp_FAKESECRET_dist_ah_03_stub_token'

SRC='ghcr.io/platformrelay/kollect'
DST='ghcr.io/platformrelay/charts/kollect'
# Deterministic, well-formed 64-hex digests keyed by a label. (A `%064d` of the version
# digits looked simpler and was wrong: printf reads a leading zero as octal, so 0.18.0 and
# 0.19.0 produced "invalid octal number" and an empty digest.)
dig() { printf 'sha256:%s' "$(printf 'kollect-%s' "$1" | sha256sum | cut -d' ' -f1)"; }

# Scenario state. Each scene is an isolated recording directory.
new_scene() {
  local d="${stub_root}/scene-$1"
  rm -rf "${d}"
  mkdir -p "${d}"
  : >"${d}/log"
  : >"${d}/digests"
  : >"${d}/curl_stdin"
  : >"${d}/login_stdin"
  printf '%s' "${d}"
}
seed() { printf '%s %s\n' "$2" "$3" >>"$1/digests"; }
# Every chart this migration copies, present at the source.
seed_all_sources() {
  local d="$1" v
  for v in 0.14.0 0.15.0 0.16.0 0.17.0 0.18.0 0.19.0; do
    seed "${d}" "${SRC}:${v}" "$(dig "${v//./}")"
  done
}

STUB_VERIFY_FAIL=0
STUB_COPY_CREATES=1
STUB_SUBJECT_TEMPLATE="${GOOD_SUBJECT_TEMPLATE}"
STUB_GHCR_JWT="${PUSH_JWT}"
STUB_PKG_VISIBILITY=public
STUB_PKG_HTTP=200
STUB_CRANE_FLAKE=""
TOKEN_OVERRIDE=""
STUB_AH_OK_FIRST=""

# Runs the script under the stub PATH. Returns the script's exit code in `rc`, output in
# `out`, so both can be asserted on -- a check that only looks at output would accept a
# script that printed the right refusal and then carried on anyway.
rc=0
out=""
run_migrate() {
  local d="$1"
  shift
  set +e
  out="$(env "PATH=${stub_bin}:${PATH}" \
    "STUB_LOG=${d}/log" "STUB_DIGESTS=${d}/digests" "STUB_CURL_STDIN=${d}/curl_stdin" \
    "STUB_LOGIN_STDIN=${d}/login_stdin" \
    "STUB_VERIFY_FAIL=${STUB_VERIFY_FAIL}" "STUB_COPY_CREATES=${STUB_COPY_CREATES}" \
    "STUB_SUBJECT_TEMPLATE=${STUB_SUBJECT_TEMPLATE}" "STUB_GHCR_JWT=${STUB_GHCR_JWT}" \
    "STUB_PKG_VISIBILITY=${STUB_PKG_VISIBILITY}" "STUB_PKG_HTTP=${STUB_PKG_HTTP}" \
    "STUB_CRANE_FLAKE=${STUB_CRANE_FLAKE}" \
    "GHCR_TOKEN=${TOKEN_OVERRIDE:-${TEST_TOKEN}}" "GHCR_USER=stubuser" \
    bash "$@" 2>&1)"
  rc=$?
  set -e
}
logged() { grep -Eq "$2" "$1/log"; }
# Fixed-string twin. Use this whenever the needle is a VALUE the gate already pins
# (an identity regexp, an issuer URL): the value is full of ERE metacharacters, so a
# hand-escaped copy of it is a second source of truth that drifts silently. SEC-VERIFYCASE-01
# is exactly that failure -- a hardcoded copy here went on asserting the lowercase identity
# that cosign could never match, and agreed with itself while the real command was broken.
logged_f() { grep -Fq "$2" "$1/log"; }

# --- 8a. A bare invocation must not write. ---------------------------------
# Kills three mutations at once: APPLY defaulting to 1; run_mutating executing regardless
# of APPLY; registry_login executing regardless of APPLY. All three turn the documented
# rehearsal into a live migration while --help goes on saying "dry run is the default".
scene="$(new_scene dryrun)"
seed_all_sources "${scene}"
run_migrate "${scene}" "${SCRIPT}"
(( rc == 0 )) || fail "a bare (dry-run) invocation must exit 0 under stubs; rc=${rc}"$'\n'"${out}"

# Non-vacuity first: prove the harness actually drove the script. Without this, a scenario
# where the script died in Phase 0 would satisfy every "no mutating command" assertion.
logged "${scene}" '^crane digest ghcr\.io/platformrelay/kollect:0\.14\.0$' ||
  fail "the stub harness did not run: no 'crane digest' for the V1 chart was recorded. Every assertion in 8a would pass vacuously."$'\n'"${out}"

while read -r verb pattern; do
  [[ -n "${verb}" ]] || continue
  if logged "${scene}" "${pattern}"; then
    fail "a bare invocation (no --apply) executed a MUTATING command (${verb}). Dry run is the documented default and must write nothing:"$'\n'"$(grep -E "${pattern}" "${scene}/log")"
  fi
done <<'MUTATORS'
cosign-copy ^cosign copy
oras-push ^oras push
cosign-login ^cosign login
crane-login ^crane auth login
oras-login ^oras login
MUTATORS

pass "8a: a bare invocation executes zero mutating registry commands (APPLY defaults to rehearsal)"

# --- 8b. --apply copies, in the right direction, and completes. ------------
scene="$(new_scene apply)"
seed_all_sources "${scene}"
run_migrate "${scene}" "${SCRIPT}" --apply
(( rc == 0 )) ||
  fail "--apply must succeed under stubs with every source tag present; rc=${rc}"$'\n'"${out}"$'\n'"recorded copies:"$'\n'"$(grep -E '^cosign copy' "${scene}/log" || echo '(none)')"

# Verification is not optional on the write path. Asserted per chart on the recorded argv,
# so "cosign verify was deleted" fails HERE, naming the chart, rather than surfacing later
# as some unrelated symptom of an unset variable.
for v in 0.14.0 0.15.0 0.16.0 0.17.0 0.18.0 0.19.0; do
  logged "${scene}" "^cosign verify .*${DST//./\\.}:${v//./\\.}@sha256:" ||
    fail "--apply copied ${v} without running 'cosign verify' against ${DST}:${v}. Recorded verifies:"$'\n'"$(grep -E '^cosign verify' "${scene}/log" || echo '(none)')"
done
# ...and with the published flag shape, on the real call rather than in the source text.
logged_f "${scene}" "cosign verify --certificate-oidc-issuer ${OIDC_ISSUER} --certificate-identity-regexp ${IDENTITY_REGEXP} " ||
  fail "the recorded 'cosign verify' does not carry the published issuer/identity flags. Recorded:"$'\n'"$(grep -E '^cosign verify' "${scene}/log" | head -1)"

# THE transposition assertion. `cosign copy DST SRC` would push the chart-only path back
# over ghcr.io/platformrelay/kollect -- the repository whose digests are pinned immutably
# in already-merged OLM bundles. Asserted on exact argv, not on plan text.
logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.14\.0 ${DST//./\\.}:0\.14\.0$" ||
  fail "the V1 copy must be 'cosign copy --force ${SRC}:0.14.0 ${DST}:0.14.0' -- source first, destination second. Recorded copies:"$'\n'"$(grep -E '^cosign copy' "${scene}/log" || echo '(none)')"
if grep -E '^cosign copy' "${scene}/log" | grep -Eq " ${DST//./\\.}:[0-9.]+ ${SRC//./\\.}:"; then
  fail "a 'cosign copy' has the destination and source TRANSPOSED -- that pushes the chart path back over the OLM-pinned controller image repository:"$'\n'"$(grep -E '^cosign copy' "${scene}/log")"
fi

# Every version copied, none of the excluded ones, and the metadata push reached.
for v in 0.14.0 0.15.0 0.16.0 0.17.0 0.18.0 0.19.0; do
  logged "${scene}" "^cosign copy --force ${SRC//./\\.}:${v//./\\.} " ||
    fail "--apply did not copy ${v}"
done
for v in 0.9.0 0.10.0 0.11.0 0.12.0 0.13.0; do
  if logged "${scene}" "^cosign copy [^\n]*[:@]${v//./\\.}( |$)"; then
    fail "--apply copied excluded version ${v}"
  fi
done
logged "${scene}" "^oras push ${DST//./\\.}:artifacthub\.io" ||
  fail "--apply must push the Artifact Hub metadata to ${DST}:artifacthub.io"
# Non-vacuity for 8a: prove these commands DO get executed when --apply is given, so 8a's
# absence assertions are testing the gate rather than a command that is never reachable.
for pattern in '^cosign login' '^crane auth login' '^oras login'; do
  logged "${scene}" "${pattern}" ||
    fail "--apply must execute '${pattern#^}'; if it never runs, 8a's 'no login in dry run' assertion is vacuous"
done
grep -Fq 'V1 gate PASSED' <<<"${out}" ||
  fail "--apply with everything healthy must report the V1 gate as passed"

# The credential path under --apply. 8i covers dry run, where the logins never execute --
# so without this, swapping `--password-stdin` for `--password "$token"` was invisible to
# the whole gate. Positive and negative, so neither half can pass for the wrong reason.
[[ -s "${scene}/login_stdin" ]] ||
  fail "no registry login supplied its credential on stdin under --apply; the argv assertion below would pass vacuously"
grep -Fq "${TEST_TOKEN}" "${scene}/login_stdin" ||
  fail "the token did not reach the registry logins on stdin, so --apply is not authenticating"
if grep -Fq "${TEST_TOKEN}" "${scene}/log"; then
  fail "under --apply the registry token appears in a command's ARGV, readable from /proc/<pid>/cmdline. Logins must use --password-stdin:"$'\n'"$(grep -Fn "${TEST_TOKEN}" "${scene}/log")"
fi
if grep -Fq "${TEST_TOKEN}" <<<"${out}"; then
  fail "under --apply the registry token was printed to stdout/stderr"
fi

pass "8b: --apply copies source->destination for all six charts, pushes the metadata, and keeps the token out of argv"

# --- 8c. The V1 gate cannot be laundered by re-running. --------------------
# The regression test for the review finding. `cosign copy` runs BEFORE verification, so a
# run that copied and then failed to verify leaves chart + signature sitting at the
# destination. If the resume path treats those bytes as proof, run 2 prints "V1 gate
# PASSED ... certificate identity is the original release-time one" having computed
# neither, and proceeds to the bulk copy. Re-running is the documented remediation for
# Phase 3, so this is the path an operator actually takes.
scene="$(new_scene launder)"
seed_all_sources "${scene}"
# Exactly the state a failed run 1 leaves behind: subject AND signature at the destination.
seed "${scene}" "${DST}:0.14.0" "$(dig 0140)"
seed "${scene}" "${DST}:sha256-$(dig 0140 | sed 's/^sha256://').sig" "$(dig 5165)"
STUB_VERIFY_FAIL=1
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_VERIFY_FAIL=0
(( rc != 0 )) ||
  fail "a resume against an already-populated destination whose signature does NOT verify must fail; rc=${rc}"$'\n'"${out}"
if grep -Fq 'V1 gate PASSED' <<<"${out}"; then
  fail "the V1 gate reported PASSED on a resume without evaluating the signature -- the gate can be laundered by re-running:"$'\n'"${out}"
fi
logged "${scene}" '^cosign verify' ||
  fail "the resume path must still run 'cosign verify' -- bytes present at the destination are not evidence that they verify"
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.15\.0 "; then
  fail "the bulk copy ran after a V1 gate that never verified anything"
fi

# And the honest half of the same path: when it DOES verify, a resume is still a skip.
scene="$(new_scene resume_ok)"
seed_all_sources "${scene}"
seed "${scene}" "${DST}:0.14.0" "$(dig 0140)"
seed "${scene}" "${DST}:sha256-$(dig 0140 | sed 's/^sha256://').sig" "$(dig 5165)"
run_migrate "${scene}" "${SCRIPT}" --apply
(( rc == 0 )) || fail "a resume whose destination verifies must succeed; rc=${rc}"$'\n'"${out}"
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.14\.0 "; then
  fail "a chart already present at the destination with a matching digest must not be re-copied -- the script must be idempotent"
fi
grep -Fq 'V1 gate PASSED' <<<"${out}" ||
  fail "a verifying resume must still pass the V1 gate"

pass "8c: the V1 gate re-verifies on resume and cannot be laundered by re-running"

# --- 8d. The V1 gate's own guard. ------------------------------------------
# A source tag missing for 0.14.0 is a skip, not a verification. Without the guard the flag
# is set anyway and five more charts are copied on the strength of a gate that never ran.
scene="$(new_scene v1_missing)"
seed "${scene}" "${SRC}:0.15.0" "$(dig 0150)"
seed "${scene}" "${SRC}:0.16.0" "$(dig 0160)"
run_migrate "${scene}" "${SCRIPT}" --apply
(( rc != 0 )) ||
  fail "the V1 gate must refuse to pass when its chart was skipped; rc=${rc}"$'\n'"${out}"
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.15\.0 "; then
  fail "the bulk copy ran even though the V1 gate chart was never verified"
fi

pass "8d: a skipped V1 chart stops the run before the bulk copy"

# --- 8e. Never overwrite a divergent destination. --------------------------
scene="$(new_scene clobber)"
seed_all_sources "${scene}"
seed "${scene}" "${DST}:0.14.0" "$(dig 9999)"
run_migrate "${scene}" "${SCRIPT}" --apply
(( rc != 0 )) ||
  fail "a destination holding a DIFFERENT digest must stop the run, not be overwritten; rc=${rc}"$'\n'"${out}"
if logged "${scene}" '^cosign copy'; then
  fail "the script issued a copy over a destination whose digest differs from the source -- it must refuse and stop:"$'\n'"$(grep -E '^cosign copy' "${scene}/log")"
fi

pass "8e: a divergent destination is refused, never clobbered"

# --- 8f. Verification 2 and 3 are load-bearing. ----------------------------
scene="$(new_scene verify_fail)"
seed_all_sources "${scene}"
STUB_VERIFY_FAIL=1
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_VERIFY_FAIL=0
(( rc != 0 )) || fail "a failing 'cosign verify' must stop the run; rc=${rc}"$'\n'"${out}"
grep -Fq 'cosign verify FAILED' <<<"${out}" ||
  fail "a failing cosign verify must say so; got:"$'\n'"${out}"
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.15\.0 "; then
  fail "the bulk copy ran after cosign verify failed on the V1 chart"
fi

# A signature that verifies but was minted from a laptop presents the maintainer's own OIDC
# identity. It satisfies a naive check and fails the command users are told to run.
scene="$(new_scene wrong_identity)"
seed_all_sources "${scene}"
STUB_SUBJECT_TEMPLATE="${LAPTOP_SUBJECT_TEMPLATE}"
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_SUBJECT_TEMPLATE="${GOOD_SUBJECT_TEMPLATE}"
(( rc != 0 )) ||
  fail "a signature whose certificate identity is NOT the original release-time workflow must stop the run; rc=${rc}"$'\n'"${out}"
grep -Fq 'release.yaml@refs/tags/v0.14.0' <<<"${out}" ||
  fail "the identity failure must name the identity it expected; got:"$'\n'"${out}"
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.15\.0 "; then
  fail "the bulk copy ran after the V1 chart's certificate identity check failed"
fi

pass "8f: cosign verify and the certificate-identity check both stop the run"

# --- 8g. The exclusion holds at runtime, not just in CI. -------------------
# CI's count assertion (section 2) only sees the committed file. A maintainer resuming a
# partial run by hand-editing BULK_VERSIONS never goes through CI at all, so the invariant
# has to hold in the process that does the writing. Proven by running exactly that edit.
# The variant lives in a mimic of the repo layout (hack/<script> next to a repo root
# holding artifacthub-repo.yml), because the script derives REPO_ROOT from its own path and
# Phase 0 refuses to start without that file. Building it under a bare temp dir made the
# run die in Phase 0, which would have made this assertion pass for the wrong reason.
mkdir -p "${stub_root}/repo/hack"
cp "${ROOT}/artifacthub-repo.yml" "${stub_root}/repo/artifacthub-repo.yml"
widened="${stub_root}/repo/hack/migrate-chart-path.sh"
sed -E 's/^BULK_VERSIONS=\(.*\)$/BULK_VERSIONS=(0.13.0 0.15.0)/' "${SCRIPT}" >"${widened}"
grep -Fq 'BULK_VERSIONS=(0.13.0 0.15.0)' "${widened}" ||
  fail "could not build the widened-version-list variant; this assertion would pass vacuously"
scene="$(new_scene excluded)"
seed_all_sources "${scene}"
seed "${scene}" "${SRC}:0.13.0" "$(dig 0130)"
run_migrate "${scene}" "${widened}" --apply
(( rc != 0 )) ||
  fail "a hand-edited version list containing 0.13.0 must be refused at runtime; rc=${rc}"$'\n'"${out}"
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.13\.0 "; then
  fail "0.13.0 was COPIED after being added to the version list by hand. The never-republish list must be enforced in the running process, not only by a CI count assertion:"$'\n'"$(grep -E '^cosign copy' "${scene}/log")"
fi
# Non-vacuity: the run must have got far enough to try, i.e. past the V1 gate.
grep -Fq 'V1 gate PASSED' <<<"${out}" ||
  fail "the widened variant did not reach the bulk copy, so the runtime exclusion was never exercised:"$'\n'"${out}"

pass "8g: the never-republish list is enforced at runtime, defeating a hand-edited version list"

# --- 8h. A transient registry error is not "the tag is absent". ------------
# `crane digest ... || true` turned a rate limit into a skip, and a skip into exit 0: a
# chart silently missing from the new path, found only after the URL repoint.
# The assertions here are about WHERE the run stops, not merely that it stops. An earlier
# version checked only `rc != 0` and that the word TOOMANYREQUESTS appeared, and both of
# those are ALSO true of the broken behaviour: the non-zero exit comes from the end-of-run
# SKIPPED_VERSIONS check, and the error text comes from digest_of's stderr, which is a
# different code path from the `die` being tested. Deleting the tri-state `die` therefore
# left the gate fully green while the run went on to copy 0.17.0-0.19.0 and push metadata
# to a live registry. The discriminating facts are the two below: with the fix, the run
# stops at 0.16.0, so 0.17.0 is never attempted and Phase 4 is never reached.
scene="$(new_scene flake)"
seed_all_sources "${scene}"
STUB_CRANE_FLAKE="${SRC}:0.16.0"
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_CRANE_FLAKE=""
(( rc != 0 )) ||
  fail "a transient (non-404) crane failure must stop the run, not be swallowed as 'no such tag'; rc=${rc}"$'\n'"${out}"
grep -Fiq 'TOOMANYREQUESTS' <<<"${out}" ||
  fail "the underlying registry error must be shown to the operator; got:"$'\n'"${out}"

# Non-vacuity: the run must have got as far as 0.16.0, or "0.17.0 was not attempted" is
# true for an unrelated reason.
logged "${scene}" "^crane digest ${SRC//./\\.}:0\.16\.0$" ||
  fail "the flake scenario never reached 0.16.0, so the assertions below prove nothing"$'\n'"${out}"

# THE discriminating assertion. A transient error misread as "no such tag" is a warn-and-
# skip, and the loop carries on to the next chart.
if logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.17\.0 "; then
  fail "0.17.0 was COPIED after a TRANSIENT registry failure on 0.16.0. The failure was misread as 'no such tag' and skipped; the run must stop at the point the registry stopped answering, because everything after it is being decided on an unknown registry state:"$'\n'"$(grep -E '^cosign copy' "${scene}/log")"
fi
# ...and it must certainly not have reached the live metadata push.
if logged "${scene}" '^oras push'; then
  fail "the Artifact Hub metadata push ran after a transient registry failure was swallowed as a skip"
fi
# The operator must not be told a tag is missing when the registry simply did not answer:
# that is the sentence that would send them to look for a chart that is actually there.
if grep -Eq "no such tag at ${SRC//./\\.}:0\.16\.0" <<<"${out}"; then
  fail "a transient registry failure was reported to the operator as 'no such tag' -- it is not the same fact, and acting on it means concluding a published chart does not exist:"$'\n'"${out}"
fi

# A genuinely absent source tag is still a skip -- but the run must not exit 0 having
# quietly dropped a version that will then be missing from Artifact Hub.
scene="$(new_scene absent)"
for v in 0.14.0 0.15.0 0.16.0 0.17.0 0.18.0; do
  seed "${scene}" "${SRC}:${v}" "$(dig "${v//./}")"
done
run_migrate "${scene}" "${SCRIPT}" --apply
(( rc != 0 )) ||
  fail "a run that skipped a chart must NOT exit 0 -- that version will be absent from the hub after the repoint; rc=${rc}"$'\n'"${out}"
grep -Fq '0.19.0' <<<"${out}" ||
  fail "the incomplete-run report must name the versions that were not copied; got:"$'\n'"${out}"
# It must still be a completed pass over the other charts, not an abort on the first gap.
logged "${scene}" "^cosign copy --force ${SRC//./\\.}:0\.18\.0 " ||
  fail "a missing tag must not abort the remaining copies; 0.18.0 was never attempted"

# The same tri-state contract has to hold at the SIGNATURE-tag read, and that one is easy
# to get wrong in a way the exit code hides: written as `[[ -n "$(digest_or_absent ...)" ]]`
# the command substitution sits inside a condition, where `set -e` does not apply -- so the
# die printed "could not determine whether ... exists" and the script carried straight on,
# re-copied, and exited 0 reporting "V1 gate PASSED". The outcome was benign (an extra
# idempotent copy, then full verification) but the message reads as fatal everywhere else
# in the file, so an operator seeing it would reasonably believe the run had stopped.
scene="$(new_scene sigflake)"
seed_all_sources "${scene}"
seed "${scene}" "${DST}:0.14.0" "$(dig 0140)"
STUB_CRANE_FLAKE="${DST}:sha256-$(dig 0140 | sed 's/^sha256://').sig"
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_CRANE_FLAKE=""
(( rc != 0 )) ||
  fail "a transient failure while reading the SIGNATURE tag must stop the run like any other indeterminate read; instead the script printed a fatal-sounding message and carried on to rc=0:"$'\n'"${out}"
if grep -Fq 'V1 gate PASSED' <<<"${out}"; then
  fail "the run reported 'V1 gate PASSED' after a registry read whose result it could not determine"
fi

pass "8h: transient errors stop the run at the subject AND signature reads; a genuinely absent tag skips but never exits 0"

# --- 8i. The credential never reaches argv. --------------------------------
# Anything on a command line is readable from /proc/<pid>/cmdline for the duration of the
# call. The script reasons about exactly this for the registry logins (--password-stdin);
# the curl calls have to hold the same line, and one of them runs in dry run too.
scene="$(new_scene creds)"
seed_all_sources "${scene}"
run_migrate "${scene}" "${SCRIPT}"
(( rc == 0 )) || fail "dry run must succeed for the credential-exposure scenario; rc=${rc}"$'\n'"${out}"
# Non-vacuity: curl must actually have been called with a config on stdin, or "the token is
# not in argv" would be true simply because nothing authenticated at all.
[[ -s "${scene}/curl_stdin" ]] ||
  fail "no curl invocation supplied a credential on stdin; the argv assertion below would pass vacuously"
grep -Fq "${TEST_TOKEN}" "${scene}/curl_stdin" ||
  fail "the token did not reach curl on stdin, so the preflight is not authenticating"
if grep -Fq "${TEST_TOKEN}" "${scene}/log"; then
  fail "the registry token appears in a command's ARGV, readable from /proc/<pid>/cmdline:"$'\n'"$(grep -Fn "${TEST_TOKEN}" "${scene}/log")"
fi
if grep -Fq "${TEST_TOKEN}" <<<"${out}"; then
  fail "the registry token was printed to stdout/stderr:"$'\n'"${out}"
fi

pass "8i: the token reaches curl on stdin and never appears in argv or output"

# --- 8j. Phase 0 and Phase 3 refusals. -------------------------------------
# A pull-only token is indistinguishable from a write-capable one until the first push
# fails -- by which point the V1 gate has already "passed".
scene="$(new_scene pullonly)"
seed_all_sources "${scene}"
STUB_GHCR_JWT="${PULL_JWT}"
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_GHCR_JWT="${PUSH_JWT}"
(( rc != 0 )) || fail "a token granting only 'pull' must be refused in Phase 0; rc=${rc}"$'\n'"${out}"
grep -Fq 'write:packages' <<<"${out}" ||
  fail "the pull-only refusal must name the scope that is missing; got:"$'\n'"${out}"
if logged "${scene}" '^cosign copy'; then
  fail "a copy was attempted with a pull-only token"
fi

# GHCR creates packages private. A private package is invisible to Artifact Hub, which
# reads anonymously, while every command in the migration reports success.
scene="$(new_scene private)"
seed_all_sources "${scene}"
STUB_PKG_VISIBILITY=private
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_PKG_VISIBILITY=public
(( rc != 0 )) || fail "a private destination package must stop the run before the metadata push; rc=${rc}"$'\n'"${out}"
if logged "${scene}" '^oras push'; then
  fail "the metadata push ran against a PRIVATE package -- it succeeds locally and Artifact Hub sees nothing"
fi
grep -Fiq 'settings' <<<"${out}" ||
  fail "the private-package refusal must give the click-path to publish it; got:"$'\n'"${out}"

# A 404 under --apply means Phases 1-2 reported copies that did not create the package.
# Continuing from there would push metadata to a coordinate that holds no chart.
scene="$(new_scene pkg404)"
seed_all_sources "${scene}"
STUB_PKG_HTTP=404
run_migrate "${scene}" "${SCRIPT}" --apply
STUB_PKG_HTTP=200
(( rc != 0 )) ||
  fail "a 404 from the packages API under --apply must stop the run -- the copies claimed to have created the package; rc=${rc}"$'\n'"${out}"
if logged "${scene}" '^oras push'; then
  fail "the metadata push ran even though the destination package does not exist"
fi

# assert_token_shape. It is a security guard added in this lane, and it was shipping with
# no coverage at all. curl config files take one directive per line, so a token carrying a
# newline followed by `output = /path` makes curl write the response body wherever the
# value says -- demonstrated with real curl during review. The value comes from the
# operator's own environment, so this is self-inflicted rather than an external attack, but
# a guard that exists to make an escaping question disappear has to be shown to hold.
# Case 1 isolates the NEWLINE. Every other character in this payload is [A-Za-z0-9_], so
# the only thing the guard can be rejecting is the line break -- a mutant that admits a
# newline while still rejecting '/', '=' or spaces cannot survive it. And a newline alone
# is enough: `insecure` is a bare curl directive needing no argument, so "the payload has
# no punctuation" is not a defence.
scene="$(new_scene badtoken_newline)"
seed_all_sources "${scene}"
TOKEN_OVERRIDE=$'ghp_FAKESECRET_ok\ninsecure'
run_migrate "${scene}" "${SCRIPT}"
TOKEN_OVERRIDE=""
(( rc != 0 )) ||
  fail "a token whose ONLY offending character is a newline must be refused in Phase 0; rc=${rc}"$'\n'"${out}"
grep -Fiq 'token' <<<"${out}" ||
  fail "the refusal must tell the operator it is about the token; got:"$'\n'"${out}"
# The point of refusing early: no curl config is ever emitted, so the injected directive
# never reaches a parser. Asserted on the capture, not inferred from the exit code.
[[ ! -s "${scene}/curl_stdin" ]] ||
  fail "a malformed token reached curl's config parser on stdin -- the shape check must run BEFORE anything interpolates it:"$'\n'"$(cat "${scene}/curl_stdin")"
if logged "${scene}" '^curl '; then
  fail "curl was invoked at all with a malformed token; Phase 0 must stop first"
fi

# Case 2 is the full write-primitive payload, kept because it names the concrete harm. The
# target lives under the per-run stub_root, not at a fixed /tmp path: a fixed path can be
# pre-seeded by anything on the machine, and this assertion would then red with a message
# falsely claiming the injection had been honoured -- a test that lies about what it saw.
PWNED="${stub_root}/proof-injection"
[[ ! -e "${PWNED}" ]] ||
  fail "${PWNED} already exists before the run; this assertion could not distinguish an injection from a leftover"
scene="$(new_scene badtoken_output)"
seed_all_sources "${scene}"
TOKEN_OVERRIDE=$'ghp_FAKESECRET_ok\noutput = '"${PWNED}"
run_migrate "${scene}" "${SCRIPT}"
TOKEN_OVERRIDE=""
(( rc != 0 )) ||
  fail "a token carrying an 'output =' curl directive must be refused in Phase 0; rc=${rc}"$'\n'"${out}"
[[ ! -e "${PWNED}" ]] ||
  fail "the injected 'output =' directive was honoured: ${PWNED} was created by curl writing a response body to an attacker-named path"

pass "8j: pull-only tokens, private and missing packages, and malformed tokens are all refused before they can do harm"

# --- 8k. Usage errors are not verdicts. ------------------------------------
# `shift` past the end of the argument list trips `set -e` and exits 1 with no output --
# and 1 is the documented FAIL code, so a typo'd baseline was indistinguishable from
# "Artifact Hub still reports errors".
# Word-split via an explicit array rather than an unquoted expansion. (The previous
# version carried a `# shellcheck disable=SC2086` that attached to the following `set +e`
# and was therefore inert -- a disable comment that silenced nothing.)
for bad_args in "--since" "--interval" "--timeout" "--since notanumber" "--this-flag-does-not-exist"; do
  read -r -a bad_argv <<<"${bad_args}"
  set +e
  usage_out="$(env -u GHCR_TOKEN -u GITHUB_TOKEN -u CR_PAT bash "${SCRIPT}" "${bad_argv[@]}" 2>&1)"
  usage_rc=$?
  set -e
  (( usage_rc != 0 )) || fail "'${bad_args}' must be rejected; it exited 0"
  (( usage_rc != 1 )) ||
    fail "'${bad_args}' exited 1, which is the documented FAIL verdict -- a usage error must not be confusable with a migration result"
  (( usage_rc != 2 )) ||
    fail "'${bad_args}' exited 2, which is the documented INCONCLUSIVE verdict"
  [[ -n "${usage_out}" ]] ||
    fail "'${bad_args}' produced no output at all; the operator has nothing to act on"
done

pass "8k: bad arguments exit with a usage code and a message, never a migration verdict"

# --- 8l. --verify-ac1 against a live (stubbed) endpoint. -------------------
# Everything in section 7 runs through MIGRATE_AH_FIXTURES, which short-circuits the HTTP
# path entirely -- so the poll loop's live behaviour, including the --interval floor, had
# no coverage at all. These two scenarios reach the curl branch: the fixture variable must
# stay UNSET for that.
ah_json="${stub_root}/ah.json"
write_fixture "${ah_json}" 1788190801 'null' "${NEW_URL}" true
ah_count="${stub_root}/ah.count"

elapsed=0
run_ac1_live() {
  : >"${ah_count}"
  local t0=${SECONDS}
  set +e
  out="$(env "PATH=${stub_bin}:${PATH}" \
    "STUB_LOG=${stub_root}/ac1.log" "STUB_CURL_STDIN=${stub_root}/ac1.stdin" \
    "STUB_AH_JSON=${ah_json}" "STUB_AH_COUNT=${ah_count}" "STUB_AH_HTTP=${1}" \
    "STUB_AH_OK_FIRST=${STUB_AH_OK_FIRST:-}" \
    bash "${SCRIPT}" --verify-ac1 "${@:2}" 2>&1)"
  rc=$?
  set -e
  elapsed=$(( SECONDS - t0 ))
}

# --interval 0 against a public third-party API is an unbounded hot loop for as long as
# --timeout allows -- measured at ~724 requests in 10s, which over the default 3600s
# timeout is roughly a quarter of a million requests to artifacthub.io. The floor is what
# stops that, and the nap is bounded by the remaining timeout so the floor does not
# overshoot a short deadline either.
run_ac1_live 200 --interval 0 --timeout 2
requests="$(wc -l <"${ah_count}")"
(( requests >= 2 )) ||
  fail "the live poll loop issued ${requests} request(s) in a 2s window; it must poll at least twice, or the assertion below is measuring a loop that never ran"
(( requests <= 5 )) ||
  fail "the live poll loop issued ${requests} requests in 2 seconds. --interval must be clamped to a floor: artifacthub.io is a public third-party API and its tracking runs are ~30 minutes apart, so a tighter poll adds load and observes nothing"
grep -Fiq 'raised to' <<<"${out}" ||
  fail "the operator must be told their --interval was raised, and why; got:"$'\n'"${out}"
# The floor must not overshoot the deadline the operator set. A 60s floor under
# --timeout 2 has to spend 2 seconds, not 60: the nap is bounded by the time remaining.
# Measured in wall clock, because that is the thing an operator actually experiences and
# the only signal that separates "slept the floor" from "slept what was left".
(( elapsed <= 15 )) ||
  fail "--verify-ac1 --timeout 2 took ${elapsed}s. The --interval floor is being slept in full instead of being bounded by the time remaining, so a short --timeout is ignored"
(( requests >= 2 )) ||
  fail "only ${requests} request(s) in the timeout window; the loop must actually poll again, or the elapsed-time assertion above is measuring a loop that exited immediately"

# A failing HTTP read must stop. ah_sample's die runs inside a command substitution, and
# `set -e` does not apply to one used as an ARGUMENT -- so the script printed "could not
# read <url>", a sentence that means "stopped" everywhere else in this file, and then kept
# polling and reported a run with an empty timestamp and an empty baseline.
run_ac1_live 503 --interval 0 --timeout 2
(( rc != 0 )) ||
  fail "a failing read of the Artifact Hub endpoint must stop the mode; rc=${rc}"$'\n'"${out}"
# WHERE it stops is the discriminating fact. ah_sample's die runs inside a command
# substitution, and `set -e` does not apply to one used as an ARGUMENT -- so a call site
# written as `record_run "$(ah_sample ...)"` swallows the failure and the loop polls again.
# A run that stopped correctly reads the endpoint ONCE and never announces a next attempt;
# checking only the exit code cannot tell the two apart, because a later call site that
# DOES propagate will end the run anyway, several requests later.
read_failures="$(grep -c 'could not read https://artifacthub.io' <<<"${out}" || true)"
(( read_failures >= 1 )) ||
  fail "the failed read was never reported to the operator at all; got:"$'\n'"${out}"
(( read_failures == 1 )) ||
  fail "the Artifact Hub read failed ${read_failures} times: the mode carried on past the first failure instead of stopping at it. 'could not read ...' means stopped everywhere else in this file"
if grep -Fq 'waiting' <<<"${out}"; then
  fail "the mode announced another poll after a failed read; it must stop at the read it could not complete:"$'\n'"${out}"
fi
if grep -Eq 'RESULT:' <<<"${out}"; then
  fail "the mode reported a verdict after a read it could not complete:"$'\n'"${out}"
fi
if grep -Eq 'last_tracking_ts=[[:space:]]*$|last_tracking_ts=[[:space:]]+\(' <<<"${out}"; then
  fail "the mode carried on past a failed read and reported a tracking run with an empty timestamp:"$'\n'"${out}"
fi
if grep -Eq -- '--since[[:space:]]*$' <<<"${out}"; then
  fail "the mode printed an empty '--since' baseline after a failed read; an operator would paste that back:"$'\n'"${out}"
fi

# The SAME contract on a read that is not the first. Every call site that reaches
# ah_sample has to propagate, and a scenario that only ever fails read 1 cannot see the
# others: whichever later call site still propagates ends the run anyway, so the exit code
# looks right while the mode has quietly polled on past a failure it reported as fatal.
# Here read 1 succeeds and the poll read fails.
STUB_AH_OK_FIRST=1
run_ac1_live 503 --interval 0 --timeout 2
STUB_AH_OK_FIRST=""
(( rc != 0 )) ||
  fail "a failing read inside the poll loop must stop the mode; rc=${rc}"$'\n'"${out}"
grep -Fq 'could not read https://artifacthub.io' <<<"${out}" ||
  fail "the failed poll read was never reported; got:"$'\n'"${out}"
if grep -Eq 'RESULT:' <<<"${out}"; then
  fail "the mode reported a verdict after a poll read it could not complete. 'could not read ...' means stopped everywhere else in this file, and a verdict computed from a run it never actually observed is worse than no verdict:"$'\n'"${out}"
fi
if grep -Eq 'last_tracking_ts=[[:space:]]*$|last_tracking_ts=[[:space:]]+\(' <<<"${out}"; then
  fail "the mode recorded a tracking run with an empty timestamp from a failed poll read:"$'\n'"${out}"
fi

# A fixture path that does not exist must be an error, not an empty document parsed as
# "no such repository" -- a wrong answer wearing a real one's clothes.
out_missing="$(MIGRATE_AH_FIXTURES="${stub_root}/does-not-exist.json" bash "${SCRIPT}" --verify-ac1 --interval 0 --timeout 1 2>&1 || true)"
grep -Fiq 'does not exist' <<<"${out_missing}" ||
  fail "a missing MIGRATE_AH_FIXTURES entry must be reported as missing, not read as an empty response; got:"$'\n'"${out_missing}"

pass "8l: the live poll loop is rate-floored, stops on a failed read, and rejects a missing fixture"

echo "All dist chart path migration tests passed."
