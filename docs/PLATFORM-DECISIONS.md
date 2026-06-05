# Platform decisions (2026-06-05)

Locked outcomes from the coordinator platform review. These are **Accepted** — do not relitigate
without a new ADR. Implementation may lag docs; ADRs are the source of truth for intent.

| # | Decision | ADR / doc |
| --- | --- | --- |
| 1 | **`KollectScope` Phase 1** — validating webhook **and** reconciler enforcement (not webhook-only) | [ADR-0016](adr/0016-namespaced-multi-tenancy.md) |
| 2 | **`KollectProfile` → namespaced**; reserve **`KollectClusterProfile`** for platform-shared schemas | [ADR-0031](adr/0031-namespaced-profiles.md) |
| 3 | **Connection test** — no `KollectConnectionTest` CR; prod **`connectionTest: false`** + annotation **`kollect.dev/test-connection`**; samples/CI keep **`true`** | [ADR-0030](adr/0030-connection-test.md) |
| 4 | **Helm release sample** — `status.lastAttemptedRevision` for chart version; **contract test** validates `history[0]` ordering | [ADR-0027](adr/0027-helm-release-inventory.md) |
| 5 | **Hub L1→L4** — merge library + same image **`mode: hub\|spoke`** before **`KollectHub` CRD** owns Deployment | [ADR-0022](adr/0022-multi-cluster-sync-rfc.md) |
| 6 | **Transport** — **`inprocess` only default**; Redis/NATS/Kafka explicit opt-in after integration/e2e proof — never chart default | [ADR-0023](adr/0023-lean-queue-transport.md) |
| 7 | **ADR-0001** open items closed — Helm day 1, validating webhooks early | [ADR-0001](adr/0001-kubebuilder-v4.md) |
| 8 | **Doc-sync guardrails** reaffirmed — export-only in operator; no templating/CMS in reconcilers | [ADR-0011](adr/0011-doc-sync-templating.md) |
| 9 | **Shared informer per GVK** across Targets (kube-state-metrics pattern) | [ADR-0014](adr/0014-event-driven-informers.md) |

## Still open

| Question | Where |
| --- | --- |
| Hub spoke cross-cluster identity (mTLS, OIDC, bootstrap tokens) — distinct from inventory HTTP auth | [ADR-0028](adr/0028-hub-cluster-auth-istio-pattern.md) |

## Deferred (documented, not open)

| Item | When |
| --- | --- |
| **`KollectClusterSink`** + namespaced sink split | Phase 3 — see [ROADMAP.md](ROADMAP.md) |

## See also

- [REQUIREMENTS.md](REQUIREMENTS.md) — binding ship order
- [ROADMAP.md](ROADMAP.md) — phased delivery status
