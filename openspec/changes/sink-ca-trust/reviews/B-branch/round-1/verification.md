# Stage B round 1 — verification

Branch tip `0a8afca93`, range `317279084..0a8afca93`. Legs, run in parallel and waited on by PID:

- astra adversarial (`codex exec --sandbox read-only - < brief-astra.md`), exit 0, 4 min:
  **BLOCK** (one CRITICAL, one WARNING, one NOTE). It ran the unit suites of the changed
  packages (pass); no Docker, no mutants.
- claude/opus:spec (`fanout.sh --legs "claude/opus:spec" --no-unify --leg-timeout 1800`), exit 0:
  **CONCERNS** (one WARNING, four NOTEs), with an SCT compliance table: SCT-1, SCT-2, SCT-4,
  SCT-5 hold; SCT-3 holds apart from O1; SCT-6 partial (O3).

Holistic run and per-commit gate: `holistic.md` (finding H1).

Every finding below was checked against the code and the library source before a disposition.

| Id | Finding (one line) | Leg, sev | Verified? (evidence) | Disposition |
|---|---|---|---|---|
| A1 | A refused redirect returns `*url.Error` whose text carries the raw `Location` (any query, e.g. presigned `X-Amz-Signature`) into the probe/export error and the status condition; `security-architecture.md:232` says these errors never name a URI | astra CRITICAL | yes: `net/http/client.go:731-738` sets `url.Error.URL = loc` after a `CheckRedirect` error; `refuseInsecureRedirect` (`s3/backend.go:199`) is that callback; smithy keeps the error. Severity lowered to WARNING: the URL comes from the CA-trusted server, the sink's own credentials never sit in a URL (SigV4 headers), and the doc claim is what it breaks | fix 8.2: the guarded client returns a static terminal `ErrCAInsecureRedirect` for a refused redirect; I4 asserts no `Location` text in either error |
| A2 | CLI resolves the credential Secret (`cmd/cli/collect.go:119`) and the database Secret (`wire.go:247`) before the CA rules, the operator checks the CA first (`build_context.go:38`): both sources set plus a missing credential Secret gives `ErrCABothSources` on one path and a lookup error on the other | astra WARNING | yes: read both orders; SCT-5 "both fail with the same shared error", design *CA resolution* "before any Secret is fetched" | fix 8.3: the CLI resolves the CA (shared check, Secret, key) before any credential Secret, as the operator does; parity cases with a missing credential Secret |
| A3 / O1 | MongoDB CA path wraps `opts.Validate()` unchanged (`mongodb/ca_trust.go:80-81`): a bad port or an SRV lookup failure names the host | astra NOTE, opus WARNING | yes: `connstring.go:844-849` (`net.AddrError`), the SRV lookup error names `_mongodb._tcp.<host>`; SCT-3 "the errors this requirement adds SHALL NOT contain the URI, host or credentials", and this wrapper is new | fix 8.4: static, target-free error; class unchanged (not terminal: an SRV lookup can be transient) |
| O2 | `internal/controller/secret_ca.go` is a third resolver with other keys and no shared check; test-only caller | opus NOTE | yes (`grep resolveCAPEM internal/controller`: only `secret_ca_test.go`) | fix 8.6 (deferred item 3): deleted with its test |
| O3 | The v0.22.0 release-note line lives only in `pr-description.md`; git-cliff builds notes from titles | opus NOTE | yes | defer to hand-off: the operator prepends it to the GitHub Release (kollect `AGENTS.md` release section: git-cliff drops bodies, upgrade notes are prepended by hand) |
| O4 | Stronger than spec, in the safe direction (Mongo `tlsAllowInvalid*`, empty `sslmode=`, empty DSN `sslrootcert=`, unparsable S3 endpoint text) | opus NOTE | yes | accepted, no change: each is deliberate and commented (`evidence/G3.md`, `G4.md`) |
| O5 | A CA'd `mongodb+srv` is not tested through the whole `clientOptions`; if the driver left `TLSConfig` nil, `checkDriverTLS` would reject it | opus NOTE (confidence 25) | rejected: the driver sets `SSL = true` for every SRV URI (`connstring.go:998-1000`) and `clientoptions.go:560` builds the TLS config from it | no change |
| H1 | `task test-integration` fails: three subprocess children are expected integration tests that skip in the parent | holistic run | yes (`holistic.md`, evidence-gate output) | fix 8.1: each child runs inside its parent test |

## Items deferred to B

| Item | Finding | Disposition |
|---|---|---|
| D1 | G3 round-2 fix (`caDSN` service presence) never reviewed | reviewed here against pgx v5.11 `pgconn/config.go:449-455` (`if service, present := settings["service"]; present`, merged settings: a DSN key present even empty, else a non-empty `PGSERVICE`) and `parseServiceSettings` (`:861-877`): `caDSN:55-62` and `serviceSettings:128-155` match, including `servicefile` by presence and the `~/.pg_service.conf` default. astra checked the same and confirmed it, with its regression test passing. Holds |
| D2 | Residual chain-only case: `require` and `prefer`+`sslnegotiation=direct` parse to the same shape, so no backstop tells them apart when the reader and pgx disagree | fix 8.5: `caDSN` pins the effective mode it validated in the DSN it hands to pgx (in front, so a later DSN `sslmode` still wins). pgx then uses the reader's mode or a DSN mode the reader missed, which `applyCA` judges; `wantVerifyFull` still catches a missed weaker mode under verify-full. No concrete splitting DSN is known after G3's fixes (`queryIndex` and the keyword reader re-read against `parse_url.go:61-200` and `config.go:768-858`) |
| D3 | `internal/controller/secret_ca.go` third resolver | fix 8.6: deleted (O2) |
| D4 | `git.ValidateTLSSpec` has its own both-sources text | fix 8.7: the check moves to `sinktls.CheckCASources` (`sink.CheckCASources` and `sink.ErrCABothSources` delegate), and git/GitLab call it: same terminal sentinel when `git.NewBackend` is reached with both sources (today only through `Registry.NewBackend` without the resolver) |
| D5 | `task lint` does not lint integration-tagged files; 9 old findings in `export*_integration_test.go` | defer: a repo-wide gate change outside SCT; backlog follow-up for the operator. The branch's own tagged files are linted at the end of stage B with `--build-tags integration` |
| D6 | `task docs:verify` not run locally | attempted at the end of stage B; otherwise CI's Docs workflow on the PR |
| D7 | `Registry.NewBackend` returns a typed-nil `Backend` on error | fix 8.8: `NewBackend` returns a nil interface whenever the factory errs |

## Round 2

astra raised a CRITICAL (A1, accepted as a fix, severity lowered), so round 2 runs on the fixed
tree with fresh legs scoped to "round-1 fixes closed, plus anything new".
