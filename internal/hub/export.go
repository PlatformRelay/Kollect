// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsnapshotsinks;kollectdatabasesinks;kollecteventsinks,verbs=get;list;watch
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

	type sinkJob struct {
		name     string
		resolved *sink.ResolvedSink
	}

	jobs := make([]sinkJob, 0, len(e.Config.SinkRefs))
	for _, sinkName := range e.Config.SinkRefs {
		if e.coalesce.shouldSkip(report.Cluster, sinkName, report.Generation, checksum, interval, now) {
			metrics.ExportDebouncedTotal.WithLabelValues("KollectHub").Inc()
			continue
		}

		resolved, err := sink.ResolveSink(ctx, e.Client, sink.ResolveOptions{
			Namespace: e.Config.ExportNamespace,
			Name:      sinkName,
		})
		if err != nil {
			return fmt.Errorf("hub export: load sink %q: %w", sinkName, err)
		}

		jobs = append(jobs, sinkJob{name: sinkName, resolved: resolved})
	}

	if len(jobs) == 0 {
		return nil
	}

	envelope, err := export.MarshalEnvelope(items, export.Metadata{
		Generation: report.Generation,
		ExportedAt: now.UTC(),
		Cluster:    report.Cluster,
	})
	if err != nil {
		return fmt.Errorf("hub export: marshal envelope: %w", err)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, job := range jobs {
		wg.Add(1)

		go func(job sinkJob) {
			defer wg.Done()

			if err := sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
				Ctx:           ctx,
				Client:        e.Client,
				Registry:      e.Registry,
				SinkNamespace: sink.SinkNamespaceForResolved(job.resolved, e.Config.ExportNamespace),
				SinkName:      job.name,
				SinkUID:       job.resolved.UID,
				ObjectPath:    objectPath,
				Envelope:      envelope,
				SinkSpec:      job.resolved.Spec,
			}); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("sink %q: %w", job.name, err))
				mu.Unlock()

				return
			}

			e.coalesce.record(report.Cluster, job.name, report.Generation, checksum, now)
		}(job)
	}

	wg.Wait()

	return errors.Join(errs...)
}
