// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkInventorySinksReachable(ctx context.Context, c client.Client, sinkRefs []string) (bool, string, string) {
	if len(sinkRefs) == 0 {
		return true, "NoSinksConfigured", "no sinkRefs configured"
	}

	for _, name := range sinkRefs {
		check := scopeCheck{client: c}
		ok, reason, msg := check.sinkReachable(ctx, name)
		if !ok {
			return false, reason, msg
		}
	}

	return true, "SinksReachable", fmt.Sprintf("%d sink(s) resolved", len(sinkRefs))
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
