// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/redact"
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

	// reasonCleanupSinkGone marks a deletion-time cleanup where a bound sink CR was
	// already gone (namespace cascade ordering): cleanup completes, but objects the
	// sink had exported may be retained in the backend because its spec/credentials
	// no longer exist to address them (K-29).
	reasonCleanupSinkGone = "CleanupSinkGone"

	// reasonCleanupRetained marks a deletion-time cleanup where the backend cannot
	// retract previously exported data (event streams; layout trees whose files can
	// interleave with other inventories'). Deletion proceeds, retention is announced (K-28).
	reasonCleanupRetained = "CleanupRetained"

	// reasonCleanupRetainedByPolicy marks a deletion-time cleanup that left a
	// snapshot sink's exported objects in place because its deletionPolicy is
	// Retain, the default (ADR-0421). Expected behaviour: a Normal event.
	reasonCleanupRetainedByPolicy = "CleanupRetainedByPolicy"

	// reasonCleanupSharedIdentity marks a Delete-policy cleanup that skipped the
	// retraction because another live inventory renders the same export identity
	// (KollectClusterInventory X and KollectInventory X in namespace "cluster").
	reasonCleanupSharedIdentity = "CleanupSharedIdentity"

	// reasonCleanupForced marks deletion where the operator set the
	// kollect.dev/force-cleanup annotation: the finalizer is dropped without
	// backend cleanup (K-30 escape hatch).
	reasonCleanupForced = "CleanupForced"

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
	reason, message string,
) (written bool, err error) {
	// Redact before the skip check so it compares like with like: the
	// persisted message is always the redacted one (K-23).
	message = redact.Text(message)

	const status = metav1.ConditionTrue

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
