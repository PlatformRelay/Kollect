// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

func TestInProcessHubMergeRoundTrip(t *testing.T) {
	t.Parallel()

	bus := transport.NewInProcessBus()
	store := collect.NewStore()
	merger := hub.NewMerger(store)
	consumer := hub.NewConsumer(bus, merger, "inventory/reports", "test-hub", nil, hub.ConsumerOptions{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatal(err)
	}

	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-1",
		InventoryRef: hub.InventoryRef{
			Namespace: "default",
			Name:      "team-inventory",
		},
		Items: []collect.Item{
			{Namespace: "apps", Name: "web", UID: "uid-web", Version: "v1", Kind: "Deployment"},
		},
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Publish(ctx, "inventory/reports", payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for store.TotalCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if store.TotalCount() != 1 {
		t.Fatalf("store count = %d, want 1", store.TotalCount())
	}
}

func TestConsumerStartRequiresSubscriber(t *testing.T) {
	t.Parallel()

	var c hub.Consumer
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected error for nil consumer")
	}
}

func TestNewConsumerDefaultSubject(t *testing.T) {
	t.Parallel()

	bus := transport.NewInProcessBus()
	c := hub.NewConsumer(bus, hub.NewMerger(collect.NewStore()), "", "hub", nil, hub.ConsumerOptions{})
	if c.Subject != "inventory/reports" {
		t.Fatalf("subject = %q", c.Subject)
	}
}

// wireTestBus implements WireSubscriber with explicit wire cluster metadata for unit tests.
type wireTestBus struct {
	mu      sync.Mutex
	handler transport.WireHandler
}

func (b *wireTestBus) Subscribe(_ context.Context, _ string, handler transport.Handler) error {
	return b.SubscribeWire(context.Background(), "", func(ctx context.Context, wireCluster string, payload []byte) error {
		return handler(ctx, payload)
	})
}

func (b *wireTestBus) SubscribeWire(_ context.Context, _ string, handler transport.WireHandler) error {
	b.mu.Lock()
	b.handler = handler
	b.mu.Unlock()

	return nil
}

func (b *wireTestBus) PublishWire(ctx context.Context, wireCluster string, payload []byte) error {
	b.mu.Lock()
	handler := b.handler
	b.mu.Unlock()

	if handler == nil {
		return fmt.Errorf("wire test bus: no subscriber")
	}

	return handler(ctx, wireCluster, payload)
}

func TestConsumerRejectsTransportACL(t *testing.T) {
	t.Parallel()

	bus := &wireTestBus{}
	store := collect.NewStore()
	consumer := hub.NewConsumer(
		bus,
		hub.NewMerger(store),
		"inventory/reports",
		"hub",
		nil,
		hub.ConsumerOptions{TransportACL: transport.ACLSettings{AllowedClusterIDs: []string{"spoke-a"}}},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatal(err)
	}

	report := hub.SpokeReport{
		APIVersion:   "kollect.dev/v1alpha1",
		Cluster:      "rogue",
		InventoryRef: hub.InventoryRef{Namespace: "team-a", Name: "inv"},
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	err = bus.PublishWire(ctx, "rogue", payload)
	if err == nil {
		t.Fatal("expected ACL rejection for rogue wire cluster")
	}

	if !strings.Contains(err.Error(), "rogue") {
		t.Fatalf("ACL error = %q, want mention of rogue wire cluster", err)
	}

	if store.TotalCount() != 0 {
		t.Fatalf("store count = %d, want 0 after ACL rejection", store.TotalCount())
	}
}

func TestConsumerRejectsWireBodyClusterMismatch(t *testing.T) {
	t.Parallel()

	bus := &wireTestBus{}
	store := collect.NewStore()
	consumer := hub.NewConsumer(
		bus,
		hub.NewMerger(store),
		"inventory/reports",
		"hub",
		nil,
		hub.ConsumerOptions{TransportACL: transport.ACLSettings{AllowedClusterIDs: []string{"spoke-a", "rogue"}}},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatal(err)
	}

	report := hub.SpokeReport{
		APIVersion:   "kollect.dev/v1alpha1",
		Cluster:      "spoke-a",
		InventoryRef: hub.InventoryRef{Namespace: "team-a", Name: "inv"},
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	err = bus.PublishWire(ctx, "rogue", payload)
	if err == nil {
		t.Fatal("expected wire/body cluster mismatch error")
	}

	if !strings.Contains(err.Error(), "does not match report cluster") {
		t.Fatalf("mismatch error = %q", err)
	}

	if store.TotalCount() != 0 {
		t.Fatalf("store count = %d, want 0 after mismatch rejection", store.TotalCount())
	}
}

func TestConsumerMarksRemoteClusterConnected(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-a", Namespace: "platform"},
		Spec:       kollectdevv1alpha1.KollectRemoteClusterSpec{ClusterName: "spoke-a"},
	}
	statusClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(rc).WithObjects(rc).Build()

	bus := transport.NewInProcessBus()
	store := collect.NewStore()
	consumer := hub.NewConsumer(
		bus,
		hub.NewMerger(store),
		"inventory/reports",
		"hub",
		statusClient,
		hub.ConsumerOptions{},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatal(err)
	}

	report := hub.SpokeReport{
		APIVersion:   "kollect.dev/v1alpha1",
		Cluster:      "spoke-a",
		InventoryRef: hub.InventoryRef{Namespace: "team-a", Name: "inv"},
		Items: []collect.Item{{
			TargetNamespace: "team-a",
			TargetName:      "inv",
			Namespace:       "apps",
			Name:            "web",
			UID:             "uid-1",
			Version:         "v1",
			Kind:            "Deployment",
		}},
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Publish(ctx, "inventory/reports", payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var got kollectdevv1alpha1.KollectRemoteCluster
		if err := statusClient.Get(ctx, client.ObjectKeyFromObject(rc), &got); err == nil {
			for i := range got.Status.Conditions {
				if got.Status.Conditions[i].Type == kollectdevv1alpha1.ConditionConnected &&
					got.Status.Conditions[i].Status == metav1.ConditionTrue {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("expected Connected condition after consumer merge")
}
