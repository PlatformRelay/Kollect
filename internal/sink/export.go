// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/metrics"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
	"github.com/platformrelay/kollect/internal/sink/objectstore"
	"github.com/platformrelay/kollect/internal/validation"
)

// ErrSpillRequired is returned when a non-object-store backend is asked to
// deliver an envelope above the inline cap (SpillMandatoryBytes). There is no
// spill write path, so this is terminal: the export cannot succeed until the
// payload shrinks, an object-store sink is bound, or real spill ships. It is
// raised instead of silently returning success so the controller can surface
// Degraded/SpillRequired rather than recording a green export that wrote
// nothing (K-01).
var ErrSpillRequired = errors.New("export payload requires object-store spill")

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
	SinkFamily    string
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
	SinkUID       types.UID
	ObjectPath    string
	Envelope      []byte
	SinkSpec      kollectdevv1alpha1.KollectSinkSpec
	// PrunePlan accumulates the projected file paths of every part in a multipart git-layout export
	// so prune can run exactly once, against the union, on the final part. Nil for single-part and
	// non-git sinks (no-op).
	PrunePlan *PrunePlan
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

	resolved, err := ResolveSink(req.Ctx, req.Client, ResolveOptions{
		Namespace: req.SinkNamespace,
		Name:      req.SinkName,
		Family:    req.SinkFamily,
	})
	if err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("load sink %q: %w", req.SinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	return RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           req.Ctx,
		Client:        req.Client,
		Registry:      req.Registry,
		SinkNamespace: sinkNamespaceForExport(resolved, req.SinkNamespace),
		SinkName:      req.SinkName,
		SinkUID:       resolved.UID,
		ObjectPath:    req.ObjectPath,
		Envelope:      envelope,
		SinkSpec:      resolved.Spec,
	})
}

// RunExportEnvelope exports a pre-built envelope without re-marshalling items.
func RunExportEnvelope(req ExportEnvelopeRequest) error {
	if req.Registry == nil {
		return kollecterrors.Terminal(fmt.Errorf("sink registry is not configured"))
	}

	if req.SinkSpec.Type == "" {
		return kollecterrors.Terminal(fmt.Errorf("sink spec is required for export to %q", req.SinkName))
	}

	backend, release, err := acquireBackend(
		req.Ctx, req.Client, req.Registry, req.SinkNamespace, req.SinkName, req.SinkUID, req.SinkSpec,
	)
	if err != nil {
		err = kollecterrors.ClassifyAPI(fmt.Errorf("acquire backend for %q: %w", req.SinkName, err))
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

		// Preserve the original envelope's header when re-marshalling the
		// capability-redacted item set: generation, cluster, and the REL-02
		// completeness marker (partIndex/partTotal) must survive redaction, or a
		// redacting sink would emit an unattributed part and break torn-set
		// detection (and, via generation, the object-path derivation below).
		orig := export.EnvelopeMetaFromPayload(envelope)
		preserved := export.Metadata{
			Generation: orig.Generation,
			Cluster:    orig.Cluster,
			ExportedAt: orig.ExportedAt,
			PartIndex:  orig.PartIndex,
			PartTotal:  orig.PartTotal,
		}
		if preserved.ExportedAt.IsZero() {
			preserved.ExportedAt = time.Now().UTC()
		}

		envelope, err = export.MarshalEnvelope(exportItems, preserved)
		if err != nil {
			err = kollecterrors.Terminal(err)
			metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

			return err
		}
	}

	if !shouldExportForSpill(backend.Capabilities(), int64(len(envelope))) {
		// Above the inline cap with no object-store spill path: fail loudly. The
		// previous `return nil` recorded Exported/Ready while writing nothing and
		// suppressed re-export for the interval — silent data loss with green
		// telemetry (K-01).
		err = kollecterrors.Terminal(fmt.Errorf("%w: %q is %d bytes (inline cap %d)",
			ErrSpillRequired, req.SinkName, len(envelope), export.SpillMandatoryBytes))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	invNS, invName := objectstore.InventoryFromObjectPath(req.ObjectPath)
	generation := export.GenerationFromEnvelope(envelope)
	defaultObjectPath := objectstore.ObjectPath(req.SinkSpec, invNS, invName, generation)

	plan, err := resolveSnapshotExport(backend, req.SinkSpec, envelope, invNS, invName, generation, defaultObjectPath, req.PrunePlan)
	if err != nil {
		err = kollecterrors.Terminal(fmt.Errorf("resolve layout for %q: %w", req.SinkName, err))
		metrics.SinkErrorsTotal.WithLabelValues(ExportErrorReason(err)).Inc()

		return err
	}

	commitCtx := git.CommitContextFromExport(
		envelope, plan.objectPath, strings.TrimSpace(req.SinkSpec.Cluster), req.SinkName,
	)
	exportCtx := git.WithCommitContext(req.Ctx, commitCtx)

	start := time.Now()
	err = exportThroughBreaker(req.SinkNamespace+"/"+req.SinkName, func() error {
		return plan.run(exportCtx)
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

func sinkNamespaceForExport(resolved *ResolvedSink, fallback string) string {
	return SinkNamespaceForResolved(resolved, fallback)
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

	// Spill-required is terminal but deserves its own label: operators are told
	// to alert on kollect_sink_errors_total{reason="spill_required"} (K-01).
	if errors.Is(err, ErrSpillRequired) {
		return "spill_required"
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
