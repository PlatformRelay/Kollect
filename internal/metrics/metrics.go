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
)

var (
	InventoryItemsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kollect_inventory_items_total",
			Help: "Number of inventory items in the last aggregated snapshot.",
		},
	)

	CollectItemsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kollect_collect_items_total",
			Help: "Number of items currently held in the in-memory collection store.",
		},
	)

	CollectedObjects = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_collected_objects",
			Help: "Collected objects by profile and GVK.",
		},
		[]string{"profile", "gvk"},
	)

	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_reconcile_total",
			Help: "Reconcile attempts by controller and result.",
		},
		[]string{"controller", "result"},
	)

	ReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_reconcile_errors_total",
			Help: "Reconcile errors by kind and error class.",
		},
		[]string{"kind", "error_class"},
	)

	exportDurationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

	ExportDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kollect_export_duration_seconds",
			Help:    "Sink export duration in seconds.",
			Buckets: exportDurationBuckets,
		},
		[]string{"sink_type"},
	)

	SinkErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_sink_errors_total",
			Help: "Inventory export failures by reason (transient, terminal, forbidden, payload_too_large).",
		},
		[]string{"reason"},
	)

	SinkConnectionTestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_sink_connection_test_total",
			Help: "Git/TLS sink connection tests by sink type and result.",
		},
		[]string{"type", "result"},
	)

	// ReconcileInFlight approximates workqueue depth (items currently being reconciled).
	ReconcileInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_workqueue_depth",
			Help: "Approximate reconcile workqueue depth (in-flight reconciles per controller).",
		},
		[]string{"controller"},
	)

	ReconcileDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kollect_reconcile_duration_seconds",
			Help:    "Controller reconcile latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller"},
	)

	InformerObjects = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kollect_informer_objects",
			Help: "Objects in the dynamic informer indexer by GVR.",
		},
		[]string{"group", "version", "resource"},
	)

	ExportBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_export_bytes_total",
			Help: "Total inventory payload bytes exported to sinks.",
		},
		[]string{"sink_type"},
	)

	// HubSpokeReportsTotal is a stub for future hub/spoke merge at 100+ cluster scale.
	HubSpokeReportsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_hub_spoke_reports_total",
			Help: "Spoke inventory reports received by hub transport (stub until merge wired).",
		},
		[]string{"hub", "result"},
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
		SinkConnectionTestTotal,
		ReconcileInFlight,
		ReconcileDurationSeconds,
		InformerObjects,
		ExportBytesTotal,
		HubSpokeReportsTotal,
	)
}
