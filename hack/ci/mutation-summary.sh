#!/usr/bin/env bash
# Mutation counts per package for the nightly job summary (MUT-1, MUT-2, openspec change
# mutation-testing-signal).
#
# Usage: hack/ci/mutation-summary.sh <package>=<report> ...
#   exactly one argument per measured package (internal/aggregate, internal/redact,
#   internal/pathvalidate), each naming the gremlins JSON report of that package.
#
# Writes one table row per package with its killed, lived, not covered, timed out and not viable
# counts, counted from the report's mutation statuses, to $GITHUB_STEP_SUMMARY (stdout outside
# Actions). It is a measurement, not a gate: any number of survivors exits 0. Every report is
# validated before anything is printed, so a broken one fails the run with no partial table:
# missing, empty, not one JSON object, no files array, no mutation at all, a status that is not a
# measurement (RUNNABLE from a dry run, SKIPPED, anything unknown), or a file that is not in the
# package directory it is passed for (reports swapped between packages). A missing, repeated or
# unknown package fails too. Diagnostics are ::error::mutation: lines on stderr.
# Locked by hack/test/mutation_signal_test.sh (M1-M8).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
PACKAGES=(internal/aggregate internal/redact internal/pathvalidate)
STATUSES='["KILLED", "LIVED", "NOT COVERED", "TIMED OUT", "NOT VIABLE"]'

errors=0
err() {
  printf '::error::mutation: %s\n' "$*" >&2
  errors=$((errors + 1))
}

declare -A report=()
for arg in "$@"; do
  pkg="${arg%%=*}" path="${arg#*=}"
  if [[ "${arg}" != *=* || -z "${pkg}" || -z "${path}" ]]; then
    err "argument '${arg}' is not <package>=<report>"
    continue
  fi
  known=0
  for p in "${PACKAGES[@]}"; do
    [[ "${p}" != "${pkg}" ]] || known=1
  done
  if ((known == 0)); then
    err "${pkg}: not a measured package (measured: ${PACKAGES[*]})"
    continue
  fi
  if [[ -n "${report[${pkg}]+set}" ]]; then
    err "${pkg}: given more than once"
    continue
  fi
  report[${pkg}]="${path}"
done
for p in "${PACKAGES[@]}"; do
  [[ -n "${report[${p}]+set}" ]] || err "${p}: no report given; every measured package needs one"
done

# Validate every report first; nothing is printed until all of them pass.
declare -A row=()
for p in "${PACKAGES[@]}"; do
  [[ -n "${report[${p}]+set}" ]] || continue
  r="${report[${p}]}"
  if [[ ! -f "${r}" ]]; then
    err "${p}: report ${r} is missing"
    continue
  fi
  if [[ ! -s "${r}" ]]; then
    err "${p}: report ${r} is empty"
    continue
  fi
  if ! verdict="$(jq -r -s --argjson known "${STATUSES}" '
      if length != 1 then "not one JSON document"
      elif (.[0] | type) != "object" then "not a JSON object"
      else .[0]
        | if (.files | type) != "array" then "no files array"
          elif ([.files[] | (type == "object") and ((.file_name | type) == "string") and ((.mutations | type) == "array")] | all | not) then "a files entry without file_name or mutations"
          elif ([.files[].mutations[]] | length) == 0 then "no mutation: nothing was measured"
          elif ([.files[].mutations[] | (.status | type) == "string"] | all | not) then "a mutation without a status"
          else ([.files[].mutations[].status | select(. as $st | $known | index($st) | not)] | unique) as $bad
            | if ($bad | length) > 0 then "status " + ($bad | join(", ")) + " is not a measurement" else "ok" end
          end
      end' "${r}" 2>/dev/null)"; then
    err "${p}: report ${r} is not valid JSON"
    continue
  fi
  if [[ "${verdict}" != ok ]]; then
    err "${p}: report ${r}: ${verdict}"
    continue
  fi
  misplaced=0
  while IFS= read -r name; do
    if [[ "${name}" == */* || ! -f "${ROOT}/${p}/${name}" ]]; then
      err "${p}: report ${r} names ${name}, which is not a file of ${p} (a report of another package?)"
      misplaced=1
    fi
  done < <(jq -r '.files[].file_name' "${r}")
  ((misplaced == 0)) || continue
  row[${p}]="$(jq -r --argjson known "${STATUSES}" '[.files[].mutations[].status] as $all
    | $known | map(. as $k | $all | map(select(. == $k)) | length | tostring) | join(" | ")' "${r}")"
done
((errors == 0)) || exit 1

emit_table() {
  echo "### Mutation testing (report only)"
  echo
  echo "| Package | Killed | Lived | Not covered | Timed out | Not viable |"
  echo "| --- | --- | --- | --- | --- | --- |"
  for p in "${PACKAGES[@]}"; do
    echo "| ${p} | ${row[${p}]} |"
  done
  echo
  echo "A measurement, not a gate: survivors (lived) never fail this run. Not covered is code no test executes."
}
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  emit_table >>"${GITHUB_STEP_SUMMARY}"
else
  emit_table
fi
