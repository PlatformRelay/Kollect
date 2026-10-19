## Verdict: CONCERNS

## Findings
- [WARNING] The subprocess "keeps system roots" test assumes `SSL_CERT_FILE` controls the system pool, which is only true on Linux and other Unix systems, and nothing guards it — `internal/sink/sinktls/sinktlstest/sinktlstest.go:131`, `internal/sink/sinktls/appended_pool_test.go:48`
  Failure: on macOS, Go's `SystemCertPool` ignores `SSL_CERT_FILE` and uses the platform verifier. Root P is never trusted there, so on a darwin dev machine `TestAppendedPoolChild_keepsSystemRoots` fails and `task test` goes red. CI on Linux stays green. The 0.1 probe only covered Linux (G1.md:21).
  Fix: in `RunWithSystemRoots`, call `t.Skip` when `runtime.GOOS` is not linux (or not in the set Go honours `SSL_CERT_FILE` for), and say why in the skip message.
  Confidence: 60
- [NOTE] The `AppendedPool` doc comment says "The S3, GCS, Postgres and MongoDB sinks use it", but nothing calls it at HEAD — `internal/sink/sinktls/sinktls.go:89-90`
  Failure: the prose describes groups 2–4, which have not landed. A reader at this commit is told something that is not true yet.
  Fix: reword to "for the S3, GCS, Postgres and MongoDB sinks" (intended users), or leave it and let groups 2–4 make it true before merge.
  Confidence: 85
- [NOTE] The test changes are recorded in the wrong place, and the commit message repeats the wrong place — `openspec/changes/sink-ca-trust/tasks.md:86-87`, HEAD commit body
  Failure: tasks.md 1.1 says to record the *Test changes* in `loop.md`. They are in `evidence/G1.md:218-229`, and `loop.md` gained only the 0.1 row. The commit body says "recorded in the change's loop state", which does not match.
  Fix: add a pointer in `loop.md` to the G1.md section, or reword the commit body to name G1.md.
  Confidence: 80
- [NOTE] A third CA resolver survives in the controller package with different rules — `internal/controller/secret_ca.go:19-59`
  Failure: it reads `ca-bundle.crt` instead of `ca.pem`, lets the inline bundle win, and returns a non-terminal error. It has no production caller today (only `secret_ca_test.go`), so no live path bypasses SCT-4. A future caller would quietly bring back the precedence this change removes. G1.md:233 already flags it.
  Fix: in the branch review, delete it or route it through `sink.CheckCASources` and `sink.SelectCAKey`.
  Confidence: 75
- [NOTE] `git.ValidateTLSSpec` still has its own both-sources check with different, non-terminal text — `internal/sink/git/tls.go:23-25`
  Failure: on the operator and CLI paths, `CheckCASources` now runs first, so this copy only matters if it has other callers. Two messages for one rule can drift.
  Fix: return `sink.ErrCABothSources` here too, or leave it and add a note; it is outside the G1 scope.
  Confidence: 55

What I checked and found correct:
- `CheckCASources` runs before any Secret fetch on both paths (`build_context.go:221`, `wire.go:143`), and `BuildContextFromSpec` resolves the CA before any other Secret (`build_context.go:38`).
- The connection probe (`kollectconnectiontest_controller.go:91`, `family_sink_connection.go:49`) and the export pool (`backend_pool.go:112`, `:145`) all go through `BuildContextFromSpec`. So no operator export, probe or CLI path lets a spec with both sources, or a Secret without a CA key, pass.
- `Terminal` keeps `errors.Is` working through `ClassError.Unwrap` (`internal/errors/errors.go:42`).
- The errors are static or name only the Secret and the keys; no value or PEM is echoed. No key or PEM fixture is committed; certificates are generated in memory.
- R1 covers every row tasks.md 1.1 lists. R2 makes all four named changes: the rename to "both sources rejected", "caSecret no key" expecting the missing-key error, "caSecret ca.pem" added, and `errors.Is` checks. The not-found cases still compare presence only.
- The fixed `FromSpec` comment matches the code (caPEM first, then caBundle as fallback, CA-only pool).
- The 0.1 probe evidence agrees with the design's fallback for the mongo DNS-free parse and with its need to check sources before parsing the DSN.

## Could not check
- I did not run any test, mutant or race run. The mutation script `/tmp/sct-probe/mutate.py` and the probe module are outside the repository, so the six kills and the surviving no-op control are taken from G1.md as recorded, not reproduced.
- I did not read `proposal.md` in full against the 0.1 probe, only the design sections *One pool rule* and *CA resolution, shared*.
- I did not confirm whether `git.ValidateTLSSpec` has callers outside the operator and CLI resolution paths.
- I did not confirm how Go's darwin and windows root loading treats `SSL_CERT_FILE` on this toolchain; the WARNING rests on documented Go behaviour, not a run.
