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
