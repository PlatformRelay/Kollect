// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
)

func checkInventorySinksReachable(
	ctx context.Context,
	c client.Client,
	namespace string,
	sinkRefs []string,
) (bool, string, string) {
	if len(sinkRefs) == 0 {
		return true, "NoSinksConfigured", "no sinkRefs configured"
	}

	for _, name := range sinkRefs {
		check := scopeCheck{client: c}
		ok, reason, msg := check.sinkReachable(ctx, namespace, name)
		if !ok {
			return false, reason, msg
		}
	}

	return true, reasonSinksReachable, fmt.Sprintf("%d sink(s) resolved", len(sinkRefs))
}

func checkTargetNamespaceSinksReachable(ctx context.Context, c client.Client, namespace string) (bool, string, string) {
	var list kollectdevv1alpha1.KollectInventoryList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return false, "InventoryListFailed", fmt.Sprintf("list KollectInventory in %q: %v", namespace, err)
	}

	refs := uniqueSinkRefs(list.Items)
	if len(refs) == 0 {
		return true, "NoSinksInNamespace", "no inventory sinkRefs in namespace"
	}

	return checkInventorySinksReachable(ctx, c, namespace, refs)
}

func uniqueSinkRefs(inventories []kollectdevv1alpha1.KollectInventory) []string {
	seen := make(map[string]struct{})
	var refs []string

	for i := range inventories {
		for _, ref := range inventories[i].Spec.SinkRefs {
			if ref.Name == "" {
				continue
			}
			if _, ok := seen[ref.Name]; ok {
				continue
			}
			seen[ref.Name] = struct{}{}
			refs = append(refs, ref.Name)
		}
	}

	return refs
}

func setSinkReachableFromExport(conditions *[]metav1.Condition, generation int64, exportErr error) {
	if exportErr == nil {
		setSinkReachableCondition(conditions, generation, true, "ExportSucceeded", "last export completed successfully")

		return
	}

	reason := "ExportFailed"
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
