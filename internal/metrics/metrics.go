// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	ResultSuccess = "success"
	ResultFailure = "failure"

	ErrorClassTransient = "transient"
	ErrorClassTerminal  = "terminal"
	ErrorClassForbidden = "forbidden"

	// SAR access-cache outcomes (REL-05). Bounded enum.
	AccessCacheResultHit  = "hit"
	AccessCacheResultMiss = "miss"

	// Static-ref resolution results for cluster kinds (ADR-0208). Bounded enum.
	StaticRefResultOK        = "ok"
	StaticRefResultNotFound  = "not_found"
	StaticRefResultForbidden = "forbidden"

	// Static-ref types for cluster kinds (ADR-0208). Bounded enum.
	StaticRefTypeProfile  = "profile"
	StaticRefTypeSnapshot = "snapshot"
	StaticRefTypeDatabase = "database"
	StaticRefTypeEvent    = "event"

	// Metric label names shared by the metric vectors (metrics.go,
	// aggregation*.go) and the agent catalog (metrics_catalog.go). Bounded
	// label enums — do not extend without a cardinality note.
	LabelProfile  = StaticRefTypeProfile
	LabelGVK      = "gvk"
	LabelSeries   = "series"
	LabelGroup    = "group"
	LabelVersion  = "version"
	LabelResource = "resource"

	LabelController = "controller"
	LabelResult     = "result"
	LabelKind       = "kind"
	LabelSinkType   = "sink_type"

	// Catalog type names and the metric names repeated by the vectors and
	// the catalog.
	MetricTypeGauge     = "gauge"
	MetricTypeCounter   = "counter"
	MetricTypeHistogram = "histogram"

	MetricNameInventoryItemsTotal = "kollect_inventory_items_total"
	MetricNameCollectItemsTotal   = "kollect_collect_items_total"
)

var (
	InventoryItemsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricNameInventoryItemsTotal,
			Help: "Number of inventory items in the last aggregated snapshot.",
		},
	)

	CollectItemsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricNameCollectItemsTotal,
			Help: "Number of items currently held in the in-memory collection store.",
		},
	)

	CollectedObjects = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_collected_objects",
			Help: "Collected objects by profile and GVK.",
		},
		[]string{StaticRefTypeProfile, LabelGVK},
	)

	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_reconcile_total",
			Help: "Reconcile attempts by controller and result.",
		},
		[]string{LabelController, LabelResult},
	)

	ReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_reconcile_errors_total",
			Help: "Reconcile errors by kind and error class.",
		},
		[]string{LabelKind, "error_class"},
	)

	exportDurationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

	ExportDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kollect_export_duration_seconds",
			Help:    "Sink export duration in seconds.",
			Buckets: exportDurationBuckets,
		},
		[]string{LabelSinkType},
	)

	SinkErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_sink_errors_total",
			Help: "Inventory export failures by reason (transient, terminal, forbidden, payload_too_large).",
		},
		[]string{"reason"},
	)

	ExportSpillWarnTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kollect_export_spill_warn_total",
			Help: "Export payloads at or above the 1 MiB object-store spill warn threshold (ADR-0103).",
		},
	)

	ExportShardWarnTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kollect_export_shard_warn_total",
			Help: "Inventory namespace aggregates at or above the export sharding warn row threshold.",
		},
	)

	SinkConnectionTestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_sink_connection_test_total",
			Help: "Git/TLS sink connection tests by sink type and result.",
		},
		[]string{"type", LabelResult},
	)

	// ReconcileInFlight approximates workqueue depth (items currently being reconciled).
	ReconcileInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_workqueue_depth",
			Help: "Approximate reconcile workqueue depth (in-flight reconciles per controller).",
		},
		[]string{LabelController},
	)

	ReconcileDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kollect_reconcile_duration_seconds",
			Help:    "Controller reconcile latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelController},
	)

	InformerObjects = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_informer_objects",
			Help: "Objects in the dynamic informer indexer by GVR.",
		},
		[]string{LabelGroup, LabelVersion, LabelResource},
	)

	ExportBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_export_bytes_total",
			Help: "Total inventory payload bytes exported to sinks.",
		},
		[]string{LabelSinkType},
	)

	// CustomResourceSeries is registered via aggregation.go (ADR-0304 Phase 4, wired).

	ExportDebouncedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_export_debounced_total",
			Help: "Export attempts skipped by per-sink debounce coalescing.",
		},
		[]string{LabelController},
	)

	NamespaceFingerprintCacheTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_namespace_fingerprint_cache_total",
			Help: "Namespace content fingerprint cache outcomes (AR-10): hit skips the " +
				"SnapshotNamespace + ItemsFingerprint recompute, miss pays for it.",
		},
		[]string{LabelController, LabelResult},
	)

	WatchMapListErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_watch_map_list_errors_total",
			Help: "Secondary watch map handlers that failed to list related objects.",
		},
		[]string{LabelController, "watch"},
	)

	CollectDispatchDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "kollect_collect_dispatch_duration_seconds",
			Help:    "Collection informer dispatch latency (extract + store upsert) in seconds.",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	CollectDispatchQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kollect_collect_dispatch_queue_depth",
			Help: "Approximate depth of the collection dispatch queue (channel length).",
		},
	)

	CollectDispatchBackpressureTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kollect_collect_dispatch_backpressure_total",
			Help: "Informer dispatch sends that blocked waiting for dispatch queue capacity.",
		},
	)

	InformerResyncDispatchesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_informer_resync_dispatches_total",
			Help: "Informer Update events driven by periodic resync (same resourceVersion).",
		},
		[]string{LabelGroup, LabelVersion, LabelResource},
	)

	// CollectNamespaceMismatchTotal counts objects dropped because their namespace is
	// outside a target's effective namespace set. This rejection is otherwise silent:
	// a target resolved against a stale namespace snapshot collects nothing and looks
	// healthy (COLLECT-NS-BACKFILL). Labels are the GVR — bounded by profile specs.
	CollectNamespaceMismatchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_collect_namespace_mismatch_total",
			Help: "Collected objects rejected because their namespace is outside a target's " +
				"effective namespace set.",
		},
		[]string{LabelGroup, LabelVersion, LabelResource},
	)

	InformerClusterWideScope = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_informer_cluster_wide_scope",
			Help: "1 when a GVR informer watches all namespaces; 0 when namespace-scoped.",
		},
		[]string{LabelGroup, LabelVersion, LabelResource},
	)

	// StaticRefResolutionTotal counts cluster-kind namespaced static-ref resolutions (ADR-0208).
	// Labels are bounded enums — never pass free-form values.
	StaticRefResolutionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_static_ref_resolution_total",
			Help: "Cluster-kind namespaced static-ref resolutions by kind, ref_type, and result (ok/not_found/forbidden).",
		},
		[]string{LabelKind, "ref_type", LabelResult},
	)

	// AccessCacheTotal counts SelfSubjectAccessReview cache outcomes (REL-05).
	// result is a bounded enum (hit/miss); a miss triggers a fresh SAR.
	AccessCacheTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_access_cache_total",
			Help: "SelfSubjectAccessReview access-cache outcomes (REL-05): hit serves a cached " +
				"decision, miss issues a fresh SubjectAccessReview.",
		},
		[]string{LabelResult},
	)

	// LabeledSeriesCardinalityCappedTotal counts label tuples dropped by the
	// EC-P2-09 cardinality guard (DefaultMaxLabeledSeriesPerKey). profile/gvk/series
	// are config-bounded (from KollectProfile specs), not user data, so this stays low-cardinality.
	LabeledSeriesCardinalityCappedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_custom_resource_labeled_series_capped_total",
			Help: "Distinct label tuples dropped because they exceeded the per-series cardinality cap.",
		},
		[]string{StaticRefTypeProfile, LabelGVK, LabelSeries},
	)

	// CleanupTerminalTotal counts terminal sink-cleanup attempts on deleting
	// inventories (K-30). The finalizer is retained and the attempt re-checks
	// every 5 minutes, so a single wedged object increments it once per re-check:
	// alert on any increase, not on the absolute value. kind is a bounded enum
	// (inventory/cluster-inventory).
	CleanupTerminalTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_cleanup_terminal_total",
			Help: "Terminal sink-cleanup attempts on deleting inventories; the finalizer is retained and the attempt re-checks every 5 minutes, so one wedged object increments it once per re-check.",
		},
		[]string{LabelKind},
	)
)

// Register adds kollect custom metrics to the controller-runtime registry.
func Register() {
	metrics.Registry.MustRegister(
		InventoryItemsTotal,
		CollectItemsTotal,
		CollectedObjects,
		ReconcileTotal,
		ReconcileErrorsTotal,
		ExportDurationSeconds,
		SinkErrorsTotal,
		ExportSpillWarnTotal,
		ExportShardWarnTotal,
		SinkConnectionTestTotal,
		ReconcileInFlight,
		ReconcileDurationSeconds,
		InformerObjects,
		ExportBytesTotal,
		CustomResourceSeries,
		customResourceLabeledCollector{},
		ExportDebouncedTotal,
		NamespaceFingerprintCacheTotal,
		WatchMapListErrorsTotal,
		CollectDispatchDurationSeconds,
		CollectDispatchQueueDepth,
		CollectDispatchBackpressureTotal,
		InformerResyncDispatchesTotal,
		CollectNamespaceMismatchTotal,
		InformerClusterWideScope,
		StaticRefResolutionTotal,
		LabeledSeriesCardinalityCappedTotal,
		AccessCacheTotal,
		CleanupTerminalTotal,
	)
}
