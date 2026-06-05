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
)

var (
	InventoryItemsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kollect_inventory_items_total",
			Help: "Number of inventory items in the last aggregated snapshot.",
		},
	)

	SinkConnectionTestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kollect_sink_connection_test_total",
			Help: "Git/TLS sink connection tests by sink type and result.",
		},
		[]string{"type", "result"},
	)
)

// Register adds kollect custom metrics to the controller-runtime registry.
func Register() {
	metrics.Registry.MustRegister(InventoryItemsTotal, SinkConnectionTestTotal)
}
