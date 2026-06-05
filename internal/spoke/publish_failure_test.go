// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

type failAfterPublisher struct {
	calls    int
	failFrom int
}

func (p *failAfterPublisher) Publish(_ context.Context, _ string, _ []byte) error {
	p.calls++
	if p.calls >= p.failFrom {
		return errors.New("publish unavailable")
	}

	return nil
}

func TestTryPublishReport_publishFailureRetainsDelta(t *testing.T) {
	resetPublisherCache()
	resetPublishState()
	t.Cleanup(func() {
		resetPublisherCache()
		resetPublishState()
	})

	t.Setenv("KOLLECT_SPOKE_CLUSTER", "spoke-a")
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "inprocess")

	pub := &failAfterPublisher{failFrom: 2}
	testPublisher = pub

	store := collect.NewStore()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv", Generation: 1},
	}
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "t",
		Namespace:       "apps",
		Name:            "a",
		UID:             "uid-1",
		Version:         "v1",
		Kind:            "Deployment",
	})

	ctx := context.Background()
	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	inv.Generation = 2
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "t",
		Namespace:       "apps",
		Name:            "b",
		UID:             "uid-2",
		Version:         "v1",
		Kind:            "Deployment",
	})

	if err := TryPublishReport(ctx, store, inv); err == nil {
		t.Fatal("expected publish failure on generation bump")
	}

	// State must still reflect the last successful publish (gen 1), not the failed gen-2 attempt.
	stateMu.Lock()
	snap, ok := lastState[inventoryKey{namespace: "team-a", name: "inv"}]
	stateMu.Unlock()
	if !ok {
		t.Fatal("expected publish state after successful first publish")
	}
	if snap.generation != 1 {
		t.Fatalf("generation = %d, want 1 (failed publish must not advance state)", snap.generation)
	}
	if _, has := snap.items["uid-2"]; has {
		t.Fatal("failed publish must not record uid-2 in lastState")
	}

	pub.failFrom = 99
	if err := TryPublishReport(ctx, store, inv); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}

	stateMu.Lock()
	snap = lastState[inventoryKey{namespace: "team-a", name: "inv"}]
	stateMu.Unlock()
	if snap.generation != 2 {
		t.Fatalf("generation after retry = %d, want 2", snap.generation)
	}
	if _, has := snap.items["uid-2"]; !has {
		t.Fatal("successful retry must record uid-2 in lastState")
	}
}
