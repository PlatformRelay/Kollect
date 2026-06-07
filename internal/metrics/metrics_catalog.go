// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

// CatalogEntry documents one operator metric for agents (grep "kollect_" in this file).
type CatalogEntry struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Labels     []string `json:"labels,omitempty"`
	Help       string   `json:"help,omitempty"`
	PromQLHint string   `json:"promqlHint"`
	AgentHint  string   `json:"agentHint"`
}

// Catalog lists perf-related kollect metrics with help strings agents can grep.
// Keep in sync with metrics.go Register() and docs/PERFORMANCE.md.
var Catalog = []CatalogEntry{
	{
		Name:       "kollect_inventory_items_total",
		Type:       "gauge",
		Help:       "Number of inventory items in the last aggregated snapshot.",
		PromQLHint: "kollect_inventory_items_total",
		AgentHint:  "Stale while store grows — inventory reconcile or export path lag.",
	},
	{
		Name:       "kollect_collect_items_total",
		Type:       "gauge",
		Help:       "Number of items currently held in the in-memory collection store.",
		PromQLHint: "kollect_collect_items_total",
		AgentHint:  "Tracks store size; RSS scales with this at 10k+ objects.",
	},
	{
		Name:       "kollect_collected_objects",
		Type:       "gauge",
		Labels:     []string{"profile", "gvk"},
		Help:       "Collected objects by profile and GVK.",
		PromQLHint: "sum by (profile, gvk) (kollect_collected_objects)",
		AgentHint:  "Per-target cardinality; split profiles when label cardinality explodes.",
	},
	{
		Name:       "kollect_reconcile_total",
		Type:       "counter",
		Labels:     []string{"controller", "result"},
		Help:       "Reconcile attempts by controller and result.",
		PromQLHint: "sum(rate(kollect_reconcile_total[5m])) by (controller, result)",
		AgentHint:  "Failure ratio rising — check error class counters next.",
	},
	{
		Name:       "kollect_reconcile_errors_total",
		Type:       "counter",
		Labels:     []string{"kind", "error_class"},
		Help:       "Reconcile errors by kind and error class.",
		PromQLHint: "sum(rate(kollect_reconcile_errors_total[5m])) by (error_class)",
		AgentHint:  "forbidden → RBAC/SAR; transient → API or sink backoff.",
	},
	{
		Name:       "kollect_sink_errors_total",
		Type:       "counter",
		Labels:     []string{"reason"},
		Help:       "Inventory export failures by reason (transient, terminal, forbidden, payload_too_large).",
		PromQLHint: "sum(rate(kollect_sink_errors_total[5m])) by (reason)",
		AgentHint:  "Separate from reconcile errors — inspect sink creds, payload size, and export path.",
	},
	{
		Name:       "kollect_export_duration_seconds",
		Type:       "histogram",
		Labels:     []string{"sink_type"},
		Help:       "Sink export duration in seconds.",
		PromQLHint: "histogram_quantile(0.95, sum(rate(kollect_export_duration_seconds_bucket[5m])) by (le, sink_type))",
		AgentHint:  "Sink slowness (Git/Postgres/Kafka), not collection.",
	},
	{
		Name:       "kollect_sink_connection_test_total",
		Type:       "counter",
		Labels:     []string{"type", "result"},
		Help:       "Git/TLS sink connection tests by sink type and result.",
		PromQLHint: "sum(rate(kollect_sink_connection_test_total[5m])) by (type, result)",
		AgentHint:  "Spikes on sink CR churn — expected; sustained failure → creds/network.",
	},
	{
		Name:       "kollect_workqueue_depth",
		Type:       "gauge",
		Labels:     []string{"controller"},
		Help:       "Approximate reconcile workqueue depth (in-flight reconciles per controller).",
		PromQLHint: "max_over_time(kollect_workqueue_depth[5m])",
		AgentHint:  "Sustained high → raise --max-concurrent-reconciles-* or reduce reconcile work.",
	},
	{
		Name:       "kollect_reconcile_duration_seconds",
		Type:       "histogram",
		Labels:     []string{"controller"},
		Help:       "Controller reconcile latency in seconds.",
		PromQLHint: "histogram_quantile(0.99, sum(rate(kollect_reconcile_duration_seconds_bucket[5m])) by (le, controller))",
		AgentHint:  "p99 up with low depth → slow deps; p99 up with high depth → under-provisioned workers.",
	},
	{
		Name:       "kollect_informer_objects",
		Type:       "gauge",
		Labels:     []string{"group", "version", "resource"},
		Help:       "Objects in the dynamic informer indexer by GVR.",
		PromQLHint: "sum by (group, version, resource) (kollect_informer_objects)",
		AgentHint:  "Unexpected growth → cluster-wide watch; prefer namespace-scoped targets.",
	},
	{
		Name:       "kollect_export_bytes_total",
		Type:       "counter",
		Labels:     []string{"sink_type"},
		Help:       "Total inventory payload bytes exported to sinks.",
		PromQLHint: "rate(kollect_export_bytes_total[5m])",
		AgentHint:  "Spike → debounce too low or inventory churn.",
	},
	{
		Name:       "kollect_export_spill_warn_total",
		Type:       "counter",
		Help:       "Export payloads at or above the 1 MiB object-store spill warn threshold (ADR-0103).",
		PromQLHint: "increase(kollect_export_spill_warn_total[1h])",
		AgentHint:  "Approaching spill gate — shrink inventory or raise maxExportBytes/spill config.",
	},
	{
		Name:       "kollect_export_debounced_total",
		Type:       "counter",
		Labels:     []string{"controller"},
		Help:       "Exports skipped by per-inventory debounce coalescing.",
		PromQLHint: "sum(rate(kollect_export_debounced_total[5m])) by (controller)",
		AgentHint:  "High rate is expected when exportMinInterval is tight; use status.sinkExports for per-sink detail.",
	},
	{
		Name:       "kollect_watch_map_list_errors_total",
		Type:       "counter",
		Labels:     []string{"controller", "watch"},
		Help:       "Errors listing resources during watch map handler setup.",
		PromQLHint: "increase(kollect_watch_map_list_errors_total[15m])",
		AgentHint:  "Sustained increase → RBAC or API list failures for a GVR.",
	},
	{
		Name:       "kollect_collect_dispatch_duration_seconds",
		Type:       "histogram",
		Help:       "Collection informer dispatch latency (extract + store upsert).",
		PromQLHint: "histogram_quantile(0.95, sum(rate(kollect_collect_dispatch_duration_seconds_bucket[5m])) by (le))",
		AgentHint:  "Rising p95 with queue depth → raise --collect-dispatch-workers or --collect-dispatch-queue-size.",
	},
	{
		Name:       "kollect_collect_dispatch_queue_depth",
		Type:       "gauge",
		Help:       "Approximate collection dispatch queue depth.",
		PromQLHint: "max_over_time(kollect_collect_dispatch_queue_depth[5m])",
		AgentHint:  "Sustained high depth → increase workers/queue; watch sync_fallback counter.",
	},
	{
		Name:       "kollect_collect_dispatch_sync_fallback_total",
		Type:       "counter",
		Help:       "Informer events processed synchronously when dispatch queue was full.",
		PromQLHint: "increase(kollect_collect_dispatch_sync_fallback_total[15m])",
		AgentHint:  "Non-zero sustained rate → dispatch pool undersized for churn.",
	},
	{
		Name:       "kollect_informer_resync_dispatches_total",
		Type:       "counter",
		Labels:     []string{"group", "version", "resource"},
		Help:       "Informer Update events from periodic resync (unchanged resourceVersion).",
		PromQLHint: "sum(increase(kollect_informer_resync_dispatches_total[1h])) by (group, version, resource)",
		AgentHint:  "Spike every resync period → expected; tune --informer-resync-period if costly.",
	},
	{
		Name:       "kollect_informer_cluster_wide_scope",
		Type:       "gauge",
		Labels:     []string{"group", "version", "resource"},
		Help:       "1 when informer watches all namespaces for a GVR; 0 when namespace-scoped.",
		PromQLHint: "max by (group, version, resource) (kollect_informer_cluster_wide_scope)",
		AgentHint:  "Value 1 at scale → high RSS risk; tighten namespace selectors.",
	},
	{
		Name:       "kollect_custom_resource_series",
		Type:       "gauge",
		Labels:     []string{"profile", "gvk", "series"},
		Help:       "Domain metric series from collected custom resources (ADR-0304 Phase 4 stub).",
		PromQLHint: "sum by (profile, gvk, series) (kollect_custom_resource_series)",
		AgentHint:  "Phase 4 KSM-style paths; misconfigured series names explode cardinality.",
	},
	{
		Name:       "kollect_custom_resource_labeled_series",
		Type:       "gauge",
		Labels:     []string{"profile", "gvk", "series", "<attribute labels>"},
		Help:       "Domain metric series with attribute label dimensions from spec.metrics[].labels.",
		PromQLHint: "sum by (profile, gvk, series) (kollect_custom_resource_labeled_series)",
		AgentHint:  "Per-label-tuple sums when profile metrics declare labels; bounded by distinct tuples.",
	},
}
