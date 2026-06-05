// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// MetricPathSpec describes one kube-state-metrics-style series from a collected CR field.
// Config surface (KollectProfile.spec.metrics vs companion CR) is TBD — see ADR-0033.
type MetricPathSpec struct {
	// Name is the Prometheus series identifier within a profile (bounded cardinality).
	Name string
	// Path is the attribute extraction path (JSONPath or CEL) — not evaluated in this stub.
	Path string
	// Labels lists optional bounded label keys copied from extracted attributes.
	Labels []string
}

// CustomResourceSeries exposes domain gauges derived from collected custom resources.
// Wired to the collection engine in Phase 4; registered now for catalog and scrape stability.
var CustomResourceSeries = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "kollect_custom_resource_series",
		Help: "Domain metric series from collected custom resources (ADR-0033 Phase 4 stub).",
	},
	[]string{"profile", "gvk", "series"},
)

// RecordCustomResourceSeries sets one domain series value for a profile/GVK tuple.
// Collection engine integration will call this when metric paths are configured.
func RecordCustomResourceSeries(profile, gvk, series string, value float64) {
	CustomResourceSeries.WithLabelValues(profile, gvk, series).Set(value)
}
