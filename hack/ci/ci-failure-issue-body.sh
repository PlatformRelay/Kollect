#!/usr/bin/env bash
# CFR-5 (openspec change ci-failure-reporting): build the body of the CI failure issue or comment
# for a failed scheduled run, and print it on stdout.
#
# The body is, in order: the bot marker (CFR-4) as its first line, the workflow and run URL, every
# job whose conclusion is not success, skipped or neutral (a timed-out or cancelled job included)
# with its first such step, then at most 50 log lines containing `FAIL` in one fenced excerpt.
#
# Hardening, because the log and the job list come from the run being reported:
#   * no text from the log or the jobs JSON is evaluated or expanded; it only passes through
#     printf '%s', jq and grep as data;
#   * the excerpt fence is one backtick longer than the longest backtick run inside it (at least
#     three), so no log line can close it;
#   * text outside the fence (workflow name, job and step names) has control characters replaced
#     (no new line or list item), `&` and `<` escaped (no entity, tag or HTML comment) and every
#     `@` followed by an escaped zero-width space (no mention); the marker comes only from validated
#     arguments and is always the first line, so a marker lookalike in a log stays in the fence;
#   * the body is kept under 60000 bytes (lengths are counted in bytes, never fewer than the
#     characters GitHub counts against its 65536 limit): each excerpt line is cut at 4000 bytes,
#     and the excerpt stops at the first line that would not fit, the fence is closed, and a
#     visible truncation note follows it. Jobs are listed before the excerpt, so truncation never
#     drops one; a job list that alone would not fit is an error, never a cut list.
# When --log is absent, empty or unreadable the body says "logs unavailable" and still lists the
# jobs.
#
# Exit codes: 0 body printed; 2 bad arguments or unreadable jobs JSON (nothing printed).
#
# Usage:
#   hack/ci/ci-failure-issue-body.sh --workflow-name <name> --workflow-path <path> \
#     --run-id <id> --run-url <url> --key <run_number>.<run_attempt> --jobs <jobs.json> \
#     [--log <failed.log>]
#   --jobs  output of `gh run view <id> --attempt <n> --json jobs`
#   --log   output of `gh run view <id> --attempt <n> --log-failed`
set -euo pipefail
export LC_ALL=C

CAP=60000
LINE_CAP=4000
MAX_FAIL_LINES=50

die() {
  echo "ci-failure-issue-body: $*" >&2
  exit 2
}

WF_NAME=""
WF_PATH=""
RUN_ID=""
RUN_URL=""
KEY=""
JOBS=""
LOG=""
while [[ $# -gt 0 ]]; do
  [[ $# -ge 2 ]] || die "$1 needs a value"
  case "$1" in
    --workflow-name) WF_NAME="$2" ;;
    --workflow-path) WF_PATH="$2" ;;
    --run-id) RUN_ID="$2" ;;
    --run-url) RUN_URL="$2" ;;
    --key) KEY="$2" ;;
    --jobs) JOBS="$2" ;;
    --log) LOG="$2" ;;
    *) die "unknown argument: $1" ;;
  esac
  shift 2
done

# The marker is built from these four values, so each is held to a shape that cannot end the
# HTML comment, add a field or break the line.
[[ "${WF_PATH}" =~ ^\.github/workflows/[A-Za-z0-9._-]+\.ya?ml$ ]] ||
  die "--workflow-path must be .github/workflows/<file>.y(a)ml, got '${WF_PATH}'"
[[ "${RUN_ID}" =~ ^[1-9][0-9]{0,19}$ ]] || die "--run-id must be a positive integer, got '${RUN_ID}'"
[[ "${KEY}" =~ ^[1-9][0-9]{0,9}\.[1-9][0-9]{0,4}$ ]] ||
  die "--key must be <run_number>.<run_attempt>, got '${KEY}'"
[[ "${RUN_URL}" =~ ^https://github\.com/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+/actions/runs/[0-9]+(/attempts/[0-9]+)?$ ]] ||
  die "--run-url must be a GitHub Actions run URL, got '${RUN_URL}'"
[[ -n "${WF_NAME}" ]] || die "--workflow-name is required"
[[ -n "${JOBS}" && -r "${JOBS}" ]] || die "--jobs must name a readable file"

# safe: text rendered outside a fence. Cut first, then escape, so no entity is cut.
JQ_SAFE='def safe: tostring | gsub("[[:cntrl:]]"; " ") | .[0:200]
  | gsub("&"; "&amp;") | gsub("<"; "&lt;") | gsub("@"; "@&#8203;");'

name_safe="$(jq -rn --arg n "${WF_NAME}" "${JQ_SAFE}"' $n | safe')" || die "cannot escape the workflow name"

# One item per failed job: "- <job> (<conclusion>): failed step <step> (<conclusion>)".
# shellcheck disable=SC2016 # $s and $c are jq variables
jobs_md="$(jq -r "${JQ_SAFE}"'
  def bad: (. // "none") as $c | ($c != "success" and $c != "skipped" and $c != "neutral");
  if (.jobs | type) != "array" then error("no jobs array") else . end
  | .jobs[] | select(.conclusion | bad)
  | ([.steps[]? | select(.conclusion | bad)] | first) as $s
  | "- " + (.name | safe) + " (" + ((.conclusion // "none") | safe) + "): "
    + (if $s == null then "no failed step recorded"
       else "failed step " + ($s.name | safe) + " (" + (($s.conclusion // "none") | safe) + ")" end)
' "${JOBS}")" || die "cannot read the jobs JSON in ${JOBS}"

head_md="<!-- ci-failure-report v1 workflow=${WF_PATH} run_id=${RUN_ID} key=${KEY} event=failure -->
## CI failure: ${name_safe}

- Workflow: ${name_safe} (\`${WF_PATH}\`)
- Run: ${RUN_URL}

### Failed jobs

"
if [[ -n "${jobs_md}" ]]; then
  head_md+="${jobs_md}
"
else
  head_md+="No failed job in this run's job list.
"
fi
head_md+="
### FAIL lines

"

TRUNC_NOTE="_Excerpt truncated: the issue body is kept under 60000 bytes (GitHub rejects bodies over 65536 characters)._"
# Room the excerpt needs at the least: two fences of at most LINE_CAP+1 backticks, one line, the note.
[[ $((${#head_md} + ${#TRUNC_NOTE} + 3 * (LINE_CAP + 16))) -lt "${CAP}" ]] ||
  die "the failed job list alone does not fit the ${CAP}-byte body cap"

UNAVAILABLE="_logs unavailable: the failed-job logs could not be fetched, were empty or could not be read._"
if [[ -z "${LOG}" || ! -s "${LOG}" ]]; then
  printf '%s%s\n' "${head_md}" "${UNAVAILABLE}"
  exit 0
fi

# grep: 0 lines found, 1 none, anything else the log could not be read (logs unavailable).
grep_rc=0
fail_lines="$(grep -a -m "${MAX_FAIL_LINES}" -F 'FAIL' -- "${LOG}")" || grep_rc=$?
if [[ "${grep_rc}" -gt 1 ]]; then
  printf '%s%s\n' "${head_md}" "${UNAVAILABLE}"
  exit 0
fi
lines=()
if [[ -n "${fail_lines}" ]]; then
  mapfile -t lines <<<"${fail_lines}"
fi
if [[ "${#lines[@]}" -eq 0 ]]; then
  printf '%s%s\n' "${head_md}" "_No line containing FAIL in the failed-job logs._"
  exit 0
fi

# Strip a trailing CR and cut over-long lines (dropping a partial UTF-8 sequence at the cut).
for i in "${!lines[@]}"; do
  l="${lines[i]%$'\r'}"
  if [[ "${#l}" -gt "${LINE_CAP}" ]]; then
    l="$(printf '%s' "${l:0:LINE_CAP}" | sed 's/[\xC0-\xFF][\x80-\xBF]*$//') [line cut]"
  fi
  lines[i]="${l}"
done

# Fence: one backtick longer than the longest run in the excerpt, at least three.
longest="$(printf '%s\n' "${lines[@]}" |
  awk '{ while (match($0, /`+/)) { if (RLENGTH > m) m = RLENGTH; $0 = substr($0, RSTART + RLENGTH) } } END { print m + 0 }')"
fence_len=$((longest + 1))
[[ "${fence_len}" -ge 3 ]] || fence_len=3
fence="$(printf '%*s' "${fence_len}" '' | tr ' ' '`')"

# Total = head + fence\n + excerpt + fence\n + \n note \n, kept at most CAP-1.
budget=$((CAP - 1 - ${#head_md} - 2 * (${#fence} + 1) - ${#TRUNC_NOTE} - 2))
excerpt=""
truncated=0
for l in "${lines[@]}"; do
  if [[ $((${#excerpt} + ${#l} + 1)) -gt "${budget}" ]]; then
    truncated=1
    break
  fi
  excerpt+="${l}
"
done

printf '%s%s\n%s%s\n' "${head_md}" "${fence}" "${excerpt}" "${fence}"
if [[ "${truncated}" -eq 1 ]]; then
  printf '\n%s\n' "${TRUNC_NOTE}"
fi
