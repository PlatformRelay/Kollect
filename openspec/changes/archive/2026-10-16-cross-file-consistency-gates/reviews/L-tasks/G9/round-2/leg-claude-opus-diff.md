## Verdict: CONCERNS

## Findings
- [WARNING] Nothing in CWS-7 covers the new CFC-6 list entry, so reverting 86081a23f still passes `ci_workflow_security_test.sh --self-test` — `hack/test/ci_workflow_security_test.sh:599-600`, `:1087-1102`
  Failure: someone drops `openspec_evidence_markdown_only_test.sh` from `cfc_guards` and deletes both of its `ci.yaml` steps. Plain CWS goes green, because the generic CWS-6 loop only sees guards CI still runs. `--self-test` goes green too, because the only CFC-6 unwiring mutants target `consistency_metrics_test.sh`, which comes from the glob. G9 row 12 proved the gap closed once, by hand, and no mutant keeps it closed. `go_patch_lag_test.sh` has the same hole. The commit title says it "require[s] the evidence guard in both lint modes", but the self-test does not hold that requirement in place.
  Fix: add one `mutant_rejected "CFC-6 evidence guard unwired"` that `yq`-deletes both `openspec_evidence_markdown_only_test` steps and expects `…openspec_evidence_markdown_only_test.sh is not run`.
  Confidence: 80
- [NOTE] The CWS-6 header and the CFC-6 failure message don't match the widened list — `hack/test/ci_workflow_security_test.sh:27-29`, `:607`
  Failure: the header still lists only `consistency_*` and `go_patch_lag_test.sh`. When the evidence guard is unwired, the message calls it a "cross-file consistency guard", so the reader is pointed at the wrong family of guards.
  Fix: name the evidence guard in the header, and make the message say "CFC-6 guard".
  Confidence: 85
- [NOTE] The "Probe sources" re-run instructions tell the reader to put the file back at `evidence/<name>.py|.sh`, which is the path the guard forbids — `openspec/changes/cross-file-consistency-gates/evidence/G3.md:238,243,311` (same in G5.md and G8.md)
  Failure: the reader recreates `evidence/G3-rbac-probe.py`, and a later `git add -A` commits it. The guard would red in lint, so CI stays safe, but the docs steer people into the failure. Line 238 also says "save the block to a scratch file", which conflicts with the Run-as line. The new Run-as wording is otherwise correct: the mutant scripts `cd "$(dirname "$0")/../../../.."`, which lands at the repo root from `evidence/`.
  Fix: say "with the file saved as `evidence/<name>` (untracked; do not commit it)", or rewrite the commands for a scratch path.
  Confidence: 60
- [NOTE] G9.md, which will be committed, records an absolute host path, `/home/koni/Projects/PlatformRelay/worktrees/...` — `openspec/changes/cross-file-consistency-gates/evidence/G9.md:84`
  Failure: a local username and workspace layout land in the product repo.
  Fix: write it as `bash /tmp/g9/gate-mutants.sh <repo>`.
  Confidence: 70
- [NOTE] The self-test depends on the host environment in two places — `hack/test/openspec_evidence_markdown_only_test.sh:101,208-210,213`
  Failure: (a) `git add -A` in the copy honours the user's global `core.excludesFile`. A global `*.log` or `*.txt` ignore makes the `legs.log` control or the `b.txt` mutant fail with "did not land". It fails loudly, but on a clean tree. (b) If `TMPDIR` is inside a git work tree, the "outside any git work tree" root check reds with the wrong message. CI is unaffected.
  Fix: run git with `-c core.excludesFile=/dev/null` in the copy, and use `mktemp -d -p /tmp` for PLAIN, or accept the dependency and document it.
  Confidence: 55
- [NOTE] No recorded sensor covers the final working tree: `task lint:markdown` ran on `4a367e2c0` only, and the Run-as edits plus the new G9.md have not been linted — `evidence/G9.md:69`
  Fix: run `task lint:markdown`, plus the guard in plain mode (38 files expected), before the close commit.
  Confidence: 75

What else I checked, with no defect found:
- **Guard logic:** the `:(glob)openspec/changes/**/evidence/**` pathspec reaches live, archived and nested evidence. The check reads only tracked files, never runs on an empty scan, and rejects any root that is not the top of a work tree, including when the path goes through a symlink (`pwd -P` on both sides).
- **Self-test:** each mutant has a probe proving it landed and must red with its own message. The control and the no-op copy pass.
- **53eec1806:** with `pipefail`, a tar failure now reaches the named `fail`.
- **CI wiring:** the lint job is unconditional (no `changes.code` gate, `pull_request` with no `paths-ignore`), so a PR that only touches openspec still runs the guard in both modes.
- **CWS-6:** it matches the evidence guard's absolute path the same way it does for the other guards.

## Could not check
- I ran no sensors (read-only leg). The guard, its self-test, the 9 hand mutants and the six `ci.yaml` guards are taken from G9.md's records.
- I did not compare the five inlined blocks byte for byte against the removed files (matrix row 1). I only read G3.md's blocks against the Run-as wording, not the G5.md and G8.md bodies.
- I did not read the round-1 leg reports under `reviews/L-tasks/G9/round-1/` or the untracked `legs.log` files.
- I did not confirm the SonarCloud result on the PR head.
