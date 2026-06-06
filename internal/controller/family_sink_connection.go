// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/sink"
)

type familySinkConnection struct {
	client client.Client
}

func (f familySinkConnection) reconcile(
	ctx context.Context,
	obj client.Object,
	spec kollectdevv1alpha1.KollectSinkSpec,
	common *kollectdevv1alpha1.SinkCommonFields,
	conditions *[]metav1.Condition,
) error {
	if !shouldTestFamilyConnection(common, obj) {
		return nil
	}

	namespace := obj.GetNamespace()
	buildCtx, err := sink.BuildContextFromSpec(ctx, f.client, spec, namespace)
	if err != nil {
		return f.setConnectionFailed(ctx, obj, conditions, "SecretResolveFailed", err.Error())
	}

	okMessage, testErr := sink.RunConnectionTest(ctx, spec, buildCtx)
	if testErr != nil {
		metrics.SinkConnectionTestTotal.WithLabelValues(spec.Type, metrics.ResultFailure).Inc()

		return f.setConnectionFailed(ctx, obj, conditions, "ConnectionTestFailed", testErr.Error())
	}

	metrics.SinkConnectionTestTotal.WithLabelValues(spec.Type, metrics.ResultSuccess).Inc()

	return f.setConnectionVerified(ctx, obj, spec, common, conditions, okMessage)
}

func shouldTestFamilyConnection(common *kollectdevv1alpha1.SinkCommonFields, obj client.Object) bool {
	if kollectdevv1alpha1.ConnectionTestEnabledCommon(common) {
		return true
	}

	v, ok := obj.GetAnnotations()[kollectdevv1alpha1.AnnotationTestConnection]
	return ok && strings.EqualFold(v, "true")
}

func (f familySinkConnection) setConnectionVerified(
	ctx context.Context,
	obj client.Object,
	spec kollectdevv1alpha1.KollectSinkSpec,
	common *kollectdevv1alpha1.SinkCommonFields,
	conditions *[]metav1.Condition,
	message string,
) error {
	apimeta.RemoveStatusCondition(conditions, conditionDegraded)
	setFamilyTLSInsecureCondition(conditions, spec, obj.GetGeneration())
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionTrue,
		Reason:             "ConnectionOK",
		Message:            message,
		ObservedGeneration: obj.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})

	if err := f.client.Status().Update(ctx, obj); err != nil {
		return err
	}

	if shouldClearFamilyTestConnectionAnnotation(common, obj) {
		base := obj.DeepCopyObject().(client.Object)
		ann := obj.GetAnnotations()
		delete(ann, kollectdevv1alpha1.AnnotationTestConnection)
		obj.SetAnnotations(ann)
		if err := f.client.Patch(ctx, obj, client.MergeFrom(base)); err != nil {
			return err
		}
	}

	return nil
}

func (f familySinkConnection) setConnectionFailed(
	ctx context.Context,
	obj client.Object,
	conditions *[]metav1.Condition,
	reason, message string,
) error {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionConnectionVerified,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: obj.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: obj.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})

	if err := f.client.Status().Update(ctx, obj); err != nil {
		if apierrors.IsConflict(err) {
			return nil
		}

		return err
	}

	return nil
}

func shouldClearFamilyTestConnectionAnnotation(common *kollectdevv1alpha1.SinkCommonFields, obj client.Object) bool {
	if kollectdevv1alpha1.ConnectionTestEnabledCommon(common) {
		return false
	}

	_, ok := obj.GetAnnotations()[kollectdevv1alpha1.AnnotationTestConnection]

	return ok
}

func setFamilyTLSInsecureCondition(conditions *[]metav1.Condition, spec kollectdevv1alpha1.KollectSinkSpec, generation int64) {
	insecure := spec.TLS != nil && spec.TLS.InsecureSkipVerify
	if !insecure {
		apimeta.RemoveStatusCondition(conditions, kollectdevv1alpha1.ConditionTLSInsecure)

		return
	}

	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               kollectdevv1alpha1.ConditionTLSInsecure,
		Status:             metav1.ConditionTrue,
		Reason:             "InsecureSkipVerify",
		Message:            "TLS certificate verification is disabled; use only for development",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
	})
}
