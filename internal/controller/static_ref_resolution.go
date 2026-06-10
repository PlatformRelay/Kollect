// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

// staticRefResult classifies a cluster-kind static-ref resolution error into an ADR-0208 metric
// result label. Transient errors return "" — they are tracked by the reconcile-error metric, not
// the bounded static-ref enum.
func staticRefResult(err error) string {
	switch {
	case err == nil:
		return metrics.StaticRefResultOK
	case apierrors.IsForbidden(err):
		return metrics.StaticRefResultForbidden
	case apierrors.IsNotFound(err):
		return metrics.StaticRefResultNotFound
	default:
		return ""
	}
}

// recordStaticRefResolution increments the ADR-0208 resolution counter for ok/not_found/forbidden
// outcomes. It is a no-op for transient errors so the counter stays a bounded RBAC/ref signal.
func recordStaticRefResolution(kind, refType string, err error) {
	if result := staticRefResult(err); result != "" {
		metrics.StaticRefResolutionTotal.WithLabelValues(kind, refType, result).Inc()
	}
}

// staticRefTypeForFamily maps a sink family to its ADR-0208 static-ref type label.
func staticRefTypeForFamily(family string) string {
	switch family {
	case kollectdevv1alpha1.SinkFamilySnapshot:
		return metrics.StaticRefTypeSnapshot
	case kollectdevv1alpha1.SinkFamilyDatabase:
		return metrics.StaticRefTypeDatabase
	case kollectdevv1alpha1.SinkFamilyEvent:
		return metrics.StaticRefTypeEvent
	default:
		return metrics.StaticRefTypeSnapshot
	}
}
