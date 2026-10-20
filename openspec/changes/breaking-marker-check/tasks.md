# Tasks

## 1. Test first (BMC-1 to BMC-5)

- [ ] 1.1 Write `hack/test/breaking_marker_test.sh` and its fixtures under
  `hack/test/testdata/breaking-marker/`; run it with no script present and record the red
- [ ] 1.2 Add the guard to `cfc_guards`; run `hack/test/ci_workflow_security_test.sh` before the
  `lint` steps exist and record the red

## 2. Implement (BMC-1 to BMC-4)

- [ ] 2.1 `hack/check-breaking-marker.sh`
- [ ] 2.2 `.github/workflows/breaking-marker.yaml`, job `breaking-marker`
- [ ] 2.3 Wire the guard into the `lint` job, plain and `--self-test`

## 3. Docs

- [ ] 3.1 `CONTRIBUTING.md`: the marker goes on a commit title

## 4. Proof and land

- [ ] 4.1 Run the script against the #478 commit range on `main`
- [ ] 4.2 Independent review (two legs); PR with every check green on the head SHA, the new job
  among them

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| BMC-1 | fixture cases `title-bang-plain-commits`, `body-footer-plain-commits`, `pr478`, `title-and-commit-marked`, `mentions-only` | fail, fail, fail, pass, pass | not-run | |
| BMC-2 | fixture case `commit-marked-title-plain` | `::warning::`, exit 0 | not-run | |
| BMC-3 | unknown revision, missing `PR_TITLE`, wrong argument count, empty range | exit 2, 2, 2, 0 | not-run | |
| BMC-4 | static workflow checks in the guard | pass on the tree; each injected defect reds | not-run | |
| BMC-5 | `--self-test` mutants and no-op; CWS-6/CFC-6 with the steps removed | each mutant reds its own case; unwired reds | not-run | |
| all | `breaking-marker` job on this PR; independent review; CI on the PR head | success, APPROVE, green | not-run | |
