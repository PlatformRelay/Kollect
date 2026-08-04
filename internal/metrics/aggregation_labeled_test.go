// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

func TestCustomResourceLabeledCollectorCollect(t *testing.T) {
	const (
		profile = "cov90s19/labeled-collect"
		gvk     = "apps/v1/Deployment"
		series  = "replicas"
	)
	labels := map[string]string{"namespace": "apps", "name": "web"}

	ResetCustomResourceLabeledSeries(profile, gvk)
	t.Cleanup(func() { ResetCustomResourceLabeledSeries(profile, gvk) })

	RecordCustomResourceLabeledSeries(profile, gvk, series, labels, 7)

	ch := make(chan prometheus.Metric, 16)
	customResourceLabeledCollector{}.Collect(ch)
	close(ch)

	var found bool
	for m := range ch {
		pb := &dto.Metric{}
		if err := m.Write(pb); err != nil {
			t.Fatalf("metric.Write: %v", err)
		}
		if pb.Gauge == nil || pb.Gauge.GetValue() != 7 {
			continue
		}
		got := make(map[string]string, len(pb.Label))
		for _, lp := range pb.Label {
			got[lp.GetName()] = lp.GetValue()
		}
		if got["profile"] == profile &&
			got["gvk"] == gvk &&
			got["series"] == series &&
			got["namespace"] == "apps" &&
			got["name"] == "web" {
			found = true

			break
		}
	}
	if !found {
		t.Fatal("Collect did not emit the recorded labeled series gauge")
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
