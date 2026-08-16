// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

const (
	conditionReady         = kollectdevv1alpha1.ConditionReady
	conditionDegraded      = kollectdevv1alpha1.ConditionDegraded
	conditionSinkReachable = kollectdevv1alpha1.ConditionSinkReachable
	conditionSynced        = kollectdevv1alpha1.ConditionSynced

	reasonSinkNotFound    = "SinkNotFound"
	reasonSinkUnreachable = "SinkUnreachable"
	reasonSinksReachable  = "SinksReachable"
	reasonExportFailed    = "ExportFailed"
	reasonProgressing     = "Progressing"
	reasonCleanupTerminal = "CleanupTerminal"

	// ADR-0208 static-ref resolution reasons (forbidden classification on cross-namespace refs).
	reasonProfileNotFound     = "ProfileNotFound"
	reasonProfileForbidden    = "ProfileForbidden"
	reasonSinkForbidden       = "SinkForbidden"
	reasonSinkNamespaceDenied = "SinkNamespaceDenied"

	// reasonScopeForbidden marks a degraded (not hard-failed) scope: RBAC denied
	// list access for one or more scoped namespaces (GUIDELINES.md §1 ErrForbidden).
	reasonScopeForbidden = "ScopeForbidden"

	// reasonExtractionFailed marks a hard-failed (Degraded) target: one or more resources
	// failed CEL/JSONPath attribute extraction (GUIDELINES.md §1 ErrTerminal — EC-P1-05).
	reasonExtractionFailed = "ExtractionFailed"

	// reasonCollecting is the Ready/Synced reason while a target is successfully collecting.
	reasonCollecting = "Collecting"
)

// setTargetCondition writes conditionType into conditions and persists the whole status
// subresource, skipping the API call when nothing about the condition moved.
//
// It reports whether it issued that call. Status carries fields no condition describes
// (KollectTarget.status.collectedCount), and a caller that changed one of those is
// responsible for persisting it when this skipped the write — otherwise the value stays
// in memory and the API server keeps serving the previous one (PERF-FIX-05 / F-05).
func setTargetCondition(
	ctx context.Context,
	c client.Client,
	target client.Object,
	generation int64,
	conditions *[]metav1.Condition,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) (written bool, err error) {
	existing := apimeta.FindStatusCondition(*conditions, conditionType)
	if existing != nil &&
		existing.Status == status &&
		existing.Reason == reason &&
		existing.Message == message &&
		existing.ObservedGeneration == generation {
		return false, nil
	}

	next := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	}
	if existing != nil &&
		existing.Status == status &&
		existing.Reason == reason &&
		existing.Message == message {
		next.LastTransitionTime = existing.LastTransitionTime
	}

	apimeta.SetStatusCondition(conditions, next)

	return true, c.Status().Update(ctx, target)
}
