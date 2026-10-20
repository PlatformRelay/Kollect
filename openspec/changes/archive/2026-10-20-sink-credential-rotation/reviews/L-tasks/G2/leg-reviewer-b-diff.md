## Verdict: CONCERNS

## Findings
- [WARNING] The SCR-7 watchdog applies to every degraded path, but the design limits it to a total export failure — `internal/controller/kollectinventory_controller.go:658`, `internal/controller/kollectclusterinventory_controller.go:719`
  Failure: `setInventoryDegraded` is also called for `Suspended` (:145), scope denial (:150), an unreachable sink (:158) and the spill gate (:220). The cluster `setDegraded` is called for the gate (:193), `ExportUnavailable` (:206) and scope denial (`cluster_scope_enforce.go:35,41`). A `0s` inventory with `spec.suspend: true` used to stay quiet. Now it reconciles and runs `Status().Update` every 30 s. design.md:157-164 and spec SCR-7 (spec.md:256-261) scope the change to "every sink export failed". Self-check item 6 in evidence/G2.md says the design specifies the wider behaviour; it does not. No test pins the suspended or denied rows.
  Fix: apply `RequeueAfterForZeroInterval` only in the total-failure branches (`kollectinventory_controller.go:264`, cluster `:226`), before the terminal zeroing. Alternatively, amend design and spec to the wider rule and add a suspended `0s` row.
  Confidence: 85
- [WARNING] An entirely empty `tls.caSecretRef` Secret changed from terminal `ErrCAKeyMissing` to the transient "has no data" error. Nothing tests or records this — `internal/sink/credentials.go:73`, `internal/sink/secret_refs.go:164`, `internal/sink/backend_pool.go:240`
  Failure: before the change, `SelectCAKey` (`ca_source.go:42`) returned a terminal error for an empty CA Secret on both paths. Now:
  - On the pool-disabled path, `getSecret` refuses the Secret before `checkCASecret` reaches `SelectCAKey` (`build_context.go:150-154`).
  - On the pool path, `fetchSecrets` fails on the CA use. `caRulesFirst` then returns that read error unchanged because `failed.use == caUse`.

  Result: a misconfiguration that used to be final now retries forever, and `errors.Is(err, ErrCAKeyMissing)` is false. Self-check item 5 says "New texts only", but an error class changed on an existing path.
  Fix: decide it on purpose. Either let the CA use skip the empty check so `SelectCAKey` decides, or keep the change and add a test row plus a line in G4's upgrading notes.
  Confidence: 80
- [NOTE] A retired backend is now closed before the rebuild, and also on spec change or a failed rebuild. Before, it was closed only at store — `internal/sink/backend_pool.go:287-295`, `:171`
  Failure: a concurrent export holding the old backend is closed under it for the whole build, not just at the swap. The brief puts this in G3's scope, but G2 widens the window until G3 lands.
  Fix: none in G2; G3's holders and deferred close must cover `lookupOrRetire` and `retirePoolEntry`.
  Confidence: 90
- [NOTE] In `TestRotationErrors_noSentinel`, the pool-dump check only proves the key and metadata fields are clean — `internal/sink/backend_pool_retire_test.go:1316-1319`
  Failure: the factory ignores `BuildContext`, so no pooled backend can ever hold a sentinel. That half of the assertion cannot fail. Real backends do hold credentials by design, so the claim should name what it covers.
  Fix: in evidence row 12, say that "pool entries" means the key and the metadata.
  Confidence: 70

Checked and holds:
- **Retire rules:** only not-found retires on a metadata error (`isSecretNotFound`). Cancelled, timeout and forbidden keep the entry. An observed key change retires before the rebuild.
- **Empty check:** present in `getSecret`, `fetchSecrets`, the `caRulesFirst` read and the CLI `resolveSecretValues`, after `stringData` and env substitution, with one shared text.
- **Stream retain:** happens before the acquire. The only types skipped are ones the old routing already retained (`cleanup.go:220-224`). It also skips the CA-source check for streams, which is benign.
- **Terminal zeroing:** still in place (`:265-267`).
- **"Green on base" claim:** true for `TestSecretMeta_notFoundIsTransient`. The diff only adds a `notFound` field; the plain-text mapping already existed in G1's `getSecretObject`.
- **Secret values:** no sentinel or real value appears in the changed code or tests beyond the `sentinel-*` and `AKID*` probe strings.

## Could not check
- I ran no tests or mutants. Results are taken from evidence/G2.md, and the FINAL full suite is still marked pending there.
- I did not read the cluster reconciler's terminal branch (`kollectclusterinventory_controller.go:220-230`) beyond its call site, or `per_sink_export.go` `shouldSkip`.
- No test covers GCS's ambient fallback directly; only S3 does. I assumed the generic check before the factory covers GCS.
- I did not check that `enableRotationPool`'s cleanup puts `backendPoolDisabled` back after the `pooled=false` subtest sets it (`backend_pool_retire_test.go:1113`).
