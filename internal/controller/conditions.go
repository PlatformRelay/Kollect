// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	conditionReady    = "Ready"
	conditionDegraded = "Degraded"
)

func setTargetCondition(
	ctx context.Context,
	c client.Client,
	target client.Object,
	generation int64,
	conditions *[]metav1.Condition,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})

	return c.Status().Update(ctx, target)
}
