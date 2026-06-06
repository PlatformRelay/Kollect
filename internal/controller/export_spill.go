// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
)

type spillGateResult struct {
	degraded bool
	reason   string
	message  string
}

//nolint:logcheck // export spill assessment uses named reconcile logger alongside ctx deadline
func assessExportSpill(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	payloadSize int64,
	maxBytes int64,
	sinkNamespace string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
) (spillGateResult, error) {
	spill := export.AssessSpill(payloadSize, maxBytes)
	if spill.ExceedsCap {
		return spillGateResult{
			degraded: true,
			reason:   spillReasonPayloadTooLarge,
			message:  fmt.Sprintf("export payload %d bytes exceeds cap %d", payloadSize, spill.Cap),
		}, nil
	}
	if spill.Warn {
		log.Info("export payload exceeds spill warn threshold",
			"bytes", payloadSize, "threshold", export.SpillWarnBytes)
	}
	if !spill.RequiresSpill {
		return spillGateResult{}, nil
	}

	hasObjectStore, err := hasObjectStoreSink(ctx, c, sinkNamespace, bindings)
	if err != nil {
		return spillGateResult{}, err
	}
	if hasObjectStore {
		return spillGateResult{}, nil
	}

	return spillGateResult{
		degraded: true,
		reason:   spillReasonSpillRequired,
		message: fmt.Sprintf(
			"export payload %d bytes requires object-store spill (configure s3 or gcs snapshot sink)",
			payloadSize,
		),
	}, nil
}

const (
	spillReasonPayloadTooLarge = "PayloadTooLarge"
	spillReasonSpillRequired   = "SpillRequired"
)

func recordSpillGateMetrics(gate spillGateResult) {
	if !gate.degraded {
		return
	}

	switch gate.reason {
	case spillReasonPayloadTooLarge:
		metrics.SinkErrorsTotal.WithLabelValues("payload_too_large").Inc()
	case spillReasonSpillRequired:
		metrics.SinkErrorsTotal.WithLabelValues("spill_required").Inc()
	}
}

func hasObjectStoreSink(
	ctx context.Context,
	c client.Client,
	sinkNamespace string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
) (bool, error) {
	for _, binding := range bindings {
		if binding.Family != kollectdevv1alpha1.SinkFamilySnapshot {
			continue
		}
		resolved, err := loadClusterInventorySink(ctx, c, sinkNamespace, binding)
		if err != nil {
			return false, err
		}
		if export.IsObjectStoreSinkType(resolved.Spec.Type) {
			return true, nil
		}
	}

	return false, nil
}
