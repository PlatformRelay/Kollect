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
# Run with a deliberately hostile environment: no registry token in scope, and PATH
# reduced so that a stray cosign/crane/oras/curl invocation would fail loudly rather
# than silently succeed on the runner.
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
# live migration. Assert the *documented* default, and (below, via --plan) the behaviour.
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

# 0.14.0 is not just "the first copy": it is the V1 gate, the one chart whose copied
# signature is verified before five more are copied on the strength of that result. If the
# plan stops distinguishing it, the gate has been flattened into a plain bulk copy.
grep -Eq '^plan: copy 0\.14\.0.*(V1|v1)' <<<"${plan_out}" ||
  fail "--plan must mark 0.14.0 as the V1 gate chart, distinct from the bulk copies"

# Direction of travel. Catches a transposed source/destination -- which, with --apply,
# would push the new path's content back over the controller image repository.
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
IDENTITY_REGEXP='^https://github.com/platformrelay/kollect/.+'
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
  fail "--verify-ac1 must be able to report INCONCLUSIVE; a mode that can only say PASS or FAIL cannot express 'the tracking run did not advance'"
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
trap 'rm -rf "${fixture_dir}"' EXIT
write_fixture() {
  # $1 = path, $2 = last_tracking_ts, $3 = last_tracking_errors (JSON string body), $4 = url, $5 = verified_publisher
  cat >"$1" <<JSON
[{"repository_id":"cb3be9a6-8e3b-4419-9de5-1184fe349c29","name":"kollect","url":"$4","kind":0,"verified_publisher":$5,"last_tracking_ts":$2,"last_tracking_errors":$3}]
JSON
}

NEW_URL="oci://${NEW_PATH}"
write_fixture "${fixture_dir}/clean_a.json" 1788190801 'null' "${NEW_URL}" true
write_fixture "${fixture_dir}/clean_b.json" 1788194401 'null' "${NEW_URL}" true
write_fixture "${fixture_dir}/errors_b.json" 1788194401 '"error preparing package: error loading chart (oci://ghcr.io/platformrelay/charts/kollect:v0.19.0): layer not found"' "${NEW_URL}" true
write_fixture "${fixture_dir}/oldurl_b.json" 1788194401 'null' "oci://${OLD_PATH}" true
write_fixture "${fixture_dir}/unverified_b.json" 1788194401 'null' "${NEW_URL}" false

verify_ac1() {
  # $1 = colon-separated fixture list; remaining args passed through.
  local fixtures="$1"
  shift
  MIGRATE_AH_FIXTURES="${fixtures}" bash "${SCRIPT}" --verify-ac1 --interval 0 --timeout 1 "$@" 2>&1
}

# (a) The control: two reads whose timestamp genuinely advanced, no errors, new URL,
#     verified publisher still true. This MUST be able to reach PASS -- otherwise every
#     negative case below is satisfied by a mode that can never say PASS at all, which is
#     the classic way a safety assertion becomes vacuous.
out_pass="$(verify_ac1 "${fixture_dir}/clean_a.json:${fixture_dir}/clean_b.json" --since 1788190800 || true)"
grep -Eq '^(RESULT: )?PASS' <<<"${out_pass}" ||
  fail "--verify-ac1 must report PASS for two reads with an ADVANCED timestamp, no tracking errors, the new URL and verified_publisher true; got:"$'\n'"${out_pass}"

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

echo "All dist chart path migration tests passed."
