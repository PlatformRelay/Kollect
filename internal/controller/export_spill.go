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

func assessExportSpill(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	payloadSize int64,
	maxBytes int64,
	sinkNamespace string,
	sinkRefs []string,
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
		metrics.ExportSpillWarnTotal.Inc()
	}
	if !spill.RequiresSpill {
		return spillGateResult{}, nil
	}

	hasObjectStore, err := hasObjectStoreSink(ctx, c, sinkNamespace, sinkRefs)
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
			"export payload %d bytes requires object-store spill (configure s3 or gcs sink)",
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
	sinkRefs []string,
) (bool, error) {
	for _, name := range sinkRefs {
		var ks kollectdevv1alpha1.KollectSink
		if err := c.Get(ctx, client.ObjectKey{Namespace: sinkNamespace, Name: name}, &ks); err != nil {
			return false, err
		}
		if export.IsObjectStoreSinkType(ks.Spec.Type) {
			return true, nil
		}
	}

	return false, nil
}
