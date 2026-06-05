// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

// MetricPathsFromProfile converts KollectProfile.spec.metrics into engine MetricPathSpec values.
func MetricPathsFromProfile(profile kollectdevv1alpha1.KollectProfileSpec) []metrics.MetricPathSpec {
	if len(profile.Metrics) == 0 {
		return nil
	}

	out := make([]metrics.MetricPathSpec, 0, len(profile.Metrics))
	for _, m := range profile.Metrics {
		out = append(out, metrics.MetricPathSpec{
			Name:   m.Name,
			Path:   m.Path,
			Labels: append([]string(nil), m.Labels...),
		})
	}

	return out
}
