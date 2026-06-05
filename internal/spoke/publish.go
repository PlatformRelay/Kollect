// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

const defaultSubject = "inventory/reports"

type inventoryKey struct {
	namespace string
	name      string
}

type publishSnapshot struct {
	generation int64
	items      map[string]collect.Item
}

var (
	stateMu   sync.Mutex
	lastState = make(map[inventoryKey]publishSnapshot)
)

// TryPublishReport publishes a summarized SpokeReport when spoke env is configured.
//
// No-op when KOLLECT_SPOKE_CLUSTER is unset. When set, publishes a delta JSON payload
// (changed/new items and removed UIDs) via the configured transport subject.
func TryPublishReport(
	ctx context.Context,
	store *collect.Store,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	cluster := os.Getenv("KOLLECT_SPOKE_CLUSTER")
	if cluster == "" {
		return nil
	}

	if os.Getenv("KOLLECT_TRANSPORT_TYPE") == "" {
		return nil
	}

	if store == nil || inv == nil {
		return fmt.Errorf("spoke publish: store and inventory are required")
	}

	cfg := transport.ConfigFromEnv()

	pub, err := publisherFor(cfg)
	if err != nil {
		return fmt.Errorf("spoke publish transport: %w", err)
	}

	key := inventoryKey{namespace: inv.Namespace, name: inv.Name}
	current := store.SnapshotNamespace(inv.Namespace)

	stateMu.Lock()
	prev, hasPrev := lastState[key]
	if hasPrev && prev.generation == inv.Generation && !snapshotChanged(prev.items, current) {
		stateMu.Unlock()

		return nil
	}

	items, removed := deltaItems(prev.items, current, hasPrev)
	lastState[key] = publishSnapshot{
		generation: inv.Generation,
		items:      indexByUID(current),
	}
	stateMu.Unlock()

	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    cluster,
		InventoryRef: hub.InventoryRef{
			Namespace: inv.Namespace,
			Name:      inv.Name,
		},
		Generation:  inv.Generation,
		Items:       items,
		RemovedUIDs: removed,
	}

	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("spoke publish marshal: %w", err)
	}

	subject := os.Getenv("KOLLECT_HUB_SUBJECT")
	if subject == "" {
		subject = defaultSubject
	}

	pubCtx := transport.WithWireClusterID(ctx, cluster)
	if err := pub.Publish(pubCtx, subject, payload); err != nil {
		return fmt.Errorf("spoke publish: %w", err)
	}

	return nil
}

func indexByUID(items []collect.Item) map[string]collect.Item {
	out := make(map[string]collect.Item, len(items))
	for _, item := range items {
		out[item.UID] = item
	}

	return out
}

func snapshotChanged(prev map[string]collect.Item, current []collect.Item) bool {
	if len(prev) != len(current) {
		return true
	}

	for _, item := range current {
		old, ok := prev[item.UID]
		if !ok || !itemEqual(old, item) {
			return true
		}
	}

	return false
}

func itemEqual(a, b collect.Item) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)

	return errA == nil && errB == nil && string(ab) == string(bb)
}

func deltaItems(
	prev map[string]collect.Item,
	current []collect.Item,
	hasPrev bool,
) (changed []collect.Item, removed []string) {
	currentByUID := indexByUID(current)

	if !hasPrev {
		return append([]collect.Item(nil), current...), nil
	}

	for uid, item := range currentByUID {
		old, ok := prev[uid]
		if !ok || !itemEqual(old, item) {
			changed = append(changed, item)
		}
	}

	for uid := range prev {
		if _, ok := currentByUID[uid]; !ok {
			removed = append(removed, uid)
		}
	}

	return changed, removed
}

// resetPublishState clears delta coalescing state (tests only).
func resetPublishState() {
	stateMu.Lock()
	lastState = make(map[inventoryKey]publishSnapshot)
	stateMu.Unlock()
}
