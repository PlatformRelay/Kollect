## Verdict: CONCERNS

## Findings
- [WARNING] The C1 test (5.1) cannot reach the CLI's CA error, so "CLI fails fast" is not real. The CA is resolved in `cliBuildContext` only after a live cluster collect, and the error is only printed to stderr, never returned. — `internal/pipeline/wire.go:462-496`, `cmd/cli/collect.go:143-153`
  Failure: with the fixture kubeconfig, collect fails first (`wire.go:464`) and the process exits 2 for the wrong reason. The `errors.Is` check in C1 has no returned error to test, and `collect_test.go:301-306` fakes `runAllContexts`, which skips the CA path entirely. With a real cluster, a garbage CA is only reported after a full collection in every context.
  Fix: resolve the CA (and build the context) once in `runCollectPipeline`, next to `ResolveSinkSecretData` (`collect.go:119`), and return it there as `ExitFatalError`. C1 then becomes testable and the CLI really fails fast.
  Confidence: 85
- [WARNING] The C3 test can never run green in CI, yet 7.1 requires "none skipped". `collect` needs envtest, which the integration job does not set up (`internal/pipeline/integration_test.go:38-45`). Loopback dialing needs the `integration` tag (`internal/sink/netguard/integration_policy.go:1-14`). — `tasks.md:212-214`
  Failure: C3 skips under `-tags integration` and is refused by netguard under `task test`, so the CLI-passes-the-CA guard never runs.
  Fix: follow the existing precedent `git_integration_test.go:36-45`: fill a store by hand and call `cliBuildContext` → `withCLIBackend` → `ExportTargets` against the X-signed httptest server.
  Confidence: 85
- [WARNING] The spec promises recovery after a Secret is fixed, but nothing triggers that retry. A terminal export error sets `RequeueAfter = 0`, and no controller watches Secrets. — `internal/controller/kollectinventory_controller.go:262-264`, `:685-700`, `family_sink_controller.go:118-125`
  Failure: a Secret with no CA key (exports today) or a DSN with `sslrootcert` becomes terminal on upgrade. Fixing the Secret changes nothing until some unrelated sink, inventory or store event arrives. PI4 calls `acquireBackend` directly, so it passes while the operator stays stuck.
  Fix: SCT-4 should name what triggers the next acquisition, and `troubleshooting.md` / `upgrading.md` should say to touch the sink. Alternatively, classify Secret-content errors as retryable.
  Confidence: 70
- [NOTE] The spec rejects `tlsInsecure`/`sslInsecure` whenever "set" (`spec.md:134-135`); the design rejects them only when "set to anything but `false`" (`design.md:82-83`). — `specs/sink-tls-trust/spec.md:134`
  Failure: `tlsInsecure=false` is an error under the spec and accepted under the design. M4 does not pin either reading.
  Fix: align the two texts and add a `tlsInsecure=false` row to M4.
  Confidence: 80
- [NOTE] "Reads the effective base endpoint from the loaded config" will miss `AWS_ENDPOINT_URL_S3`. `aws.Config.BaseEndpoint` carries only the global variable; the S3-specific one is resolved inside `s3.NewFromConfig`. — `design.md:94-95`
  Failure: a naive implementation passes S1's global case and misses the S3-specific one. S1b would catch it red.
  Fix: check `client.Options().BaseEndpoint` after `NewFromConfig`.
  Confidence: 60
- [NOTE] The CLI fetches the sink's credential Secret (`collect.go:119`) before the both-sources check; the operator checks the CA first (`build_context.go:38` before `:45`). — `specs/sink-tls-trust/spec.md:180`
  Failure: a both-set spec whose `secretRef` Secret is missing gives the both-sources error from the operator and not-found from the CLI. "Before either Secret is fetched" does not hold on the CLI.
  Fix: run the shared spec check before `ResolveSinkSecretData`, or reword to "before the CA Secret is fetched".
  Confidence: 60

**Requirements:**
- SCT-1 holds (S1–S3, I1–I4, G1).
- SCT-2 holds (D1–D8, PI1–PI5). Rejecting an unset sslmode or `prefer` is an interpretation of "sslmode still applies", recorded at `proposal.md:51-53`. It is stronger than the ruling's literal text and breaks every Postgres sink that has a CA and no sslmode; it is defensible because otherwise the CA would be silently ignored.
- SCT-3 holds, apart from the `false` mismatch above.
- SCT-4 holds as written; the recovery gap is above.
- SCT-5 is partial: C1 and C3 are not executable as written.
- SCT-6 holds; FR-EXP-7 is at `REQUIREMENTS.md:90`.
- Beyond any requirement: the shared resolver changes Kafka and NATS (both-set and missing-key errors). This is release-noted.

**File:line claims:** I checked all of these and they are accurate: `build_context.go:38/:215-245/:225/:244`, `wire.go:143-171/:169/:257/:494/:496/:506/:523`, `registry.go:120-157`, `probe.go:91/:115/:151/:163/:37`, `s3/backend.go:124-149/:127/:141-145`, `pool.go:16-24/:46-55/:69-76`, `mongodb/backend.go:97-105`, `connection.go:34-41`, `gcs/backend.go:26-40`, `gcs/connection.go:13-21`, `sinktls.go:24-26/:44`, `kollectsink_types.go:341-348`, `backend_pool.go:94/:112/:145/:263`, `sink_credentials_test.go:312/:354/:356/:384`, `dialer.go:242-248`, `exitcode.go:17`, `kollectconnectiontest_controller.go:91`, `family_sink_connection.go:49`, `security-architecture.md:221/:243`.

**Round-1 fixes in `verification.md`:**
- Hold: A-F1 (with the `false` note above), A-F4, A-F5, A-F6, A-F8, A-F9, A-F10, C-W3, C-N1, C-N3, C-N4, A-stale.
- A-F2 holds in the spec; the design detail is the NOTE above.
- A-F3 is partial: the CLI scope fix holds, but the rewritten C1 and the new C3 cannot run as specified (first two findings).
- A-F7 holds on the operator path; the CLI ordering is the last NOTE above.

## Could not check
- `agent-context/decisions.md`: does not exist in this worktree; the workspace copy is out of bounds. I could not confirm the operator accepted the "reject unset/`prefer`" reading.
- Third-party claims, because the module cache is outside the repo: pgx v5.11.0 `config.go:883-993/:903-908/:945-955`, `defaults.go:58`; mongo-driver `connstring.go:348/:579-640/:994/:1016`, `clientoptions.go:560-598`; aws-sdk-go-v2 `env_config.go:76`; go-git `common.go:262-275`.
- `docs/crds/*.md`, `pipeline-cli.md`, `troubleshooting.md`, `upgrading.md` line anchors, `Taskfile.yml:20`, `loader.go:41/:122-135`, `postgres/export_integration_test.go:34-45`: not opened.
