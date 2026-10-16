#!/usr/bin/env bash
# CFR-1 to CFR-4, CFR-6 (openspec change ci-failure-reporting): turn the latest completed
# scheduled run of a reported workflow into one open, up-to-date `ci-failure` issue, and record
# its recovery.
#
# The workflow_run payload is a trigger only. The script reads from it the event, the head
# repository, the workflow path and name; it acts only on a `schedule` event whose head repository
# is this repository (CFR-6), and then on the latest completed scheduled run of that workflow,
# read with `gh run list`, at that run's attempt (CFR-1). It finds the newest bot issue for the
# workflow by bot authorship, the `ci-failure` label and the first-line marker of its body
# (CFR-4), reads the recorded key from the last bot marker of that issue (body or bot comment),
# and takes the action the decision table in spec.md CFR-1 names:
#
#   failure class (failure, timed_out, startup_failure, cancelled with a started job):
#     no issue, or closed with an older key -> open an issue; open with an older key -> comment;
#   success: open with an older key -> recovery comment, then close; open with an equal key whose
#     last marker is event=recovery -> close (retry of a failed close);
#   every other combination, and the no-op class -> no write.
#
# Hardening: every gh value is a quoted argument; every body goes through --body-file; every id
# used in a gh call or in arithmetic is checked to be a bounded positive integer first; a marker
# counts only on the first line of a bot-authored body or comment. Any gh error other than the
# log fetch stops the script before its next call, with gh's error on stderr; so does a read that
# exits 0 without exactly one JSON document of the expected shape. A failed or empty log fetch
# still writes (with "logs unavailable") and then exits non-zero.
#
# Environment (set by the workflow, never interpolated into a command):
#   GITHUB_EVENT_PATH   the workflow_run payload (JSON)
#   GITHUB_REPOSITORY   owner/repo this workflow runs in
#   GH_TOKEN            token with issues: write and actions: read (read by gh only)
#
# Exit codes: 0 done (including nothing to do); 1 a gh call failed, the logs were unavailable, or
# an input was malformed.
set -euo pipefail
export LC_ALL=C

LABEL=ci-failure
# The lookup lists every github-actions issue (labels are checked here, not by gh: a --label
# makes gh use the search API). A full page means the list may be cut, so the script stops.
LOOKUP_LIMIT=500
BOT_ISSUE_LOGIN='app/github-actions'
BOT_COMMENT_LOGIN='github-actions'
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILDER="${HERE}/ci-failure-issue-body.sh"

die() {
  echo "ci-failure-report: $*" >&2
  exit 1
}

need_int() { # need_int <name> <value> <max digits>
  [[ "$2" =~ ^[1-9][0-9]*$ && "${#2}" -le "$3" ]] || die "$1 is not a positive integer: '$2'"
}

# need_json <file> <jq test> <gh call>: a gh read that exited 0 must have printed exactly one JSON
# document passing the test. Empty or malformed output is an error, never "nothing found": an
# empty issue list read as "no issue" would open a duplicate issue.
need_json() {
  jq -e -s "length == 1 and (.[0] | $2)" "$1" >/dev/null 2>&1 ||
    die "$3 did not return one JSON document of the expected shape"
}

[[ -n "${GITHUB_EVENT_PATH:-}" && -r "${GITHUB_EVENT_PATH}" ]] || die "GITHUB_EVENT_PATH must name the payload file"
[[ "${GITHUB_REPOSITORY:-}" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]] ||
  die "GITHUB_REPOSITORY must be owner/repo, got '${GITHUB_REPOSITORY:-}'"
export GH_REPO="${GITHUB_REPOSITORY}"

# --- CFR-6: only scheduled runs of this repository ---------------------------------------------
event="$(jq -r '.workflow_run.event // ""' "${GITHUB_EVENT_PATH}")" || die "cannot read the payload"
head_repo="$(jq -r '.workflow_run.head_repository.full_name // ""' "${GITHUB_EVENT_PATH}")"
if [[ "${event}" != schedule || "${head_repo}" != "${GITHUB_REPOSITORY}" ]]; then
  echo "ci-failure-report: event '${event}' from '${head_repo}' is not a scheduled run of ${GITHUB_REPOSITORY}; nothing to do"
  exit 0
fi
wf_path="$(jq -r '.workflow_run.path // ""' "${GITHUB_EVENT_PATH}")"
[[ "${wf_path}" =~ ^\.github/workflows/[A-Za-z0-9._-]+\.ya?ml$ ]] || die "unexpected workflow path '${wf_path}'"
wf_name="$(jq -r '[.workflow_run.name, .workflow.name, .workflow_run.path] | map(select(. != null and . != "")) | first // ""' \
  "${GITHUB_EVENT_PATH}" | tr -d '[:cntrl:]' | cut -c1-200)"

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK:?}"' EXIT

# --- CFR-1: the run acted on is the latest completed scheduled run -----------------------------
gh run list --workflow "${wf_path}" --event schedule --status completed --limit 1 \
  --json databaseId,number,attempt,conclusion >"${WORK}/runs.json" || die "gh run list failed"
need_json "${WORK}/runs.json" 'type == "array"' "gh run list"
if [[ "$(jq 'length' "${WORK}/runs.json")" -eq 0 ]]; then
  echo "ci-failure-report: ${wf_path} has no completed scheduled run yet; nothing to do"
  exit 0
fi
run_id="$(jq -r '.[0].databaseId | tostring' "${WORK}/runs.json")"
run_number="$(jq -r '.[0].number | tostring' "${WORK}/runs.json")"
run_attempt="$(jq -r '.[0].attempt | tostring' "${WORK}/runs.json")"
conclusion="$(jq -r '.[0].conclusion // "" | tostring' "${WORK}/runs.json")"
need_int "run id" "${run_id}" 20
need_int "run number" "${run_number}" 10
need_int "run attempt" "${run_attempt}" 5
run_url="https://github.com/${GITHUB_REPOSITORY}/actions/runs/${run_id}/attempts/${run_attempt}"
echo "ci-failure-report: ${wf_path} run ${run_id} (#${run_number}, attempt ${run_attempt}) concluded '${conclusion}'"

# Class of the run, from its conclusion only (a job failing under continue-on-error does not
# change it). A cancelled run is a failure only if a job started: a non-empty steps array.
case "${conclusion}" in
failure | timed_out | startup_failure) class=failure ;;
success) class=success ;;
cancelled) class=cancelled ;;
action_required | neutral | skipped | stale) class=noop ;;
*) die "unknown run conclusion '${conclusion}'" ;;
esac
if [[ "${class}" == failure || "${class}" == cancelled ]]; then
  gh run view "${run_id}" --attempt "${run_attempt}" --json jobs >"${WORK}/jobs.json" || die "gh run view --json jobs failed"
  if [[ "${class}" == cancelled ]]; then
    started="$(jq '[.jobs[]? | select((.steps // []) | length > 0)] | length' "${WORK}/jobs.json")" ||
      die "cannot read the jobs JSON"
    if [[ "${started}" -gt 0 ]]; then class=failure; else class=noop; fi
  fi
fi
if [[ "${class}" == noop ]]; then
  echo "ci-failure-report: conclusion '${conclusion}' needs no report; nothing to do"
  exit 0
fi

# --- CFR-4: the newest bot issue for this workflow ---------------------------------------------
gh issue list --author 'github-actions[bot]' --state all --limit "${LOOKUP_LIMIT}" \
  --json number,state,author,labels,body >"${WORK}/issues.json" || die "gh issue list failed"
need_json "${WORK}/issues.json" 'type == "array"' "gh issue list"
listed="$(jq 'length' "${WORK}/issues.json")"
[[ "${listed}" -lt "${LOOKUP_LIMIT}" ]] ||
  die "gh issue list returned ${listed} issues, the --limit; the lookup may be incomplete, refusing to write"

# marker($wf): the first-line marker of a body for this workflow, as {n, a, e}, or nothing.
JQ_MARKER='def marker($wf):
  (. // "") | split("\n")[0] | rtrimstr("\r")
  | capture("^<!-- ci-failure-report v1 workflow=(?<wf>\\.github/workflows/[A-Za-z0-9._-]+\\.ya?ml) run_id=[1-9][0-9]{0,19} key=(?<n>[1-9][0-9]{0,9})\\.(?<a>[1-9][0-9]{0,4}) event=(?<e>failure|recovery) -->$")?
  | select(.wf == $wf) | {n: .n, a: .a, e: .e};'
jq --arg wf "${wf_path}" --arg login "${BOT_ISSUE_LOGIN}" --arg label "${LABEL}" "${JQ_MARKER}"'
  [ .[] | select(.author.login == $login and .author.is_bot == true)
        | select(any(.labels[]?; .name == $label))
        | select([.body | marker($wf)] | length == 1) ]' "${WORK}/issues.json" >"${WORK}/mine.json" ||
  die "cannot read the issue list"
# max_by orders strings after numbers, so a non-numeric number is the one picked and refused.
issue_num="$(jq -r 'max_by(.number) | .number // empty | tostring' "${WORK}/mine.json")"
issue_state=""
rec_n=0
rec_a=0
rec_event=""
if [[ -n "${issue_num}" ]]; then
  need_int "issue number" "${issue_num}" 12
  issue_state="$(jq -r --argjson n "${issue_num}" '.[] | select(.number == $n) | .state' "${WORK}/mine.json")"
  rec="$(jq -r --argjson n "${issue_num}" --arg wf "${wf_path}" "${JQ_MARKER}"'
    .[] | select(.number == $n) | .body | marker($wf) | "\(.n) \(.a) \(.e)"' "${WORK}/mine.json")"
  gh issue view "${issue_num}" --json comments >"${WORK}/comments.json" || die "gh issue view --json comments failed"
  need_json "${WORK}/comments.json" 'type == "object" and (.comments | type == "array")' "gh issue view --json comments"
  # Comment authors carry no is_bot: the bot's comments read as login github-actions.
  last="$(jq -r --arg wf "${wf_path}" --arg login "${BOT_COMMENT_LOGIN}" "${JQ_MARKER}"'
    [ .comments[]? | select(.author.login == $login) | .body | marker($wf) ] | last // empty
    | "\(.n) \(.a) \(.e)"' "${WORK}/comments.json")" || die "cannot read the comments of issue ${issue_num}"
  read -r rec_n rec_a rec_event <<<"${last:-${rec}}"
  need_int "recorded run number" "${rec_n}" 10
  need_int "recorded run attempt" "${rec_a}" 5
  echo "ci-failure-report: newest bot issue #${issue_num} is ${issue_state}, recorded key ${rec_n}.${rec_a} (${rec_event})"
fi

# Ordering key (run_number, run_attempt), compared lexicographically: -1 older, 0 equal, 1 newer.
order=1
if [[ -n "${issue_num}" ]]; then
  if ((run_number != rec_n)); then
    if ((run_number > rec_n)); then order=1; else order=-1; fi
  elif ((run_attempt != rec_a)); then
    if ((run_attempt > rec_a)); then order=1; else order=-1; fi
  else
    order=0
  fi
fi

action=none
if [[ "${class}" == failure ]]; then
  if [[ -z "${issue_num}" ]] || [[ "${issue_state}" == CLOSED && "${order}" -gt 0 ]]; then
    action=open
  elif [[ "${issue_state}" == OPEN && "${order}" -gt 0 ]]; then
    action=comment
  fi
elif [[ "${class}" == success && "${issue_state}" == OPEN ]]; then
  if [[ "${order}" -gt 0 ]]; then
    action=recover
  elif [[ "${order}" -eq 0 && "${rec_event}" == recovery ]]; then
    action=close
  fi
fi
echo "ci-failure-report: action ${action}"

key="${run_number}.${run_attempt}"
rc=0
case "${action}" in
open | comment)
  log="${WORK}/failed.log"
  log_args=(--log "${log}")
  if ! gh run view "${run_id}" --attempt "${run_attempt}" --log-failed >"${log}"; then
    echo "ci-failure-report: gh run view --log-failed failed; reporting with logs unavailable" >&2
    log_args=()
    rc=1
  elif [[ ! -s "${log}" ]]; then
    echo "ci-failure-report: the failed-job logs are empty; reporting with logs unavailable" >&2
    rc=1
  fi
  bash "${BUILDER}" --workflow-name "${wf_name}" --workflow-path "${wf_path}" --run-id "${run_id}" \
    --run-url "${run_url}" --key "${key}" --jobs "${WORK}/jobs.json" "${log_args[@]}" >"${WORK}/body.md" ||
    die "the issue body could not be built"
  if [[ "${action}" == open ]]; then
    gh issue create --title "CI failure: ${wf_name}" --label "${LABEL}" --body-file "${WORK}/body.md" ||
      die "gh issue create failed"
  else
    gh issue comment "${issue_num}" --body-file "${WORK}/body.md" || die "gh issue comment failed"
  fi
  ;;
recover)
  printf '%s\n' "<!-- ci-failure-report v1 workflow=${wf_path} run_id=${run_id} key=${key} event=recovery -->" \
    "## Recovered" "" "Run ${run_url} concluded \`success\`; closing." >"${WORK}/body.md"
  gh issue comment "${issue_num}" --body-file "${WORK}/body.md" || die "gh issue comment failed"
  gh issue close "${issue_num}" || die "gh issue close failed"
  ;;
close)
  gh issue close "${issue_num}" || die "gh issue close failed"
  ;;
none) ;;
esac
exit "${rc}"
