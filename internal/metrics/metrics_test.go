// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	Register()

	InventoryItemsTotal.Set(3)
	if v := testutil.ToFloat64(InventoryItemsTotal); v != 3 {
		t.Fatalf("inventory items gauge: got %v", v)
	}

	CollectItemsTotal.Set(5)
	if v := testutil.ToFloat64(CollectItemsTotal); v != 5 {
		t.Fatalf("collect items gauge: got %v", v)
	}

	CollectedObjects.WithLabelValues("deployments", "apps/v1/Deployment").Set(2)
	if v := testutil.ToFloat64(CollectedObjects.WithLabelValues("deployments", "apps/v1/Deployment")); v != 2 {
		t.Fatalf("collected objects gauge: got %v", v)
	}

	ReconcileTotal.WithLabelValues("kollecttarget", ResultSuccess).Inc()
	if v := testutil.ToFloat64(ReconcileTotal.WithLabelValues("kollecttarget", ResultSuccess)); v < 1 {
		t.Fatalf("reconcile counter: got %v", v)
	}

	ReconcileErrorsTotal.WithLabelValues("KollectTarget", ErrorClassForbidden).Inc()
	if v := testutil.ToFloat64(ReconcileErrorsTotal.WithLabelValues("KollectTarget", ErrorClassForbidden)); v < 1 {
		t.Fatalf("reconcile errors counter: got %v", v)
	}

	ExportDurationSeconds.WithLabelValues("git").Observe(0.5)
	if count := testutil.CollectAndCount(ExportDurationSeconds); count != 1 {
		t.Fatalf("export duration histogram count: got %d", count)
	}

	SinkConnectionTestTotal.WithLabelValues("git", ResultSuccess).Inc()
	if v := testutil.ToFloat64(SinkConnectionTestTotal.WithLabelValues("git", ResultSuccess)); v < 1 {
		t.Fatalf("connection test counter: got %v", v)
	}

	ReconcileInFlight.WithLabelValues("kollecttarget").Inc()
	if v := testutil.ToFloat64(ReconcileInFlight.WithLabelValues("kollecttarget")); v != 1 {
		t.Fatalf("workqueue depth gauge: got %v", v)
	}
	ReconcileInFlight.WithLabelValues("kollecttarget").Dec()

	ReconcileDurationSeconds.WithLabelValues("kollectinventory").Observe(0.1)
	if count := testutil.CollectAndCount(ReconcileDurationSeconds); count != 1 {
		t.Fatalf("reconcile duration histogram count: got %d", count)
	}

	InformerObjects.WithLabelValues("apps", "v1", "deployments").Set(42)
	if v := testutil.ToFloat64(InformerObjects.WithLabelValues("apps", "v1", "deployments")); v != 42 {
		t.Fatalf("informer objects gauge: got %v", v)
	}

	ExportBytesTotal.WithLabelValues("git").Add(1024)
	if v := testutil.ToFloat64(ExportBytesTotal.WithLabelValues("git")); v != 1024 {
		t.Fatalf("export bytes counter: got %v", v)
	}

	RecordCustomResourceSeries("deployments", "apps/v1/Deployment", "ready_replicas", 3)
	series := CustomResourceSeries.WithLabelValues("deployments", "apps/v1/Deployment", "ready_replicas")
	if v := testutil.ToFloat64(series); v != 3 {
		t.Fatalf("custom resource series gauge: got %v", v)
	}

	HubSpokeReportsTotal.WithLabelValues("platform", ResultSuccess).Inc()
	if v := testutil.ToFloat64(HubSpokeReportsTotal.WithLabelValues("platform", ResultSuccess)); v < 1 {
		t.Fatalf("hub spoke reports counter: got %v", v)
	}

	SinkErrorsTotal.WithLabelValues("transient").Inc()
	if v := testutil.ToFloat64(SinkErrorsTotal.WithLabelValues("transient")); v < 1 {
		t.Fatalf("sink errors counter: got %v", v)
	}

	if len(Catalog) < 10 {
		t.Fatalf("metrics catalog too short: got %d entries", len(Catalog))
	}
}
