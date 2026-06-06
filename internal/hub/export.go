// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

package hub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink"
)

// Exporter fans out merged hub inventory to configured namespaced sinks in parallel.
type Exporter struct {
	Store    *collect.Store
	Client   client.Client
	Registry *sink.Registry
	Config   ExportConfig

	coalesce hubExportCoalesce
}

// ExportAfterMerge exports the merged target inventory to all configured sinks.
func (e *Exporter) ExportAfterMerge(ctx context.Context, report SpokeReport) error {
	if e == nil || !e.Config.ExportEnabled() {
		return nil
	}

	if e.Store == nil || e.Client == nil || e.Registry == nil {
		return fmt.Errorf("hub export: store, client, and registry are required")
	}

	targetNS := report.Cluster
	targetName := report.InventoryRef.Name
	if targetName == "" {
		targetName = defaultInventoryName
	}

	items := e.Store.SnapshotTarget(targetNS, targetName)
	checksum := checksumForItems(items, report)
	interval := e.Config.ExportMinInterval
	now := time.Now()

	invNS := report.InventoryRef.Namespace
	if invNS == "" {
		invNS = e.Config.ExportNamespace
	}

	invName := report.InventoryRef.Name
	if invName == "" {
		invName = defaultInventoryName
	}

	objectPath := fmt.Sprintf("inventory/%s/%s.json", invNS, invName)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, sinkName := range e.Config.SinkRefs {
		if e.coalesce.shouldSkip(report.Cluster, sinkName, report.Generation, checksum, interval, now) {
			metrics.ExportDebouncedTotal.WithLabelValues("KollectHub").Inc()
			continue
		}

		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			if err := sink.RunExportItems(sink.ExportItemsRequest{
				Ctx:           ctx,
				Client:        e.Client,
				Registry:      e.Registry,
				SinkNamespace: e.Config.ExportNamespace,
				SinkName:      name,
				ObjectPath:    objectPath,
				Items:         items,
				Meta: export.Metadata{
					Generation: report.Generation,
					Cluster:    report.Cluster,
				},
			}); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("sink %q: %w", name, err))
				mu.Unlock()

				return
			}

			e.coalesce.record(report.Cluster, name, report.Generation, checksum, now)
		}(sinkName)
	}

	wg.Wait()

	return errors.Join(errs...)
}
