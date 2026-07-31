# ADR-0105: Webhook serving and certificate management

> How validating webhooks are served and how their TLS certificates are provisioned.

**Theme:** 01 · Foundations · **Status:** Current

## Context

Kollect uses validating webhooks to reject invalid profiles, sinks, and scopes before reconciliation.
The manager serves them on port 9443 and mounts TLS material from the Secret named by
`webhooks.certManager.secretName` (default `webhook-server-cert`).

## Decision

- Validating webhooks are enabled by default; there are no mutating webhooks.
- The default Helm path requires cert-manager. The chart renders a namespaced self-signed `Issuer`
  and a `Certificate`; cert-manager creates and rotates the serving Secret and injects CA trust into
  the `ValidatingWebhookConfiguration`.
- `webhooks.certManager.create: false` only suppresses those cert-manager resources. It does not
  generate a Secret or inject a CA. Operators selecting it must provision the named TLS Secret and
  establish webhook CA trust outside the chart before the manager starts.
- `webhooks.enabled: false` disables validating admission and its serving-certificate mount. It is
  supported for constrained development overlays, not recommended as a production workaround.
- Every ready replica may serve webhook traffic through the chart's Service.

## Trust and rotation

cert-manager owns rotation on the default path. On the operator-provided path, rotation and CA
rollover are also operator-owned. Sink `caBundle` and `caSecretRef` fields are unrelated: they
configure outbound sink trust, not the manager's serving certificate.

## Verification

Helm tests cover the default `Issuer`, `Certificate`, Secret mount, webhook service, and CA-injection
annotations. Existing-cluster installation documentation names cert-manager as a prerequisite and
states the obligations of the operator-provided path.

## Consequences

- The secure default provides automated certificate rotation, at the cost of a hard cert-manager
  dependency for the default install.
- Clusters with another PKI controller can integrate it by disabling chart-created cert-manager
  resources and owning both the Secret and CA trust explicitly.
- Kollect does not ship an automatic certificate-generation alternative.
