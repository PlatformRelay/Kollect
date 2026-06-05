// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// MetricSpec describes one kube-state-metrics-style Prometheus series from collected CR fields.
// Paths reference attribute names defined in spec.attributes (ADR-0304 spike).
type MetricSpec struct {
	// name is the bounded Prometheus series identifier within a profile.
	// +required
	Name string `json:"name"`

	// path is the attribute name whose numeric values are aggregated for this series.
	// Must match an entry in spec.attributes[].name.
	// +required
	Path string `json:"path"`

	// labels lists optional bounded label keys copied from extracted attributes.
	// Each entry must match an attribute name; cardinality is bounded by distinct label tuples.
	// +listType=set
	// +optional
	Labels []string `json:"labels,omitempty"`
}
