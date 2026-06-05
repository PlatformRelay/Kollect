// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import "testing"

func TestMetricPathSpecStub(t *testing.T) {
	t.Parallel()

	spec := MetricPathSpec{
		Name: "not_after_days",
		Path: `status.notAfter`,
	}
	if spec.Name == "" || spec.Path == "" || len(spec.Labels) != 0 {
		t.Fatal("metric path spec stub should carry name and path with no labels yet")
	}
}
