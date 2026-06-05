// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/konih/kollect/internal/metrics"
)

func TestRecordTargetSnapshotMetrics(t *testing.T) {
	t.Parallel()

	metrics.Register()

	recordTargetSnapshotMetrics("deployments", "apps/v1/Deployment", []Item{
		{Attributes: map[string]any{"ready_replicas": 2, "name": "a"}},
		{Attributes: map[string]any{"ready_replicas": 1, "status": "ok"}},
	})

	count := metrics.CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "object_count")
	if v := testutil.ToFloat64(count); v != 2 {
		t.Fatalf("object_count = %v, want 2", v)
	}

	replicas := metrics.CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "ready_replicas")
	if v := testutil.ToFloat64(replicas); v != 3 {
		t.Fatalf("ready_replicas sum = %v, want 3", v)
	}

	recordTargetSnapshotMetrics("deployments", "apps/v1/Deployment", nil)
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
