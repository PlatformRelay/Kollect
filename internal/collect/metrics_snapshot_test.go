// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/konih/kollect/internal/metrics"
)

func TestRecordTargetSnapshotMetrics(t *testing.T) {
	ensureMetricsRegistered()

	recordTargetSnapshotMetrics("deployments", "apps/v1/Deployment", []Item{
		{Attributes: map[string]any{"ready_replicas": 2, "name": "a"}},
		{Attributes: map[string]any{"ready_replicas": 1, "status": "ok"}},
	}, nil)

	count := metrics.CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "object_count")
	if v := testutil.ToFloat64(count); v != 2 {
		t.Fatalf("object_count = %v, want 2", v)
	}

	replicas := metrics.CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "ready_replicas")
	if v := testutil.ToFloat64(replicas); v != 3 {
		t.Fatalf("ready_replicas sum = %v, want 3", v)
	}

	recordTargetSnapshotMetrics("deployments", "apps/v1/Deployment", nil, nil)
	if v := testutil.ToFloat64(count); v != 0 {
		t.Fatalf("empty snapshot object_count = %v, want 0", v)
	}
}

func TestNumericAttribute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   any
		want float64
		ok   bool
	}{
		{in: 3, want: 3, ok: true},
		{in: int64(5), want: 5, ok: true},
		{in: float32(1.5), want: 1.5, ok: true},
		{in: "2.25", want: 2.25, ok: true},
		{in: json.Number("3.5"), want: 3.5, ok: true},
		{in: "not-a-number", ok: false},
		{in: true, ok: false},
	}

	for _, tc := range cases {
		got, ok := numericAttribute(tc.in)
		if ok != tc.ok {
			t.Fatalf("numericAttribute(%#v) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Fatalf("numericAttribute(%#v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// EC-P2-09: KollectProfile.spec.metrics[].labels takes raw attribute values as
// Prometheus label values with no bound, so a high-cardinality attribute (UIDs,
// timestamps, free text) can explode kollect_custom_resource_labeled_series.
// The guard must (a) cap distinct tuples per (profile,gvk,series) and (b) survive
// the *same* tuples on every snapshot refresh — a non-deterministic cap would make
// graphed series flap every scrape, which is worse than no cap.
func TestRecordLabeledMetricSeries_CapsCardinalityDeterministically(t *testing.T) {
	ensureMetricsRegistered()

	metrics.SetMaxLabeledSeriesPerKeyGlobal(2)
	t.Cleanup(func() { metrics.SetMaxLabeledSeriesPerKeyGlobal(metrics.DefaultMaxLabeledSeriesPerKey) })

	const profile, gvk, series = "card-test-profile", "apps/v1/CardTest", "size"
	metrics.ResetCustomResourceLabeledSeries(profile, gvk)

	items := []Item{
		{Attributes: map[string]any{"size": 1, "name": "a"}},
		{Attributes: map[string]any{"size": 2, "name": "b"}},
		{Attributes: map[string]any{"size": 3, "name": "c"}},
	}
	spec := metrics.MetricPathSpec{Name: series, Path: "size", Labels: []string{"name"}}

	recordLabeledMetricSeries(profile, gvk, spec, items)

	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "a"}); !ok {
		t.Fatal("expected tuple {name=a} to survive the cap (lexicographically first)")
	}
	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "b"}); !ok {
		t.Fatal("expected tuple {name=b} to survive the cap (lexicographically second)")
	}
	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "c"}); ok {
		t.Fatal("expected tuple {name=c} to be dropped by the cap")
	}

	capped := testutil.ToFloat64(metrics.LabeledSeriesCardinalityCappedTotal.WithLabelValues(profile, gvk, series))
	if capped != 1 {
		t.Fatalf("capped counter = %v, want 1", capped)
	}

	// Re-run as if a second snapshot refresh happened; the survivors must be
	// identical (no flapping), not a different random N from map iteration.
	metrics.ResetCustomResourceLabeledSeries(profile, gvk)
	recordLabeledMetricSeries(profile, gvk, spec, items)

	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "a"}); !ok {
		t.Fatal("second refresh: expected tuple {name=a} to survive again")
	}
	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "b"}); !ok {
		t.Fatal("second refresh: expected tuple {name=b} to survive again")
	}
}

// Below the cap, nothing is dropped and the capped counter does not increment.
func TestRecordLabeledMetricSeries_NoCapWhenUnderLimit(t *testing.T) {
	ensureMetricsRegistered()

	const profile, gvk, series = "card-test-profile-under", "apps/v1/CardTestUnder", "size"
	metrics.ResetCustomResourceLabeledSeries(profile, gvk)

	items := []Item{
		{Attributes: map[string]any{"size": 1, "name": "a"}},
		{Attributes: map[string]any{"size": 2, "name": "b"}},
	}
	spec := metrics.MetricPathSpec{Name: series, Path: "size", Labels: []string{"name"}}

	recordLabeledMetricSeries(profile, gvk, spec, items)

	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "a"}); !ok {
		t.Fatal("expected tuple {name=a} present")
	}
	if _, ok := metrics.CustomResourceLabeledSeriesValue(profile, gvk, series, map[string]string{"name": "b"}); !ok {
		t.Fatal("expected tuple {name=b} present")
	}

	capped := testutil.ToFloat64(metrics.LabeledSeriesCardinalityCappedTotal.WithLabelValues(profile, gvk, series))
	if capped != 0 {
		t.Fatalf("capped counter = %v, want 0 (under limit)", capped)
	}
}

func TestAttributeLabelValue(t *testing.T) {
	t.Parallel()

	if got := attributeLabelValue(nil); got != "" {
		t.Fatalf("nil = %q", got)
	}
	if got := attributeLabelValue(42); got != "42" {
		t.Fatalf("int = %q", got)
	}
}
