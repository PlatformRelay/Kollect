// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

// PublishInventoryDeletion sends a final delta report with all previously published UIDs removed.
// No-op when spoke publish env is unset.
func PublishInventoryDeletion(
	ctx context.Context,
	store *collect.Store,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	cluster := os.Getenv("KOLLECT_SPOKE_CLUSTER")
	if cluster == "" || os.Getenv("KOLLECT_TRANSPORT_TYPE") == "" {
		clearPublishState(inv)
		return nil
	}

	if store == nil || inv == nil {
		return fmt.Errorf("spoke publish deletion: store and inventory are required")
	}

	key := inventoryKey{namespace: inv.Namespace, name: inv.Name}

	stateMu.Lock()
	prev, hasPrev := lastState[key]
	delete(lastState, key)
	stateMu.Unlock()

	if !hasPrev || len(prev.items) == 0 {
		return nil
	}

	removed := make([]string, 0, len(prev.items))
	for uid := range prev.items {
		removed = append(removed, uid)
	}

	report := hub.SpokeReport{
		APIVersion:    export.WireAPIVersion,
		SchemaVersion: export.SchemaVersion,
		Cluster:       cluster,
		InventoryRef: hub.InventoryRef{
			Namespace: inv.Namespace,
			Name:      inv.Name,
		},
		Generation:  inv.Generation,
		RemovedUIDs: removed,
	}

	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("spoke publish deletion marshal: %w", err)
	}

	cfg := transport.ConfigFromEnv()

	pub, err := publisherFor(cfg)
	if err != nil {
		return fmt.Errorf("spoke publish deletion transport: %w", err)
	}

	subject := os.Getenv("KOLLECT_HUB_SUBJECT")
	if subject == "" {
		subject = defaultSubject
	}

	pubCtx := transport.WithWireClusterID(ctx, cluster)
	if err := pub.Publish(pubCtx, subject, payload); err != nil {
		return fmt.Errorf("spoke publish deletion: %w", err)
	}

	return nil
}

func clearPublishState(inv *kollectdevv1alpha1.KollectInventory) {
	if inv == nil {
		return
	}

	key := inventoryKey{namespace: inv.Namespace, name: inv.Name}

	stateMu.Lock()
	delete(lastState, key)
	stateMu.Unlock()
}
