#!/usr/bin/env bash
# SVS-1.2 (openspec change scheduled-vulncheck): the daily `Vulnerability scan` step.
#
# Runs `task vulncheck`, prints all of its output, and exits with task's status. Task reports a
# failed command as its own exit code (201 for Task 3.54.0), so that is the status passed on, not
# govulncheck's. On a non-zero status it then prints one line per advisory in the report,
#   FAIL: GO-YYYY-NNNN <title>
# where the title is the first non-blank line after `Vulnerability #N: GO-YYYY-NNNN`, trimmed
# (govulncheck wraps titles at 80 columns, so a long one is cut at its first line), or, when the
# report names no advisory, one line saying the failure is a tool, network or build failure.
# ci-failure-report.yaml copies the log lines holding FAIL into its issue, and govulncheck prints
# none of its own, so these lines are what names the advisory there.
#
# Pinned by hack/test/vulncheck_schedule_test.sh (V-3, against a fake task).
#
# Usage: hack/ci/vulncheck-scheduled.sh
set -uo pipefail

out="$(mktemp)" || {
  echo "FAIL: vulncheck-scheduled: cannot create a temporary file"
  exit 1
}
trap 'rm -f "${out}"' EXIT

task vulncheck 2>&1 | tee "${out}"
status="${PIPESTATUS[0]}"

found=0
report() { # <id> <title>
  found=$((found + 1))
  if [[ -n "$2" ]]; then
    printf 'FAIL: %s %s\n' "$1" "$2"
  else
    printf 'FAIL: %s\n' "$1"
  fi
}

if [[ "${status}" -ne 0 ]]; then
  advisory='^Vulnerability #[0-9]+: (GO-[0-9]{4}-[0-9]+)[[:space:]]*$'
  id=""
  while IFS= read -r line || [[ -n "${line}" ]]; do
    if [[ "${line}" =~ ${advisory} ]]; then
      [[ -z "${id}" ]] || report "${id}" ""
      id="${BASH_REMATCH[1]}"
    elif [[ -n "${id}" && "${line}" =~ [^[:space:]] ]]; then
      title="${line}"
      title="${title#"${title%%[![:space:]]*}"}"
      title="${title%"${title##*[![:space:]]}"}"
      report "${id}" "${title}"
      id=""
    fi
  done <"${out}"
  [[ -z "${id}" ]] || report "${id}" ""
  ((found > 0)) ||
    echo "FAIL: task vulncheck exited ${status}; no advisory in its output (tool, network or build failure)"
fi

exit "${status}"
