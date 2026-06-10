// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

func checkInventorySinksReachable(
	ctx context.Context,
	c client.Client,
	namespace string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
) (bool, string, string) {
	if len(bindings) == 0 {
		return true, "NoSinksConfigured", "no family sink refs configured"
	}

	for _, binding := range bindings {
		check := scopeCheck{client: c}
		ok, reason, msg := check.familySinkReachable(ctx, namespace, binding)
		if !ok {
			return false, reason, msg
		}
	}

	return true, reasonSinksReachable, fmt.Sprintf("%d sink(s) resolved", len(bindings))
}

func checkTargetNamespaceSinksReachable(ctx context.Context, c client.Client, namespace string) (bool, string, string) {
	var list kollectdevv1alpha1.KollectInventoryList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return false, "InventoryListFailed", fmt.Sprintf("list KollectInventory in %q: %v", namespace, err)
	}

	bindings := uniqueInventorySinkBindings(list.Items)
	if len(bindings) == 0 {
		return true, "NoSinksInNamespace", "no inventory family sink refs in namespace"
	}

	return checkInventorySinksReachable(ctx, c, namespace, bindings)
}

func uniqueInventorySinkBindings(inventories []kollectdevv1alpha1.KollectInventory) []kollectdevv1alpha1.InventorySinkBinding {
	seen := make(map[string]struct{})
	var bindings []kollectdevv1alpha1.InventorySinkBinding

	for i := range inventories {
		for _, binding := range inventorySinkBindings(&inventories[i]) {
			key := sinkExportKey(binding)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			bindings = append(bindings, binding)
		}
	}

	return bindings
}

func (s scopeCheck) familySinkReachable(
	ctx context.Context,
	namespace string,
	binding kollectdevv1alpha1.InventorySinkBinding,
) (bool, string, string) {
	resolved, err := loadClusterInventorySink(ctx, s.client, namespace, binding)
	recordStaticRefResolution("KollectClusterInventory", staticRefTypeForFamily(binding.Family), err)
	if err != nil {
		reason := reasonSinkNotFound
		if apierrors.IsForbidden(err) {
			reason = reasonSinkForbidden
		}

		return false, reason, err.Error()
	}

	conditions, err := familySinkConditions(ctx, s.client, resolved)
	if err != nil {
		return false, "SinkLookupFailed", err.Error()
	}

	verified := apimeta.FindStatusCondition(conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if verified != nil && verified.Status == metav1.ConditionFalse {
		msg := "sink connection not verified"
		if verified.Message != "" {
			msg = verified.Message
		}

		return false, reasonSinkUnreachable, msg
	}

	if verified != nil && verified.Status == metav1.ConditionTrue {
		msg := fmt.Sprintf("%s %q connection verified", familySinkKind(binding.Family), binding.Name)
		if verified.Message != "" {
			msg = verified.Message
		}

		return true, "ConnectionVerified", msg
	}

	return true, "SinkResolved", fmt.Sprintf("%s %q found", familySinkKind(binding.Family), binding.Name)
}

func familySinkConditions(ctx context.Context, c client.Client, resolved *sink.ResolvedSink) ([]metav1.Condition, error) {
	switch resolved.Family {
	case kollectdevv1alpha1.SinkFamilySnapshot:
		var obj kollectdevv1alpha1.KollectSnapshotSink
		if err := c.Get(ctx, client.ObjectKey{Namespace: resolved.Namespace, Name: resolved.Name}, &obj); err != nil {
			return nil, err
		}
		return obj.Status.Conditions, nil
	case kollectdevv1alpha1.SinkFamilyDatabase:
		var obj kollectdevv1alpha1.KollectDatabaseSink
		if err := c.Get(ctx, client.ObjectKey{Namespace: resolved.Namespace, Name: resolved.Name}, &obj); err != nil {
			return nil, err
		}
		return obj.Status.Conditions, nil
	case kollectdevv1alpha1.SinkFamilyEvent:
		var obj kollectdevv1alpha1.KollectEventSink
		if err := c.Get(ctx, client.ObjectKey{Namespace: resolved.Namespace, Name: resolved.Name}, &obj); err != nil {
			return nil, err
		}
		return obj.Status.Conditions, nil
	default:
		return nil, fmt.Errorf("unknown sink family %q", resolved.Family)
	}
}

func setSinkReachableFromExport(conditions *[]metav1.Condition, generation int64, exportErr error) {
	if exportErr == nil {
		setSinkReachableCondition(conditions, generation, true, "ExportSucceeded", "last export completed successfully")

		return
	}

	reason := reasonExportFailed
	if kollecterrors.IsTerminal(exportErr) {
		reason = kollectdevv1alpha1.ReasonExportTerminal
	}

	setSinkReachableCondition(conditions, generation, false, reason, exportErr.Error())
}

func setSinkReachableCondition(conditions *[]metav1.Condition, generation int64, ok bool, reason, message string) {
	status := metav1.ConditionTrue
	if !ok {
		status = metav1.ConditionFalse
	}

	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionSinkReachable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}

func setSyncedCondition(conditions *[]metav1.Condition, generation int64, ok bool, reason, message string) {
	status := metav1.ConditionTrue
	if !ok {
		status = metav1.ConditionFalse
	}

	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionSynced,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}
