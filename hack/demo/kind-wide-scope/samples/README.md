# Annotated Kollect CR samples — wide-scope demo

These files mirror [`../base/kollect/`](../base/kollect/) with **inline comments** explaining each
step of the sales-pitch narrative. Apply the full stack via kustomize (recommended):

```sh
kubectl apply -k hack/demo/kind-wide-scope/
```

Or study / apply individually in order:

| # | File | Role |
| --- | --- | --- |
| 1 | [01-kollectscope.yaml](01-kollectscope.yaml) | Collection ceiling — 8 GVK types, 6 namespaces |
| 2 | [02-kollectprofile-core.yaml](02-kollectprofile-core.yaml) | Deployment / Service / ConfigMap extraction |
| 3 | [03-kollectprofile-trivy.yaml](03-kollectprofile-trivy.yaml) | **Headline** — CVE report attributes |
| 4 | [04-kollectprofile-certificate.yaml](04-kollectprofile-certificate.yaml) | TLS expiry + issuer |
| 5 | [05-kollectprofile-externalsecret.yaml](05-kollectprofile-externalsecret.yaml) | Secret sync posture |
| 6 | [06-kollecttarget-fleet.yaml](06-kollecttarget-fleet.yaml) | Multi-namespace fleet Targets |
| 7 | [07-kollecttarget-upstream.yaml](07-kollecttarget-upstream.yaml) | Trivy + cert-manager + ESO Targets |
| 8 | [08-kollectsink.yaml](08-kollectsink.yaml) | Git push to kollect-inventory-demo |
| 9 | [09-kollectinventory.yaml](09-kollectinventory.yaml) | Debounced Git export wiring |

For **dual-cadence** Postgres + Git (`exportMinInterval` per ref), see
`config/samples/kollect_v1alpha1_kollectinventory.yaml`
and [deployment-inventory.md](../../../docs/examples/deployment-inventory.md#step-4-kollectinventory)
([ADR-0413](../../../docs/adr/0413-export-interval-scheduling.md)).
