// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/transport"
)

func TestTryPublishReportInProcessDelta(t *testing.T) {
	resetPublisherCache()
	resetPublishState()
	t.Cleanup(func() {
		resetPublisherCache()
		resetPublishState()
	})

	bus := transport.NewInProcessBus()
	testPublisher = bus

	t.Setenv("KOLLECT_SPOKE_CLUSTER", "spoke-a")
	t.Setenv("KOLLECT_TRANSPORT_TYPE", string(transport.TypeInProcess))

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "inv",
		Namespace:       "apps",
		Name:            "web",
		UID:             "uid-1",
		Version:         "v1",
		Kind:            "Deployment",
	})

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv", Generation: 1},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("duplicate publish: %v", err)
	}

	store.Remove("team-a", "inv", "uid-1")
	inv.Generation = 2
	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("publish after remove: %v", err)
	}

	_ = bus
	_ = json.Marshal
}

func TestTryPublishReportRequiresStore(t *testing.T) {
	t.Setenv("KOLLECT_SPOKE_CLUSTER", "spoke-a")
	t.Setenv("KOLLECT_TRANSPORT_TYPE", string(transport.TypeInProcess))

	if err := TryPublishReport(context.Background(), nil, &kollectdevv1alpha1.KollectInventory{}); err == nil {
		t.Fatal("expected error for nil store")
	}
}
