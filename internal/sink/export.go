// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink/cap"
)

// Capabilities describes sink backend projection behavior (ADR-0401, ADR-0406).
type Capabilities = cap.Capabilities

// SnapshotStoreCapabilities is the default for Git, S3, GCS, and similar backends.
func SnapshotStoreCapabilities() Capabilities { return cap.SnapshotStore() }

// StreamEmitterCapabilities is the default for Kafka and NATS event sinks.
func StreamEmitterCapabilities() Capabilities { return cap.StreamEmitter() }

// RelationalStoreCapabilities is the default for Postgres upsert sinks.
func RelationalStoreCapabilities() Capabilities { return cap.RelationalStore() }

// ExportPayload decides whether to call Backend.Export for the given payload.
func ExportPayload(c Capabilities, payload []byte) (export []byte, skip bool) {
	return cap.ExportPayload(c, payload)
}

// ExportItemsRequest carries one inventory export fan-out attempt to a sink.
type ExportItemsRequest struct {
	Ctx           context.Context
	Client        client.Client
	Registry      *Registry
	SinkNamespace string
	SinkName      string
	ObjectPath    string
	Items         []collect.Item
	Meta          export.Metadata
}

// RunExportItems loads the sink, applies capability gating, wraps the envelope, and exports.
func RunExportItems(req ExportItemsRequest) error {
	if req.Registry == nil {
		return kollecterrors.Terminal(fmt.Errorf("sink registry is not configured"))
	}

	var ks kollectdevv1alpha1.KollectSink
	if err := req.Client.Get(req.Ctx, client.ObjectKey{
		Namespace: req.SinkNamespace,
		Name:      req.SinkName,
	}, &ks); err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("load KollectSink %q: %w", req.SinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	buildCtx, err := BuildContextFromSpec(req.Ctx, req.Client, ks.Spec, req.SinkNamespace)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	backend, err := req.Registry.NewBackend(ks.Spec, buildCtx)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	items := req.Items
	if items == nil {
		items = []collect.Item{}
	}

	itemsJSON, err := json.Marshal(items)
	if err != nil {
		err = kollecterrors.Terminal(fmt.Errorf("marshal export items: %w", err))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	exportItemsJSON, skip := ExportPayload(backend.Capabilities(), itemsJSON)
	if skip {
		return nil
	}

	var exportItems []collect.Item
	if unmarshalErr := json.Unmarshal(exportItemsJSON, &exportItems); unmarshalErr != nil {
		err = kollecterrors.Terminal(fmt.Errorf("decode export items: %w", unmarshalErr))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	meta := req.Meta
	if meta.ExportedAt.IsZero() {
		meta.ExportedAt = time.Now().UTC()
	}

	envelope, err := export.MarshalEnvelope(exportItems, meta)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	start := time.Now()
	err = backend.Export(req.Ctx, envelope, req.ObjectPath)
	elapsed := time.Since(start).Seconds()
	metrics.ExportDurationSeconds.WithLabelValues(ks.Spec.Type).Observe(elapsed)
	metrics.ExportBytesTotal.WithLabelValues(ks.Spec.Type).Add(float64(len(envelope)))

	if err != nil {
		reason := ExportErrorReason(err)
		metrics.SinkErrorsTotal.WithLabelValues(reason).Inc()

		return kollecterrors.Transient(fmt.Errorf("export to %q: %w", req.SinkName, err))
	}

	return nil
}

// ExportErrorReason maps classified errors to sink error metric labels.
func ExportErrorReason(err error) string {
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
