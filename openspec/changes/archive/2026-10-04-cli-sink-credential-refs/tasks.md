# Tasks

## 1. Boundary tests first (CSC-1..CSC-6)

- [x] 1.1 Write `internal/pipeline/sink_credentials_test.go`: config directory → `LoadConfig` → `ResolveSink` → `ResolveSinkSecretData` → `cliBuildContext` → `withCLIBackend` with a git factory that records its input and builds the real git backend; watch each test fail on its assertion
- [x] 1.2 Write the parity test against the operator's `sink.BuildContextFromSpec` (fake client, same Secrets) over the reference combinations; watch the defect cases fail

## 2. Fix (CSC-1..CSC-4)

- [x] 2.1 `ResolveSinkSecretData`: `spec.secretRef`, then `git.auth.secretRef` for git sinks; every set reference must resolve
- [x] 2.2 `cliBuildContext`: CA from `tls.caBundle`, else `tls.caSecretRef` (keys `tls.crt`, `ca.crt`, `ca.pem`)
- [x] 2.3 One config-dir lookup (`resolveConfigSecret`) for all references, keeping today's matching and `${env:VAR}` substitution
- [x] 2.4 Defect controls: break each rule in turn and confirm a named test fails

## 3. Docs and review

- [x] 3.1 `docs/guides/pipeline-cli.md` names the resolved references and the failure on a missing Secret
- [x] 3.2 CI green on the PR head
- [x] 3.3 Independent review

## Verification

Revision: the PR head named in the review comment. Go 1.26.6, umask 022.

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CSC-1 | `TestCLICredentials_gitAuthSecretRefOnly`, `_gitAuthOverridesDefaultSecret`; parity cases `gitAuth`, `both`, `gitlab ignores gitAuth` | backend gets the git.auth data, never the default secret's | pass | red before the fix: "backend received token "default-token", want the git.auth.secretRef one" |
| CSC-2 | `TestCLICredentials_caSecretReachesGitBackend`; parity cases `caSecret *`, `inline bundle wins`, `s3 caSecret` | the real git backend's `Config().CABundle` is the PEM and `RootCAs` is set | pass | red before the fix: "the git factory did not receive the tls.caSecretRef CA" |
| CSC-3 | `TestCLICredentials_missingReferencesFailBeforeTheBackend` (CA, git auth); parity cases `* absent` | error names the Secret; the factory is never called | pass | red before the fix: the missing reference was accepted |
| CSC-4 | `TestCLICredentials_envPlaceholderThroughGitAuth` | backend gets the environment value | pass | red before the fix: empty token |
| CSC-5 | `TestCLICredentials_matchOperatorResolver`, 14 cases | same data and CA as the operator, or both fail | pass | red before the fix: 9 of 14 cases differed |
| CSC-6 | `TestWithCLIBackend_closesAfterFailedExport` | backend closed after a failed export | pass | already true; this locks it (no red claimed) |
| all | defect controls: 10 mutations of the fix (override dropped, override for any type, default wins, default not required, missing reference tolerated, CA not passed, secret before inline bundle, CA key order, missing CA key errors, no env substitution) | each turns a named test red | pass | 10/10 killed by exit status; a no-op control survives |
| all | `go test -race` on `internal/pipeline/...` and `cmd/cli/...`; pinned golangci-lint v2.11.4 | pass, 0 issues | pass | local and reviewer |
| — | `go test -race ./internal/sink/` | pass | fail (pre-existing) | Not touched by this change (identical to `main`). A data race between `ResetBreakersForTest` and `exportThroughBreaker` fails 4 of 4 local runs and 4 of 5 of the reviewer's runs on `main`. An earlier pass recorded here was a lucky run. Filed as SINK-BREAKER-RACE-01 |
| all | CI on the PR head | required checks green | pass | d780bc88f: 40 pass; the final head is re-checked before merge |
| — | independent review | APPROVE | pass | d780bc88f: APPROVE, no P0/P1. Applied: CSC-5 wording scoped to what parity tests (F2/F3), the CLI guide names all three references and errors (F5), the test helper no longer prints values (F7). Filed: early CA check (F1), sink race (F4), release note (F6). The first reviewer's hand-back was lost; this is the second, independent review |
