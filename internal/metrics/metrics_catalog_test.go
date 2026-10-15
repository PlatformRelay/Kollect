// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"regexp"
	"sort"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
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
	"kollect_export_spill_warn_total",
	"kollect_export_shard_warn_total",
	"kollect_export_debounced_total",
	"kollect_namespace_fingerprint_cache_total",
	"kollect_watch_map_list_errors_total",
	"kollect_collect_dispatch_duration_seconds",
	"kollect_collect_dispatch_queue_depth",
	"kollect_collect_dispatch_backpressure_total",
	"kollect_informer_resync_dispatches_total",
	"kollect_collect_namespace_mismatch_total",
	"kollect_informer_cluster_wide_scope",
	"kollect_custom_resource_series",
	"kollect_custom_resource_labeled_series",
	"kollect_custom_resource_labeled_series_capped_total",
	"kollect_static_ref_resolution_total",
	"kollect_access_cache_total",
	"kollect_cleanup_terminal_total",
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

// recordingRegisterer keeps every collector registerTo hands it, so the test sees exactly what
// Register() puts into the controller-runtime registry without touching that global.
type recordingRegisterer struct{ collectors []prometheus.Collector }

func (r *recordingRegisterer) Register(c prometheus.Collector) error {
	r.collectors = append(r.collectors, c)
	return nil
}

func (r *recordingRegisterer) MustRegister(cs ...prometheus.Collector) {
	for _, c := range cs {
		_ = r.Register(c)
	}
}

func (r *recordingRegisterer) Unregister(prometheus.Collector) bool { return false }

var descFQName = regexp.MustCompile(`^Desc\{fqName: "([^"]+)"`)

// TestRegisterMatchesRegisteredMetricNames closes the gap between the hand mirror above and
// Register(): the mirror (and the docs guard hack/test/consistency_metrics_test.sh, which scans
// literals) stays green when a collector is dropped from MustRegister; this test does not.
//
// Names come from Describe, not Gather: a *Vec with no observed label values gathers no family
// at all, so Gather on a fresh registry would miss most of the set. Describe emits each
// collector's Desc unconditionally. The one exception is customResourceLabeledCollector, whose
// descriptors are built per recorded series (empty until a profile emits one); its family name
// is the package constant, matched by type.
func TestRegisterMatchesRegisteredMetricNames(t *testing.T) {
	t.Parallel()

	var rec recordingRegisterer
	registerTo(&rec)

	got := map[string]struct{}{}
	for _, c := range rec.collectors {
		if _, ok := c.(customResourceLabeledCollector); ok {
			got[customResourceLabeledSeriesName] = struct{}{}
			continue
		}
		ch := make(chan *prometheus.Desc, 16)
		go func() { c.Describe(ch); close(ch) }()
		n := 0
		for d := range ch {
			m := descFQName.FindStringSubmatch(d.String())
			if m == nil {
				t.Errorf("cannot read fqName from %s", d)
				continue
			}
			got[m[1]] = struct{}{}
			n++
		}
		if n == 0 {
			t.Errorf("collector %T describes no metric", c)
		}
	}

	want := map[string]struct{}{}
	for _, name := range registeredMetricNames {
		want[name] = struct{}{}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("registeredMetricNames lists %q but Register() does not register it", name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("Register() registers %q but registeredMetricNames does not list it", name)
		}
	}
}
