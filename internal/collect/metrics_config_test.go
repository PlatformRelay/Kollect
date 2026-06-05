// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

func TestMetricPathsFromProfile(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectProfileSpec{
		Metrics: []kollectdevv1alpha1.MetricSpec{
			{Name: "ready_total", Path: "ready_replicas"},
		},
	}

	got := MetricPathsFromProfile(spec)
	if len(got) != 1 || got[0].Name != "ready_total" || got[0].Path != "ready_replicas" {
		t.Fatalf("MetricPathsFromProfile() = %#v, want one ready_total path", got)
	}
}

func TestRecordTargetSnapshotMetricsConfiguredPaths(t *testing.T) {
	ensureMetricsRegistered()

	paths := []metrics.MetricPathSpec{{Name: "ready_total", Path: "ready_replicas"}}
	recordTargetSnapshotMetrics("deployments", "apps/v1/Deployment", []Item{
		{Attributes: map[string]any{"ready_replicas": 2}},
		{Attributes: map[string]any{"ready_replicas": 1, "ignored": 99}},
	}, paths)

	ready := metrics.CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "ready_total")
	if v := testutil.ToFloat64(ready); v != 3 {
		t.Fatalf("ready_total = %v, want 3", v)
	}

	ignored := metrics.CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "ignored")
	if v := testutil.ToFloat64(ignored); v != 0 {
		t.Fatalf("configured paths should not auto-emit ignored attribute, got %v", v)
	}
}
