// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/validation"
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

	conditionExportShardWarn = "ExportShardWarning"
	reasonExportShardWarn    = "ApproachingExportCap"
)

func noteExportShardWarning(conditions *[]metav1.Condition, generation int64, itemCount int) bool {
	if itemCount < validation.ExportShardWarnRows {
		apimeta.RemoveStatusCondition(conditions, conditionExportShardWarn)

		return false
	}

	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionExportShardWarn,
		Status:             metav1.ConditionTrue,
		Reason:             reasonExportShardWarn,
		Message:            fmt.Sprintf("%d collected rows in namespace — split into multiple KollectInventory resources (<~%d rows each)", itemCount, validation.ExportShardWarnRows),
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})

	return true
}

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
