// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups=kollect.dev,resources=kollectsinks,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

package hub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink"
)

// Exporter fans out merged hub inventory to configured namespaced sinks in parallel.
type Exporter struct {
	Store    *collect.Store
	Client   client.Client
	Registry *sink.Registry
	Config   ExportConfig
}

// ExportAfterMerge exports the merged target inventory to all configured sinks.
// Reuses the inventory export contract (payload JSON + inventory object path).
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

	payload, err := e.Store.MarshalTargetJSON(targetNS, targetName)
	if err != nil {
		return fmt.Errorf("hub export: marshal target payload: %w", err)
	}

	if len(payload) == 0 || string(payload) == "[]" || string(payload) == "null" {
		return nil
	}

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
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	for _, sinkName := range e.Config.SinkRefs {
		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			if err := e.exportToSink(ctx, name, payload, objectPath); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(sinkName)
	}

	wg.Wait()

	return firstErr
}

func (e *Exporter) exportToSink(
	ctx context.Context,
	sinkName string,
	payload []byte,
	objectPath string,
) error {
	var ks kollectdevv1alpha1.KollectSink
	if err := e.Client.Get(ctx, client.ObjectKey{
		Namespace: e.Config.ExportNamespace,
		Name:      sinkName,
	}, &ks); err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("load KollectSink %q: %w", sinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(exportSinkErrorReason(err)).Inc()

		return err
	}

	buildCtx, err := sink.BuildContextFromSpec(ctx, e.Client, ks.Spec, e.Config.ExportNamespace)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(exportSinkErrorReason(err)).Inc()

		return err
	}

	backend, err := e.Registry.NewBackend(ks.Spec, buildCtx)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(exportSinkErrorReason(err)).Inc()

		return err
	}

	start := time.Now()
	err = backend.Export(ctx, payload, objectPath)
	elapsed := time.Since(start).Seconds()
	metrics.ExportDurationSeconds.WithLabelValues(ks.Spec.Type).Observe(elapsed)
	metrics.ExportBytesTotal.WithLabelValues(ks.Spec.Type).Add(float64(len(payload)))

	if err != nil {
		reason := exportSinkErrorReason(err)
		metrics.SinkErrorsTotal.WithLabelValues(reason).Inc()

		return kollecterrors.Transient(fmt.Errorf("export to %q: %w", sinkName, err))
	}

	return nil
}

func exportSinkErrorReason(err error) string {
	if err == nil {
		return "unknown"
	}

	switch kollecterrors.ClassOf(err) {
	case kollecterrors.ClassTerminal:
		return "terminal"
	case kollecterrors.ClassForbidden:
		return "forbidden"
	default:
		return "transient"
	}
}
