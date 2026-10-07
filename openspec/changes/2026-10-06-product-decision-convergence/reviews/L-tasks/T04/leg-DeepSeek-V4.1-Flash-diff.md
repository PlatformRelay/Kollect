## Verdict: CONCERNS

## Findings

- [WARNING] The KEX relative-order guard does not enforce the spec's normative "keeps the existing algorithms first", only that the eight form an in-order subsequence — `internal/sink/git/kex_convergence_test.go:80-89` vs `openspec/changes/2026-10-06-product-decision-convergence/specs/git-engine/spec.md:43`
  Failure: T09 prepends `mlkem768x25519-sha256`/`diffie-hellman-group16-sha512` (or interleaves them) → the offer demotes existing preferences, the spec sentence is violated, yet every T04 test stays green (`positions` only checks strictly-increasing indices).
  Fix: assert the eight existing names occupy the offer's leading positions (e.g. `positions[existing[i]] == i`, or that no new name precedes the last existing one). The evidence deliberately chose subsequence, but the task text ("current relative order") and the requirement sentence diverge — the requirement is the stronger contract.
  Confidence: 85

- [NOTE] The file:// fixtures are labelled "engine-less" but call `.withDefaults()`, which assigns `Engine = GitEngineGoGit` — `internal/sink/git/file_remote_convergence_test.go:20,25` / `internal/sink/git/config.go:257-259`
  Failure: no runtime failure at HEAD; the comment and evidence ("every new test constructs configs without the Engine field") describe source literals, not the effective config, so a reviewer may wrongly conclude the routing tests exercise an unset engine.
  Fix: reword the headers/evidence to "default engine", or assert the field is empty before `withDefaults()`.
  Confidence: 95

- [NOTE] The probe test's skip guard uses ambient `exec.LookPath("git")`, while production `lsRemote` resolves against the pinned `pinnedGitPATH = "/usr/bin:/bin:/usr/local/bin"` — `internal/sink/git/file_remote_convergence_test.go:64` / `internal/sink/git/export_file.go:285,292`
  Failure: git present only on ambient PATH outside the pinned dirs (e.g. Homebrew `/opt/homebrew/bin`) → `lsRemote` soft-skips and returns nil, so the "non-repo path fails" subtest hits `err == nil` and fails spuriously. Matches the package's existing convention, so pre-existing, not newly introduced.
  Fix: gate the test on the same resolver (or a direct stat of the pinned dirs) rather than ambient `LookPath`.
  Confidence: 55

- [NOTE] `tasks.md` T04 is still unticked and is absent from the diff, though the evidence lists the "tasks.md tick" in scope — `openspec/changes/2026-10-06-product-decision-convergence/tasks.md:33`
  Failure: none functional; a closure artefact the evidence claims is missing from the committed tree (verdict is still `<pending>`, so likely owed at close).
  Fix: tick T04 when the review verdict lands.
  Confidence: 80

Verified by execution (HEAD `4161a55d`): the four reds fail for their stated reasons — validation `0 errors`, `ConfigFromSpec` returns nil, CRD enum `[go-git cli]` in both copies, KEX missing `mlkem768x25519-sha256`; the guards (`acceptsGoGitAndDefault`, `existingEightKeepRelativeOrder`) and the file:// export/delete/probe regressions all pass. No production file is touched; arch-lint excludes `test/**` and adds no new dependency. Both new KEX algorithms are in `supportedKexAlgos` for x/crypto v0.57.0, so the spec's "already implemented" claim holds.

## Could not check

- The full 47-package suite, `-race`, `task lint`, `task verify`, integration/helm gates: ran only the five new tests' targeted commands.
- T09's future diff: guard behaviour under the API change is reasoned, not executed.
- `loop.md`, design D5, the untracked `reviews/L-tasks/T04/` directory, and the remainder of `tasks.md`.
