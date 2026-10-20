# Tasks

## 1. Test first (BMC-1 to BMC-5)

- [x] 1.1 Write `hack/test/breaking_marker_test.sh` and its fixtures under
  `hack/test/testdata/breaking-marker/`; run it with no script present and record the red
- [x] 1.2 Add the guard to `cfc_guards`; run `hack/test/ci_workflow_security_test.sh` before the
  `lint` steps exist and record the red

## 2. Implement (BMC-1 to BMC-4)

- [x] 2.1 `hack/check-breaking-marker.sh`
- [x] 2.2 `.github/workflows/breaking-marker.yaml`, job `breaking-marker`
- [x] 2.3 Wire the guard into the `lint` job, plain and `--self-test`

## 3. Docs

- [x] 3.1 `CONTRIBUTING.md`: the marker goes on a commit title

## 4. Proof and land

- [x] 4.1 Run the script against the #478 commit range on `main`
- [x] 4.2 Independent review (two legs, two rounds) — evidence: evidence/T1.md (Review)
- [x] 4.3 PR with every check green on the head SHA, the new job among them; archive in a
  follow-up PR — evidence: evidence/T1.md (CI)

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| BMC-1 | fixture cases `title-bang-plain-commits`, `body-footer-plain-commits`, `body-footer-hyphen`, `body-footer-crlf`, `pr478`, `merge-commit-marked`, `cliff-unparseable`, `fence-mixed`, `title-loose-gitmoji`, `title-and-commit-marked`, `scopeless-marked`, `mentions-only`, `no-markers` | fail ×9, pass ×4 | pass | evidence/T1.md (Green, Real pull requests) |
| BMC-2 | fixture cases `commit-marked-title-plain`, `body-footer-commit-marked`, `commit-cr-subject`, `warn-not-when-title-marked` | `::warning`, `%`/CR encoded, exit 0; no warning under a marked title | pass | evidence/T1.md (Green) |
| BMC-3 | unknown revision, missing `PR_TITLE`, wrong argument count, empty range | exit 2, 2, 2, 0 | pass | evidence/T1.md (Green) |
| BMC-4 | static workflow checks in the guard | pass on the tree; each injected defect reds | pass | evidence/T1.md (Green): 18 workflow mutants; zizmor clean |
| BMC-5 | `--self-test` mutants and no-op; CWS-6/CFC-6 with the steps removed | each mutant reds its own case; unwired reds | pass | evidence/T1.md (Red, Green) |
| all | independent review | APPROVE after fixes | pass | evidence/T1.md (Review) |
| all | `breaking-marker` job on this PR; CI on the PR head | success, green | pass | evidence/T1.md (CI): PR #485 head `c29019dcc`, every check green, `breaking-marker` success |
