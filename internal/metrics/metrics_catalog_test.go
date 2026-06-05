// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"sort"
	"testing"
)

// registeredMetricNames mirrors metrics.go Register() — update both when adding metrics.
var registeredMetricNames = []string{
	"kollect_inventory_items_total",
	"kollect_collect_items_total",
	"kollect_collected_objects",
	"kollect_reconcile_total",
	"kollect_reconcile_errors_total",
	"kollect_sink_errors_total",
	"kollect_export_duration_seconds",
	"kollect_sink_connection_test_total",
	"kollect_workqueue_depth",
	"kollect_reconcile_duration_seconds",
	"kollect_informer_objects",
	"kollect_export_bytes_total",
	"kollect_custom_resource_series",
	"kollect_hub_spoke_reports_total",
}

func TestCatalogMatchesRegisteredMetrics(t *testing.T) {
	t.Parallel()

	catalogNames := make([]string, 0, len(Catalog))
	for _, e := range Catalog {
		catalogNames = append(catalogNames, e.Name)
	}
	sort.Strings(catalogNames)

	want := append([]string(nil), registeredMetricNames...)
	sort.Strings(want)

	if len(catalogNames) != len(want) {
		t.Fatalf("catalog size %d != registered %d\ncatalog: %v\nregistered: %v",
			len(catalogNames), len(want), catalogNames, want)
	}
	for i := range want {
		if catalogNames[i] != want[i] {
			t.Fatalf("catalog drift at index %d: catalog=%q registered=%q", i, catalogNames[i], want[i])
		}
	}
}
