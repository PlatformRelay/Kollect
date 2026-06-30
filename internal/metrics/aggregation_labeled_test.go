// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCustomResourceLabeledSeries(t *testing.T) {
	t.Parallel()

	ResetCustomResourceLabeledSeries("team/profile", "apps/v1/Deployment")
	RecordCustomResourceLabeledSeries(
		"team/profile",
		"apps/v1/Deployment",
		"replicas",
		map[string]string{"namespace": "apps", "name": "web"},
		3,
	)

	if v, ok := CustomResourceLabeledSeriesValue(
		"team/profile",
		"apps/v1/Deployment",
		"replicas",
		map[string]string{"namespace": "apps", "name": "web"},
	); !ok || v != 3 {
		t.Fatalf("stored value = %v ok=%v", v, ok)
	}

	collector := customResourceLabeledCollector{}
	descCh := make(chan *prometheus.Desc, 2)
	collector.Describe(descCh)
	if len(descCh) != 1 {
		t.Fatalf("describe count = %d", len(descCh))
	}

	ResetCustomResourceLabeledSeries("team/profile", "apps/v1/Deployment")
	if _, ok := CustomResourceLabeledSeriesValue(
		"team/profile",
		"apps/v1/Deployment",
		"replicas",
		map[string]string{"namespace": "apps", "name": "web"},
	); ok {
		t.Fatal("expected reset to clear series")
	}
}

func TestMaxLabeledSeriesPerKeyGlobal(t *testing.T) {
	t.Cleanup(func() { SetMaxLabeledSeriesPerKeyGlobal(DefaultMaxLabeledSeriesPerKey) })

	if got := MaxLabeledSeriesPerKeyGlobal(); got != DefaultMaxLabeledSeriesPerKey {
		t.Fatalf("default cap = %d, want %d", got, DefaultMaxLabeledSeriesPerKey)
	}

	SetMaxLabeledSeriesPerKeyGlobal(5)
	if got := MaxLabeledSeriesPerKeyGlobal(); got != 5 {
		t.Fatalf("cap after Set(5) = %d, want 5", got)
	}

	// Non-positive values are ignored, not treated as "unlimited".
	SetMaxLabeledSeriesPerKeyGlobal(0)
	if got := MaxLabeledSeriesPerKeyGlobal(); got != 5 {
		t.Fatalf("cap after Set(0) = %d, want unchanged 5", got)
	}
	SetMaxLabeledSeriesPerKeyGlobal(-1)
	if got := MaxLabeledSeriesPerKeyGlobal(); got != 5 {
		t.Fatalf("cap after Set(-1) = %d, want unchanged 5", got)
	}
}
