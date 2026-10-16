#!/usr/bin/env bash
# Fake `gh` for hack/test/ci_failure_report_test.sh (openspec change ci-failure-reporting, CFR-1..4,
# CFR-6). The test copies it to <work>/bin/gh and puts that directory first on PATH, so the
# reporter under test never reaches GitHub.
#
# It accepts only the exact argv shapes the reporter is allowed to use, records every call (one
# argument per line, so argument boundaries survive) and serves a small issue store:
#
#   ${FAKE_GH_DIR}/calls/NNNN       one file per call: the arguments, one per line
#   ${FAKE_GH_DIR}/rejected         one line per refused call (argv shape not allowed)
#   ${FAKE_GH_DIR}/runs.json        answer of `gh run list` (a JSON array, test-controlled)
#   ${FAKE_GH_DIR}/jobs.json        answer of `gh run view <id> --attempt <n> --json jobs`
#   ${FAKE_GH_DIR}/log.txt          answer of `gh run view <id> --attempt <n> --log-failed`
#   ${FAKE_GH_DIR}/issues.json      the issue store: [{number, state, title, author, labels, body,
#                                   comments: [{author, body}]}]
#   ${FAKE_GH_DIR}/fail             optional: the kind of call that errors (run-list, jobs, log,
#                                   issue-list, issue-view, create, comment, close); one-shot when
#                                   ${FAKE_GH_DIR}/fail-once exists
#   ${FAKE_GH_DIR}/raw-<kind>       optional, for run-list, jobs, issue-list and issue-view: served
#                                   verbatim (exit 0) instead of the normal answer, so a test can
#                                   hand the reporter empty or malformed output from a gh success
#
# Environment: FAKE_GH_DIR (required), FAKE_GH_REPO (the repository GH_REPO must name),
# FAKE_GH_LOG_RC (exit status of the log fetch, default 0), FAKE_GH_AUTHOR_FILTER=off (serve
# every issue regardless of --author, to prove the reporter's own author check).
set -euo pipefail

D="${FAKE_GH_DIR:?FAKE_GH_DIR is not set}"
mkdir -p "${D}/calls"
n="$(find "${D}/calls" -type f | wc -l)"
call="$(printf '%s/calls/%04d' "${D}" "$((n + 1))")"
printf '%s\n' "$@" >"${call}"

reject() {
  printf '%s | %s\n' "$*" "$(tr '\n' ' ' <"${call}")" >>"${D}/rejected"
  echo "fake gh: refused: $*" >&2
  exit 64
}

# raw <kind>: serve ${D}/raw-<kind> verbatim and exit 0, if it exists.
raw() {
  if [[ -f "${D}/raw-$1" ]]; then
    cat "${D}/raw-$1"
    exit 0
  fi
}

inject() {
  local kind="$1"
  if [[ -f "${D}/fail" && "$(cat "${D}/fail")" == "${kind}" ]]; then
    if [[ -f "${D}/fail-once" ]]; then
      rm -f "${D}/fail" "${D}/fail-once"
    fi
    echo "HTTP 502: injected fake gh error (${kind})" >&2
    exit 1
  fi
}

[[ "${GH_REPO:-}" == "${FAKE_GH_REPO:?FAKE_GH_REPO is not set}" ]] ||
  reject "GH_REPO is '${GH_REPO:-}', expected '${FAKE_GH_REPO}'"
for a in "$@"; do
  case "${a}" in
  --search | --search=* | -S | --body | --body=* | -b | --repo | --repo=* | -R | --reopen) reject "flag ${a} is not allowed" ;;
  esac
done

store="${D}/issues.json"
[[ -f "${store}" ]] || echo '[]' >"${store}"

listed_attempt() { jq -r '.[0].attempt' "${D}/runs.json"; }
listed_id() { jq -r '.[0].databaseId' "${D}/runs.json"; }

case "${1:-} ${2:-}" in
"run list")
  [[ $# -eq 12 && "$3" == --workflow && "$5" == --event && "$6" == schedule && "$7" == --status &&
    "$8" == completed && "$9" == --limit && "${10}" == 1 && "${11}" == --json &&
    "${12}" == databaseId,number,attempt,conclusion ]] || reject "run list argv"
  [[ "$4" =~ ^\.github/workflows/[A-Za-z0-9._-]+\.ya?ml$ ]] || reject "run list --workflow $4"
  inject run-list
  raw run-list
  cat "${D}/runs.json"
  ;;
"run view")
  [[ $# -ge 5 && "$4" == --attempt ]] || reject "run view without --attempt"
  [[ "$3" == "$(listed_id)" ]] || reject "run view of run $3, listed run is $(listed_id)"
  [[ "$5" == "$(listed_attempt)" ]] || reject "run view --attempt $5, listed attempt is $(listed_attempt)"
  if [[ $# -eq 7 && "$6" == --json && "$7" == jobs ]]; then
    inject jobs
    raw jobs
    cat "${D}/jobs.json"
  elif [[ $# -eq 6 && "$6" == --log-failed ]]; then
    inject log
    [[ ! -f "${D}/log.txt" ]] || cat "${D}/log.txt"
    exit "${FAKE_GH_LOG_RC:-0}"
  else
    reject "run view argv"
  fi
  ;;
"issue list")
  [[ $# -eq 10 && "$3" == --author && "$4" == 'github-actions[bot]' && "$5" == --state &&
    "$6" == all && "$7" == --limit && "$9" == --json && "${10}" == number,state,author,labels,body ]] ||
    reject "issue list argv"
  [[ "$8" =~ ^[1-9][0-9]*$ ]] || reject "issue list --limit $8"
  inject issue-list
  raw issue-list
  jq --argjson lim "$8" --arg filter "${FAKE_GH_AUTHOR_FILTER:-on}" '
    [ .[] | select($filter == "off" or (.author.login == "app/github-actions" and .author.is_bot == true)) ]
    | sort_by(-(.number | tonumber? // 0)) | .[0:$lim]
    | map({number, state, author, labels, body})' "${store}"
  ;;
"issue view")
  [[ $# -eq 5 && "$4" == --json && "$5" == comments ]] || reject "issue view argv"
  inject issue-view
  raw issue-view
  jq --arg n "$3" '[ .[] | select((.number | tostring) == $n) ] | if length == 1 then {comments: .[0].comments} else error("no issue \($n)") end' "${store}" ||
    {
      echo "GraphQL: Could not resolve to an issue with the number of $3." >&2
      exit 1
    }
  ;;
"issue create")
  [[ $# -eq 8 && "$3" == --title && "$5" == --label && "$6" == ci-failure && "$7" == --body-file ]] ||
    reject "issue create argv"
  [[ -r "$8" ]] || reject "issue create --body-file $8 is not readable"
  inject create
  num="$(jq '[.[].number | tonumber? // 0] | (max // 99) + 1' "${store}")"
  jq --argjson n "${num}" --arg t "$4" --rawfile b "$8" '. + [{number: $n, state: "OPEN", title: $t,
    author: {is_bot: true, login: "app/github-actions"}, labels: [{name: "ci-failure"}], body: $b, comments: []}]' \
    "${store}" >"${store}.new"
  mv "${store}.new" "${store}"
  echo "https://github.com/${FAKE_GH_REPO}/issues/${num}"
  ;;
"issue comment")
  [[ $# -eq 5 && "$4" == --body-file ]] || reject "issue comment argv"
  [[ -r "$5" ]] || reject "issue comment --body-file $5 is not readable"
  jq -e --arg n "$3" 'any(.[]; (.number | tostring) == $n)' "${store}" >/dev/null || reject "issue comment on unknown issue $3"
  inject comment
  jq --arg n "$3" --rawfile b "$5" 'map(if (.number | tostring) == $n then .comments += [{author: {login: "github-actions"}, body: $b}] else . end)' \
    "${store}" >"${store}.new"
  mv "${store}.new" "${store}"
  echo "https://github.com/${FAKE_GH_REPO}/issues/$3#issuecomment-1"
  ;;
"issue close")
  [[ $# -eq 3 ]] || reject "issue close takes only the issue number"
  jq -e --arg n "$3" 'any(.[]; (.number | tostring) == $n)' "${store}" >/dev/null || reject "issue close on unknown issue $3"
  inject close
  jq --arg n "$3" 'map(if (.number | tostring) == $n then .state = "CLOSED" else . end)' "${store}" >"${store}.new"
  mv "${store}.new" "${store}"
  ;;
*)
  reject "command not allowed"
  ;;
esac
