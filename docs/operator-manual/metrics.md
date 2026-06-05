# Operator metrics

Kollect exposes **operator metrics** on the controller `/metrics` endpoint ([ADR-0601](../adr/0601-prometheus-metrics-stub.md)).
Inventory payloads export to configured sinks — there is **no** `KollectSink` type `prometheus`.

## Endpoint

| Setting | Default | Notes |
| --- | --- | --- |
| Bind address | `:8443` | Helm `metrics.bindAddress` |
| TLS | On (`metrics.secure: true`) | Kubernetes API **TokenReview** + **SubjectAccessReview** on `/metrics` |
| Service | `<release>-kollect-controller-manager` | Port name `metrics` (8443) |

Scrape with a Prometheus service account that can `GET /metrics` (see `config/rbac/metrics_reader_role.yaml`).

## Prometheus Operator (Helm)

When [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) CRDs are installed (e.g. [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)):

```yaml
metrics:
  serviceMonitor:
    enabled: true
    labels:
      release: kube-prometheus-stack   # match your Prometheus serviceMonitorSelector
  prometheusRule:
    enabled: true
    labels:
      release: kube-prometheus-stack   # match your Prometheus ruleSelector
```

See [`charts/kollect/ci/monitoring-values.yaml`](../../charts/kollect/ci/monitoring-values.yaml) for a full overlay.

### Default alerts (`kollect.rules`)

| Alert | Severity | Summary |
| --- | --- | --- |
| `KollectMetricsTargetDown` | critical | Prometheus not scraping the metrics Service |
| `KollectOperatorNotReady` | critical | Controller pod failing `/readyz` |
| `KollectReconcileErrors` | warning | Any sustained reconcile error counter increase |
| `KollectInventoryExportErrors` | warning | `KollectInventory` reconcile errors |
| `KollectSinkExportErrors` | warning | Sink export failures (`kollect_sink_errors_total`) |
| `KollectSinkConnectionTestFailures` | warning | Connection test failures |
| `KollectExportLatencyHigh` | warning | p95 export duration &gt; 10s |
| `KollectWorkqueueBacklog` | warning | In-flight reconciles &gt; 10 sustained |
| `KollectHubSpokeReportFailures` | warning | Hub spoke report merge failures (hub mode) |

Append custom rules via `metrics.prometheusRule.additionalRules`.

## Metric catalog

All custom metrics use the `kollect_` prefix. Controller-runtime also exposes standard workqueue and Go runtime metrics on the same endpoint.

### Reconciliation

| Metric | Type | Labels | Help |
| --- | --- | --- | --- |
| `kollect_reconcile_total` | counter | `controller`, `result` | Reconcile attempts (`success` / `failure`) |
| `kollect_reconcile_errors_total` | counter | `kind`, `error_class` | Errors by CR kind and class (`transient`, `terminal`, `forbidden`) |
| `kollect_reconcile_duration_seconds` | histogram | `controller` | Reconcile latency |
| `kollect_workqueue_depth` | gauge | `controller` | In-flight reconciles (approximate depth) |

### Collection

| Metric | Type | Labels | Help |
| --- | --- | --- | --- |
| `kollect_collect_items_total` | gauge | — | Items in the in-memory collection store |
| `kollect_collected_objects` | gauge | `profile`, `gvk` | Objects collected per profile/GVK |
| `kollect_informer_objects` | gauge | `group`, `version`, `resource` | Dynamic informer indexer size |
| `kollect_inventory_items_total` | gauge | — | Items in last inventory HTTP snapshot |

### Export

| Metric | Type | Labels | Help |
| --- | --- | --- | --- |
| `kollect_export_duration_seconds` | histogram | `sink_type` | Sink export latency (buckets: 5ms–10s) |
| `kollect_export_bytes_total` | counter | `sink_type` | Payload bytes exported |
| `kollect_sink_errors_total` | counter | `reason` | Export failures (`transient`, `terminal`, `forbidden`, `payload_too_large`, `spill_required`, …) |
| `kollect_export_spill_warn_total` | counter | — | Payloads at/above 1 MiB spill warn threshold |
| `kollect_sink_connection_test_total` | counter | `type`, `result` | Git/TLS connection tests |

### Profile-derived (Phase 4)

| Metric | Type | Labels | Help |
| --- | --- | --- | --- |
| `kollect_custom_resource_series` | gauge | `profile`, `gvk`, `series` | KSM-style series from `KollectProfile.spec.metrics` |
| `kollect_custom_resource_labeled_series` | gauge | `profile`, `gvk`, `series`, … | Same with attribute label dimensions |

### Hub / spoke

| Metric | Type | Labels | Help |
| --- | --- | --- | --- |
| `kollect_hub_spoke_reports_total` | counter | `hub`, `result` | Spoke reports merged by hub |
| `kollect_hub_merged_items_total` | counter | `hub`, `cluster` | Rows applied by hub merge |

## Useful PromQL

```promql
# Reconcile error rate by class
sum(rate(kollect_reconcile_errors_total[5m])) by (error_class)

# Export p95 by sink type
histogram_quantile(0.95, sum(rate(kollect_export_duration_seconds_bucket[5m])) by (le, sink_type))

# Sink failure reasons
sum(increase(kollect_sink_errors_total[15m])) by (reason)

# Store growth (scalability — ADR-0603)
kollect_collect_items_total
```

Source of truth for registration: `internal/metrics/metrics.go` and `internal/metrics/metrics_catalog.go`.

## See also

- [ADR-0602: Error taxonomy and metrics](../adr/0602-error-taxonomy.md)
- [ADR-0603: Performance and scalability](../adr/0603-performance-scalability.md)
- [Helm values — metrics](helm-values.md#resources-metrics-and-webhooks)
- [Chart README — monitoring](../../charts/kollect/README.md#prometheus-operator-monitoring)
