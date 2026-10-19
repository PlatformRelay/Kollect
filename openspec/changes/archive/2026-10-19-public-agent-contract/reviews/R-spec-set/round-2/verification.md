# Stage R round 2 (last) — verification

Reviewed revision: `4685ebd33`. Legs: astra spec lens (`leg-astra-spec.md`, verdict CONCERNS),
claude/opus:spec (`leg-claude-opus-spec.md`, verdict CONCERNS). No CRITICAL. Both legs confirm
the round-1 fixes (index scanning, `--no-index`, migration ordering, wiring, line numbers). Each
finding was checked by the orchestrator on 2026-10-19; verified mechanical fixes are in the
commit that carries this file and get no further review (last round).

| Id | Sev (leg) | Verified? | Disposition | Where / why |
|---|---|---|---|---|
| A2-1 / O2-8 | WARNING / NOTE | yes | fix | PAC-6 running-host check also skips a name that is a whole word in tracked Markdown (an ordinary word) and documents `AGENT_CONTRACT_HOSTNAME=` for contributors; the maintainer's host name occurs in no tracked Markdown today (checked 2026-10-19, count only), so the check stays active there; 1.1 evidence says whether it was active, without naming the host |
| A2-2 | WARNING | yes | fix | PAC-5: `AGENT_CONTRACT_NESTED=1` skips every mutant that starts a nested self-test (empty-pattern and fixed-string); the fixed-string mutant must fail with the regex-witness "survived" diagnostic |
| A2-3 / O2-9 | WARNING / NOTE | yes | fix | PAC-5: the target diagnostic must be present, extra diagnostics allowed; the wrong-clause control is a mutant whose planted violation is absent and whose copy reds only with an unrelated diagnostic |
| A2-4 | WARNING | yes | fix | 0.2 keeps a local, untracked baseline per opaque label (presence, sha256); 4.2 verifies that baseline, not a loader everywhere; no loader created where none existed |
| A2-5 | NIT | yes | fix | 4.1 adds a local before/after checksum of the ignored files around `openspec archive` (pass/fail only) |
| O2-1 | WARNING | yes (this host has a global `core.excludesFile`) | fix | PAC-4 and the copy recipe use `git -c core.excludesFile=/dev/null check-ignore --no-index` with an empty `.git/info/exclude`; mutant with a `HOME` whose global ignore lists `.claude/` |
| O2-2 | WARNING | yes (`docs/security/security-architecture.md:347` digest, `docs/examples/postgres-state-store.md:46` DSN) | fix | PAC-6 bans only the scp form `user@host:path` (no `://` before `@`, no port); digest and DSN added to the allowed look-alikes |
| O2-3 | WARNING | yes (`metrics_catalog.go:18`, `metrics_catalog_test.go:18`, method at `internal/sink/registry.go:61`) | fix | PAC-3: symbol declared in any `.go` path on its line, methods and grouped `var (`/`const (` count; metric scenario names the two catalog files; method and grouped-block green cases |
| O2-4 | WARNING | yes (`run_gate()` defined at `hack/check.sh:26`) | fix | PAC-1 gate list between `<!-- gates:start -->`/`<!-- gates:end -->`, one code span per gate, `run_gate` names = first argument of non-comment calls; PAC-3 fixed `### When adding ...` headings; marker-removed mutant |
| O2-5 | WARNING | yes (`.github/gitleaks.toml:6` uses `[allowlist]`) | fix | PAC-4: every allowlist table form, each regex evaluated with a PCRE-compatible matcher failing on a regex error; `[[allowlists]]` and invalid-regex mutants |
| O2-6 | NOTE | yes | fix | exact OpenSpec markers, case-insensitive |
| O2-7 | NOTE | yes | fix | `100.64.0.0/10` and `fc00::/7` added to PAC-6 |
| O2-10 | NOTE | believed | fix in part | nested self-test cost bounded by the nesting rule; 1.1 records plain and self-test wall times in `evidence/G2.md` (stage B judges whether `lint` time is acceptable) |

No CRITICAL remains open.
