# ADR-0105: Webhook serving and certificate management

> How validating webhooks are served, how TLS serving certs are provisioned and rotated, and the
> cert-manager vs self-signed bootstrap paths for clusters without cert-manager.

**Theme:** 01 · Foundations · **Status:** Current

## Context

Kollect relies on **validating webhooks** from Phase 0/1 ([ADR-0201](0201-crd-model.md),
[ADR-0202](0202-static-vs-reconciled.md)) to reject bad profiles, sinks, and scope at admission —
before reconcilers wedge on terminal config. The operator serves webhooks on **port 9443**
(controller-runtime default) with certs mounted at `/tmp/k8s-webhook-server/serving-certs`
([ADR-0704](0704-helm-chart-crd-lifecycle.md)).

The Helm chart already wires **cert-manager** (`Certificate` + self-signed `Issuer` →
`webhook-server-cert` Secret) and snapshot-tests that path. Many adopters run cert-manager; many
small/lab clusters do not. **Q4 (2026-06-05):** support a **self-signed bootstrap fallback**
alongside cert-manager — cert-manager remains the **recommended** production default
([ADR-0104](0104-security-model.md)).

## Decision

### Serving model

- **Validating only** — no mutating webhooks in v1alpha1; `failurePolicy=fail`, `sideEffects=None`.
- Webhooks are **enabled by default** in the Helm chart (`webhooks.enabled: true`); dev overlays may
  disable them for fast iteration (`charts/kollect/ci/dev-values.yaml`).
- The manager registers webhooks only when `--validating-webhooks-enabled=true` (chart sets this from
  values).
- **All ready replicas** may serve webhook traffic; the apiserver load-balances to the webhook
  `Service`. Serving cert trust is cluster-wide via `ValidatingWebhookConfiguration.clientConfig.caBundle`.

### Certificate provisioning (two supported paths)

| Path | When | Mechanism |
| --- | --- | --- |
| **A — cert-manager (default)** | Production and any cluster with cert-manager | Chart renders `Issuer` + `Certificate`; cert-manager writes `tls.crt`/`tls.key`/`ca.crt` into the serving `Secret` and keeps them rotated. Documented soft dependency. |
| **B — self-signed bootstrap (fallback)** | Clusters **without** cert-manager | Chart renders a **one-shot bootstrap** (Helm hook `Job` or equivalent) that generates a long-lived self-signed serving cert + CA, writes the `Secret`, and patches `ValidatingWebhookConfiguration.clientConfig.caBundle`. No cert-manager CRs. |

**Mutual exclusion:** `webhooks.certManager.create: true` (default) selects path A;
`webhooks.certManager.create: false` with `webhooks.selfSigned.bootstrap: true` selects path B.
The chart must not render both.

### Trust and rotation

- **Path A:** cert-manager owns rotation; operator mounts the Secret read-only; no in-process cert
  generation.
- **Path B:** bootstrap runs on install/upgrade when the Secret or `caBundle` is missing or stale
  (checksum annotation on the webhook config). Rotation is **manual or upgrade-triggered** — acceptable
  for lab/small installs; document that path A is preferred for production.
- **`caBundle` size:** inline PEM in CR specs remains capped at **64 KiB** at webhook
  ([ADR-0201](0201-crd-model.md)); this ADR governs **serving** certs for the operator webhook
  endpoint, not sink `caBundle` fields.

### E2E and CI

- Kind e2e installs cert-manager when webhook tests run (`test/e2e/e2e_suite_test.go`) — path A parity.
- Add a **helm-unittest** snapshot for path B (bootstrap enabled, no cert-manager objects) before
  marking bootstrap implemented.
- Contract: webhook readiness must appear in e2e smoke when webhooks are enabled
  ([ADR-0301](0301-event-driven-informers.md), [ADR-0706](0706-testing-merge-gate-architecture.md)).

## Consequences

- Small installs can enable webhooks without cert-manager; production guidance stays "install
  cert-manager or use path A."
- Two chart paths increase snapshot-test surface; mutual exclusion keeps runtime simple.
- Self-signed bootstrap does not auto-rotate like cert-manager — operators must plan upgrades or migrate
  to path A for long-lived fleets.
- Security posture for webhook TLS is explicit in one ADR; [ADR-0104](0104-security-model.md) links here
  for serving trust.

## Open questions

- **DECIDED (2026-06-05, Q4):** Self-signed fallback **yes**; cert-manager **recommended default**.
- **OPEN:** Bootstrap implementation detail — Helm hook `Job` vs in-manager cert writer on first start?
  (Default leaning: Helm hook to avoid running cert logic in the reconciler process.)
- **OPEN:** Should path B also support `--webhook-cert-path` for bring-your-own Secret (no generation)?
- **OPEN:** NetworkPolicy template for webhook ingress from control-plane / apiserver — chart optional
  or doc-only?
