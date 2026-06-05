# Example: Cert-manager webhook install

!!! warning "Requires cert-manager"
    Enable `webhooks.certManager.create: true` only when [cert-manager](https://cert-manager.io/docs/)
    is already installed in the cluster. The chart creates a `Certificate` for the validating webhook.

`webhooks.certManager.create: true` → `webhook-server-cert` ([ADR-0105](../adr/0105-webhook-serving-cert-management.md)).

Install cert-manager, then kollect Helm chart. Profile webhook blocks `Secret.data` without opt-in annotation.
