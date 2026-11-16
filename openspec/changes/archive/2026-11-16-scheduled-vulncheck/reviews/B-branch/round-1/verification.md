# B round 1 — verification

Branch tip: G1 `0b2d1f898` plus the SVS-1.1 example fix. reviewer-a BLOCK, reviewer-b CONCERNS.

- CRITICAL (step name): `hack/ci/ci-failure-report.sh:199` fetches `gh run view --log-failed`; its lines are
  `<job>\t<step>\t<timestamp> <text>` (seen in this repository's own runs, e.g. the 2026-11-12 vulncheck log), and
  `hack/ci/ci-failure-issue-body.sh:132` keeps the first 50 lines holding `FAIL` — confirmed: a step named with
  `FAIL` floods the excerpt.
- Capture failure, timing wording, wrapped titles, key set, `github[` token form, proposal status wording, G1 path —
  confirmed by reading the cited lines.

All findings fixed in the spec set or routed to task B1.1; triage in `loop.md` (B1 rows).
