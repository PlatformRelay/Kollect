// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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

	defaultSecretNamespace = "default"
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
	existing := apimeta.FindStatusCondition(*conditions, conditionType)
	if existing != nil &&
		existing.Status == status &&
		existing.Reason == reason &&
		existing.Message == message &&
		existing.ObservedGeneration == generation {
		return nil
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

	return c.Status().Update(ctx, target)
}
