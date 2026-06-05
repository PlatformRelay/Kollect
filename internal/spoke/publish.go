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
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

const defaultSubject = "inventory/reports"

// TryPublishReport publishes a summarized SpokeReport when spoke env is configured.
//
// Phase 2 stub: no-op when KOLLECT_SPOKE_CLUSTER is unset. When set, publishes JSON to the
// configured transport subject (see ADR-0022 hub-and-spoke path). Full delta coalescing and
// cross-cluster auth are follow-up work.
func TryPublishReport(
	ctx context.Context,
	store *collect.Store,
	inv *kollectdevv1alpha1.KollectInventory,
) error {
	cluster := os.Getenv("KOLLECT_SPOKE_CLUSTER")
	if cluster == "" {
		return nil
	}

	if store == nil || inv == nil {
		return fmt.Errorf("spoke publish: store and inventory are required")
	}

	transportType := os.Getenv("KOLLECT_TRANSPORT_TYPE")
	if transportType == "" {
		transportType = string(transport.TypeInProcess)
	}

	cfg := transport.Config{
		Type:   transport.Type(transportType),
		Stream: defaultSubject,
	}
	if cfg.Type == transport.TypeRedis {
		cfg.Redis.URL = os.Getenv("KOLLECT_REDIS_URL")
	}

	pub, _, err := transport.NewTransport(cfg)
	if err != nil {
		return fmt.Errorf("spoke publish transport: %w", err)
	}

	report := hub.SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    cluster,
		InventoryRef: hub.InventoryRef{
			Namespace: inv.Namespace,
			Name:      inv.Name,
		},
		Generation: inv.Generation,
		Items:      store.SnapshotNamespace(inv.Namespace),
	}

	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("spoke publish marshal: %w", err)
	}

	subject := os.Getenv("KOLLECT_HUB_SUBJECT")
	if subject == "" {
		subject = defaultSubject
	}

	if err := pub.Publish(ctx, subject, payload); err != nil {
		return fmt.Errorf("spoke publish: %w", err)
	}

	return nil
}
