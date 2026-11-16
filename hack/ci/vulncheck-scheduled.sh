#!/usr/bin/env bash
# SVS-1.3 (openspec change scheduled-vulncheck): the daily `Vulnerability scan` step.
#
# Runs `task vulncheck`, prints all of its output, and exits with task's status. Task reports a
# failed command as its own exit code (201 for Task 3.54.0), so that is the status passed on, not
# govulncheck's. On a non-zero status it then prints one line per advisory in the report,
#   FAIL: GO-YYYY-NNNN <title>
# where the title is the non-blank lines after `Vulnerability #N: GO-YYYY-NNNN` up to a blank
# line, a `More info:` line or the next entry, each trimmed and joined with single spaces
# (govulncheck wraps titles at 80 columns), or, when the report names no advisory, one line saying
# the failure is a tool, network or build failure. When the output cannot be captured (tee fails),
# it prints one capture-failure line instead, reads no verdict from the capture, and exits
# non-zero: task's status, or 1 when task passed.
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
codes=("${PIPESTATUS[@]}")
status="${codes[0]}"
captured="${codes[1]}"

if [[ "${captured}" -ne 0 ]]; then
  echo "FAIL: vulncheck-scheduled: could not capture the scan output"
  exit "$((status != 0 ? status : 1))"
fi

found=0
id=""
title=""
flush() { # print the pending entry's FAIL line, if there is one
  [[ -n "${id}" ]] || return 0
  found=$((found + 1))
  if [[ -n "${title}" ]]; then
    printf 'FAIL: %s %s\n' "${id}" "${title}"
  else
    printf 'FAIL: %s\n' "${id}"
  fi
  id=""
  title=""
}

if [[ "${status}" -ne 0 ]]; then
  advisory='^Vulnerability #[0-9]+: (GO-[0-9]{4}-[0-9]+)[[:space:]]*$'
  while IFS= read -r line || [[ -n "${line}" ]]; do
    if [[ "${line}" =~ ${advisory} ]]; then
      flush
      id="${BASH_REMATCH[1]}"
      continue
    fi
    [[ -n "${id}" ]] || continue
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    if [[ -z "${line}" ]]; then
      # A blank line before the title is skipped; one after it ends the title.
      [[ -z "${title}" ]] || flush
    elif [[ "${line}" == "More info:"* ]]; then
      flush
    else
      title="${title:+${title} }${line}"
    fi
  done <"${out}"
  flush
  ((found > 0)) ||
    echo "FAIL: task vulncheck exited ${status}; no advisory in its output (tool, network or build failure)"
fi

exit "${status}"
