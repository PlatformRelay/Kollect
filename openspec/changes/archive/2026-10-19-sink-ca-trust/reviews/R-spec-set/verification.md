# R spec-set review, round 1 — verification

Revision reviewed: `b3da7b2c9` (spec set `70c847a8e` plus loop files). Legs: astra spec lens
(`leg-astra-spec.md`, brief `brief-astra.md`) → BLOCK; claude/opus spec lens
(`leg-claude-opus-spec.md`, via `fanout.sh --legs "claude/opus:spec" --no-unify`) → CONCERNS.
Every finding was checked against the tree or the module cache on 2026-10-19 before acting.

| ID | Sev (leg → verified) | Finding | Check | Disposition |
| --- | --- | --- | --- | --- |
| A-F1 | CRITICAL → CRITICAL | MongoDB `sslInsecure` alias bypasses the `tlsInsecure` rejection | `connstring.go:614` `case "sslinsecure", "tlsinsecure"`; keys lowercased at `:348` | fix: SCT-3 rejects both aliases, case-insensitive, plus a post-`ApplyURI` backstop; M2, M4, M7, mutation row |
| A-F2 | CRITICAL → WARNING | `s3://` lets the AWS SDK pick an `http` endpoint from env/shared config; no redirect policy | `env_config.go:76` `AWS_ENDPOINT_URL`; `s3/backend.go:142` sets `BaseEndpoint` only for a spec endpoint | fix: SCT-1 checks the effective endpoint and refuses HTTPS→HTTP redirects; S1b, I4. Not CRITICAL: needs control of the process environment |
| A-F3 | CRITICAL → CRITICAL | The CLI loads only `KollectSnapshotSink`, so the Postgres CLI scenario cannot run | `internal/pipeline/loader.go:41`, `:122-135` | fix: SCT-5 CLI scope is `s3`/`gcs` end to end, Postgres/MongoDB parity at the constructors; C1 rewritten as a `cmd/cli` exit-code test; non-goal added |
| A-F4 | CRITICAL → WARNING | MongoDB connect/ping errors wrap raw driver text | `mongodb/backend.go:97-105` `%w` | fix (narrow): the spec now claims redaction only for errors this change adds; the existing driver-error leak is a non-goal and follow-up. Not CRITICAL: not introduced by this change |
| A-F5 | WARNING | `sslnegotiation=direct` rewrites `prefer` to `require`; ruling interpretation unstated | pgx `config.go:903-908` | fix: effective sslmode defined after env defaults and the rewrite; interpretation recorded in proposal; D3, D5 cases |
| A-F6 / C-W4 | WARNING | `PGSSLROOTCERT` detected only after pgx reads it; home default `~/.postgresql/root.crt` missing | pgx `defaults.go:58`; `config.go:910-937` reads the file before returning | fix: conflict check before parsing, names its source; backstop for service files; D4 variants |
| A-F7 / C-W1 / C-W2 / C-N2 | WARNING | Both-set check after the fetch; R2 exact text breaks not-found cases; CLI wraps errors | `sink_credentials_test.go:346-358`, `:384`; `wire.go:137`, `:496`, `:506` | fix: spec check before any fetch; parity defined at the shared functions with `errors.Is`; not-found cases keep presence checks |
| A-F8 | WARNING | No successful `require`/`verify-ca`, no hostname distinction, PI4 cannot call `acquireBackend` from package `postgres`, no SRV-accepted case | `backend_pool.go:94` unexported | fix: PI2 widened, PI5 added, PI4 moved to `package sink`, M8 added |
| A-F9 | WARNING | `SSL_CERT_DIR=` (empty) keeps Go's default dirs; Go is 1.27.2; evidence runner hides `=== RUN` | `go.mod:3`; `integrationevidence/main.go:30` `-json-out` | fix: empty temp dir, anchored child run, `-v` or `-json-out` evidence rule |
| A-F10 | WARNING | Compatibility table and release note overgeneralise the shared-resolver change | `sinktls.go:54-70` Kafka/NATS plaintext without material | fix: previous behaviour per backend; release note names all-type scope; CRD comment qualified; SCT-6 text |
| C-W3 | WARNING | MongoDB key check has no backstop after parsing | design had none | fix: with A-F1 |
| C-N1 | NOTE | Validation placement split between `ConfigFromSpec` and client builders | `design.md` *Signatures* vs *Postgres* | fix: `ConfigFromSpec` gains `caPEM`; parsed-config rules in the shared client builder |
| C-N3 | NOTE | "No CRD schema change" vs regenerated descriptions | `tasks.md` 6.1 | fix: "no CRD schema change beyond field descriptions" |
| C-N4 | NOTE | "Go 1.26"; R2 "add" an existing case | `go.mod:3`; `sink_credentials_test.go:354` | fix |
| A-stale | — | `wire.go:170`, `backend_pool.go:263`, `connstring.go:995-1002`, pgx fallback lines | read | fix: `:169`; `:145` read and `:263` spec-only hash; `:994`/`:1016`; `:542-607`, `:1063-1071` added |

Accepted CRITICALs: A-F1, A-F3. Per the loop rule, one more round with fresh legs follows on the
fixed spec set (`round-2/`).
