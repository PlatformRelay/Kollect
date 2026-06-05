# kollect Helm chart

Installs the [kollect](https://github.com/konih/kollect) operator and CRDs.

## Install

```bash
helm install kollect ./charts/kollect -n kollect-system --create-namespace
```

## Values

| Key | Description | Default |
| --- | --- | --- |
| `image.repository` | Controller image | `ghcr.io/konih/kollect` |
| `featureGates.inventoryHttp.enabled` | Expose `GET /inventory` | `false` |
| `oauth2Proxy.enabled` | Reserved for future auth sidecar | `false` |
| `webhooks.enabled` | Validating webhook for profiles | `true` |

See `values.yaml` for the full list.
