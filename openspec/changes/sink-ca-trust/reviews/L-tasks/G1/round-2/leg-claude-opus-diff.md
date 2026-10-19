## Verdict: CONCERNS

## Findings
- [WARNING] The platform allow-list in the new skip leaves out two GOOS values where Go does read `SSL_CERT_FILE`, so P2 is skipped there when it could have run — `internal/sink/sinktls/sinktlstest/sinktlstest.go:120-124`
  Failure: `runtime.GOOS` is `illumos` on illumos, but illumos satisfies the `solaris` build tag. It is `android` on Android, which satisfies the `linux` tag. Both therefore build `crypto/x509/root_unix.go` and honour `SSL_CERT_FILE`, yet the switch matches neither literal GOOS, so `TestAppendedPool_keepsSystemRoots` skips on both. The effect is small (neither runs CI or a dev box here), but the evidence says the list is "a Go platform fact (`root_unix.go` build tag)", and it is not: a GOOS literal list does not match build-tag semantics.
  Fix: invert the switch to skip only where Go uses the platform verifier (`case "darwin", "ios", "windows": t.Skipf(...)`), or add `"illumos", "android"` to the list.
  Confidence: 70 (from what I know of Go's build tags; I could not read GOROOT, see below)
- [NOTE] Mutant 7 survives by design, so nothing stops someone from removing `linux` from the list, and a future edit could silently turn P2 into a skip on CI — `openspec/changes/sink-ca-trust/evidence/G1.md:216`
  Failure: if `linux` is dropped or misspelt, `go test` still exits 0 with P2 skipped, and every gate stays green.
  Fix (smallest ratchet): a test or guard that fails if `RunWithSystemRoots` skips when `GOOS=linux`. For example, put the switch in a helper `honoursSSLCertFile(goos string) bool` and add a one-line test asserting `honoursSSLCertFile("linux")`.
  Confidence: 85
- [NOTE] The skip is placed correctly: `RunWithSystemRoots` skips after the caller's `NewCA` setup and before the re-exec. The child test keeps its own `InChild` skip, so the child path is unaffected — `internal/sink/sinktls/appended_pool_test.go:49-63`. No defect.
- [NOTE] The other round-1 "fixed" claims hold:
  - The `AppendedPool` comment no longer claims callers that don't exist yet (`sinktls.go:89-90`).
  - `loop.md` rows 75-76 drop "planned" and record both changed tests. The test names they cite exist (`build_context_test.go:344`, `:450`).
  - The 0.1 evidence now names the `dns.DefaultResolver` `LookupSRV`/`LookupTXT` swap with `t.Cleanup` and no parallel run (`G1.md:96-101`). That is a sound way to prove no lookup ran.
  - The commit body for `1285a89ef` already pointed at "the change's loop state", and that now resolves.
- [NOTE] The deferrals look right to me:
  - `internal/controller/secret_ca.go` has only a test-only caller.
  - `git.ValidateTLSSpec` has its own wording for the both-sources error, but the shared check runs first on every resolver path.
  Both are low-risk to leave for the branch review. The "accepted as is" on row 10 rests on mutant 6 being killed, which the mutation table records (`G1.md:212`).

## Could not check
- `$(go env GOROOT)/src/crypto/x509/root_unix.go`: it is outside the repository, so I could not confirm its build tag (the illumos/android claim comes from what I know of Go's build tags).
- I ran no tests or mutants. The results for mutants 0, 3 and 7 are taken from `G1.md:216-219` as recorded.
- I did not open the round-1 leg files beyond the grep hit, nor the SCT-4/SCT-5 spec text. I judged the fixes against the dispositions table and the diff.
