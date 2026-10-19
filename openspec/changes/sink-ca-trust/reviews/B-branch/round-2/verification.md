# Stage B round 2 — verification

Run because round 1's astra leg raised a CRITICAL (A1). Tree `ba80c8023` (fixes 8.1-8.10 and their
records). Fresh legs, run in parallel and waited on by PID, scoped to "round-1 fixes closed, plus
anything new": astra adversarial (`brief-astra.md`), exit 0: **REQUEST_CHANGES** (one WARNING);
claude/opus:spec (`fanout.sh --no-unify --leg-timeout 1800`), exit 0: **CLEAN** (three NOTEs). No
CRITICAL. Both legs: every round-1 finding closed except the residue below; SCT-1 … SCT-5 hold,
SCT-6 holds apart from the release note deferred to hand-off (round-1 O3). This round is also the
second and last review of the section-8 fixes.

| Id | Finding | Leg, sev | Verified? (evidence) | Disposition |
|---|---|---|---|---|
| R2-1 | A 307 loop on the request's own URL reaches the limit with a `*url.Error` whose URL equals the request URL, so 8.9's URL comparison returns the raw error | astra WARNING | yes: red in the new unit test (`Head "https://endpoint.invalid/b": stopped after 10 redirects`). Impact is small (the URL is the sink's own endpoint), but 8.9's claim did not hold | fix 8.11: the limit error is a sentinel matched by identity; mutant killed |
| R2-2 | `ErrRedirectFailed` drops a context error's identity on a redirected hop | opus NOTE (40) | yes: red (`does not wrap context deadline exceeded`) | fix 8.11: `context.Canceled`/`DeadlineExceeded` kept with `%w`, no text; mutant killed |
| R2-3 | The unparsable-`Location` match relies on net/http's text and is guarded only by an integration test | opus NOTE (45) | yes | fix 8.11: unit test with a fake transport (runs in `task test`); mutant killed |
| R2-4 | `withSSLMode` says a later DSN `sslmode` wins; unconfirmed for the URL form | opus NOTE (25) | rejected: pgx `parse_url.go` `parseURLQueryParams` assigns `settings[key] = value` per pair (last occurrence wins, stated in its doc comment) | no change |

No further review after these fixes (round-2 rule: verified behaviour defects fixed with mutants).

## Final tree

`bd7f93ace`, 2026-10-19, `umask 022`, `KUBEBUILDER_ASSETS` from the main checkout:

| Gate | Command | Exit | Result |
|---|---|---|---|
| Unit, changed packages | `go test -count=1 ./internal/sink/... ./internal/pipeline/... ./internal/controller/... ./cmd/... ./api/...` | 0 | 24 `ok`, no `FAIL` (C1b ran with envtest assets) |
| Integration (Docker) | `task test-integration` | 0 | expected 66, executed 66, passed 66, skipped 0, `RESULT: PASS` |
| Lint | `task lint` | 0 | `0 issues.`, go-arch-lint `OK - No warnings found` |
| Lint, integration tag, branch files | `bin/golangci-lint run --build-tags integration ./internal/sink/... ./internal/pipeline/...` filtered to the files this branch adds | 0 | no finding in them (D5: the gate itself still skips tagged files) |
| Vet, integration tag | `go vet -tags=integration ./...` | 0 | clean |
| Codegen drift | `task verify` | 0 | `verify: ok` |
| OpenSpec | `task spec:validate` | 0 | 15 passed, 0 failed |
| Docs (D6) | `task docs:verify` (`MISE_KUBECTL_VERSION=1.36.1`, mkdocs from `docs/requirements-docs.txt` in a venv) | 0 | `mkdocs build --strict` built, browser layout check ran, `all available checks passed` (on `387ef99bf`; later commits touch no page under `docs/`) |
| Scrub, secrets | `task scrub`; `gitleaks detect --log-opts=317279084..HEAD` (`MISE_GITLEAKS_VERSION=8.30.1`) | 0, 0 | `scrub: ok`; `no leaks found` |
