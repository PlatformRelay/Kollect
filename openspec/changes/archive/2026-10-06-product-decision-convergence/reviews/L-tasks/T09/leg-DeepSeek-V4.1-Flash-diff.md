I'll start by reading the task evidence and the diff.
## Verdict: CLEAN

The change does what its task claims: engine axis removed (`types.go`, `config.go`, `export.go:125`, `delete.go:107`), `cfgNeedsCLISSH` gone with its dead engine branch, KEX list is 8-existing-in-order + the two appended, and both CRD copies + golden + chart README are consistent. I verified the KEX names are genuinely implemented by the pinned `golang.org/x/crypto v0.57.0` (`ssh/kex.go`, `ssh/mlkem.go`) — the offer is not broken. I confirmed `validateGitSpec` is actually wired into admission (`family_sink.go:54` → webhook `family_sink_webhook.go:60`) and that no mutating webhook or controller rewrites the spec, so the "not deleted or rewritten" scenario holds. I re-ran `go vet -tags integration ./internal/sink/git` (exit 0) since a test helper (`remoteLogSubjects`) was deleted; no dangling refs. I confirmed file:// always reaches the CLI path and always gets a fresh temp workdir (`mirror.go:209-216`), so the 9 deleted CLI×persistent-mirror locks pin a genuinely unreachable surface.

## Findings
- [NOTE] Residual production comments still name the removed selectable "CLI engine" — `internal/sink/git/prune_tree.go:96`, `internal/sink/git/prune_target.go:147`, `internal/sink/cleanup.go:294`
  Failure: a future reader concludes a `cli` engine value still exists; the task's "every site truthed up" claim is docs-only, these are not in the enumerated list.
  Fix: `s/CLI engine/CLI machinery/` in the three comments.
  Confidence: 92
- [NOTE] The CLI persistent-mirror branches are now unreachable for the only remotes that reach the CLI path — `realignDivergedDirectBranch` (`delete.go:323`), `cliStrandedDeliveryDue` (`delete.go:350`), `pushBranchWithoutWork` (`delete.go:405`)
  Failure: unexercised branches rot silently; a later edit assumes they run in production. ADR-0803 §Consequences explicitly parks the hoist, so this is debt, not a defect.
  Fix: the Sweep-2 hoist the ADR promises; until then a one-line `// unreachable for file:// remotes` marker would stop the drift.
  Confidence: 70 (acknowledged, so not blocking)
- [NOTE] Narrowing the CRD enum to `[go-git]` means *any* update to a stored `engine: cli` object fails API-server schema validation, not only at backend construction — `config/crd/bases/kollect.dev_kollectsnapshotsinks.yaml:143`
  Failure: the upgrade note (`upgrading.md:241`) audits and describes construction-time rejection but not that an unrelated edit to such an object is now rejected by the schema; an operator could be surprised mid-upgrade.
  Fix: add one sentence to the warning box naming schema rejection on update.
  Confidence: 78
- [NOTE] `TestApplyGitSpec_engineAcceptsGoGitAndRejectsOthers` (`config_test.go:169`) duplicates the rejection assertion of T04's `TestConfigFromSpec_rejectsCLIEngineNamingGoGit`; only the "arbitrary invalid value" case is new.
  Failure: none functional; slightly redundant coverage.
  Fix: keep — the invalid-value case is a legitimate widening.
  Confidence: 60

## Could not check
- `task test-integration` (no Docker): neither the 9 deleted nor the 6 surviving integration locks were executed; the vacuity argument for deletion is read, not run.
- Coverage gate / baseline: did not run `task coverage` or compare changed-lines coverage; deleting tests could move a baseline I did not read.
- `docs_launch_truth` and `task spec:validate`: pinned by the task, not run.
- Pipeline-CLI runtime path: I read `Dockerfile.pipeline` and the `ConfigFromSpec` callers but did not run the pipeline binary against a git sink.
