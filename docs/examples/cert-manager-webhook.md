# Example: Cert-manager webhook install

`webhooks.certManager.create: true` → `webhook-server-cert` ([ADR-0105](../adr/0105-webhook-serving-cert-management.md)).

Install cert-manager, then kollect Helm chart. Profile webhook blocks `Secret.data` without opt-in annotation.
