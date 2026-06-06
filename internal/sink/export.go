// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/git"
	"github.com/konih/kollect/internal/sink/objectstore"
	"github.com/konih/kollect/internal/validation"
)

// Capabilities describes sink backend projection behavior (ADR-0401, ADR-0406).
type Capabilities = cap.Capabilities

// SnapshotStoreCapabilities is the default for Git and similar snapshot backends.
func SnapshotStoreCapabilities() Capabilities { return cap.SnapshotStore() }

// ObjectStoreSnapshotCapabilities is the default for S3/GCS spill-capable backends.
func ObjectStoreSnapshotCapabilities() Capabilities { return cap.ObjectStoreSnapshot() }

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

// ExportEnvelopeRequest carries a pre-marshalled export envelope to a sink.
type ExportEnvelopeRequest struct {
	Ctx           context.Context
	Client        client.Client
	Registry      *Registry
	SinkNamespace string
	SinkName      string
	ObjectPath    string
	Envelope      []byte
	SinkSpec      kollectdevv1alpha1.KollectSinkSpec
}

// RunExportItems loads the sink, applies capability gating, wraps the envelope, and exports.
func RunExportItems(req ExportItemsRequest) error {
	if req.Registry == nil {
		return kollecterrors.Terminal(fmt.Errorf("sink registry is not configured"))
	}

	items := req.Items
	if items == nil {
		items = []collect.Item{}
	}

	meta := req.Meta
	if meta.ExportedAt.IsZero() {
		meta.ExportedAt = time.Now().UTC()
	}

	envelope, err := export.MarshalEnvelope(items, meta)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
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

	return RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           req.Ctx,
		Client:        req.Client,
		Registry:      req.Registry,
		SinkNamespace: req.SinkNamespace,
		SinkName:      req.SinkName,
		ObjectPath:    req.ObjectPath,
		Envelope:      envelope,
		SinkSpec:      ks.Spec,
	})
}

// RunExportEnvelope exports a pre-built envelope without re-marshalling items.
func RunExportEnvelope(req ExportEnvelopeRequest) error {
	if req.Registry == nil {
		return kollecterrors.Terminal(fmt.Errorf("sink registry is not configured"))
	}

	backend, release, err := acquireBackend(req.Ctx, req.Client, req.Registry, req.SinkNamespace, req.SinkName)
	if err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("load KollectSink %q: %w", req.SinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}
	defer release()

	envelope := req.Envelope
	itemsJSON, err := export.ItemsJSONFromEnvelope(envelope)
	if err != nil {
		err = kollecterrors.Terminal(err)
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	exportItemsJSON, skip := ExportPayload(backend.Capabilities(), itemsJSON)
	if skip {
		return nil
	}

	if len(exportItemsJSON) != len(itemsJSON) {
		var exportItems []collect.Item
		if unmarshalErr := json.Unmarshal(exportItemsJSON, &exportItems); unmarshalErr != nil {
			err = kollecterrors.Terminal(fmt.Errorf("decode export items: %w", unmarshalErr))
			metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

			return err
		}

		envelope, err = export.MarshalEnvelope(exportItems, export.Metadata{ExportedAt: time.Now().UTC()})
		if err != nil {
			err = kollecterrors.Terminal(err)
			metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

			return err
		}
	}

	if !shouldExportForSpill(backend.Capabilities(), int64(len(envelope))) {
		return nil
	}

	invNS, invName := objectstore.InventoryFromObjectPath(req.ObjectPath)
	objectPath := objectstore.ObjectPath(req.SinkSpec, invNS, invName, export.GenerationFromEnvelope(envelope))

	start := time.Now()
	err = exportThroughBreaker(req.SinkNamespace+"/"+req.SinkName, func() error {
		return backend.Export(req.Ctx, envelope, objectPath)
	})
	elapsed := time.Since(start).Seconds()
	metrics.ExportDurationSeconds.WithLabelValues(req.SinkSpec.Type).Observe(elapsed)
	metrics.ExportBytesTotal.WithLabelValues(req.SinkSpec.Type).Add(float64(len(envelope)))

	if err != nil {
		err = git.ClassifyExportError(err)
		reason := ExportErrorReason(err)
		metrics.SinkErrorsTotal.WithLabelValues(reason).Inc()

		return classifyExportFailure(req.SinkName, err)
	}

	return nil
}

func classifyExportFailure(sinkName string, err error) error {
	if kollecterrors.IsTerminal(err) {
		return fmt.Errorf("export to %q: %w", sinkName, err)
	}

	return kollecterrors.Transient(fmt.Errorf("export to %q: %w", sinkName, err))
}

func closeBackend(b Backend) error {
	switch c := b.(type) {
	case io.Closer:
		if err := c.Close(); err != nil {
			return fmt.Errorf("close sink backend: %w", err)
		}
	case interface{ Close() }:
		c.Close()
	}

	return nil
}

func shouldExportForSpill(c cap.Capabilities, payloadSize int64) bool {
	spill := export.AssessSpill(payloadSize, validation.MaxExportBytesGlobal())

	return !spill.RequiresSpill || c.ObjectStore
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
