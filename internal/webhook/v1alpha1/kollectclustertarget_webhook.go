// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

//nolint:dupl // webhook validators share boilerplate structure
package webhookv1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclustertarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclustertargets,verbs=create;update,versions=v1alpha1,name=vkollectclustertarget.kb.io,admissionReviewVersions=v1

type kollectClusterTargetValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterTarget] = &kollectClusterTargetValidator{}

func setupKollectClusterTargetWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterTarget{}).
		WithValidator(&kollectClusterTargetValidator{}).
		Complete()
}

func (v *kollectClusterTargetValidator) ValidateCreate(
	_ context.Context,
	target *kollectdevv1alpha1.KollectClusterTarget,
) (admission.Warnings, error) {
	return nil, v.validate(target)
}

func (v *kollectClusterTargetValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterTarget,
	newTarget *kollectdevv1alpha1.KollectClusterTarget,
) (admission.Warnings, error) {
	if newTarget.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(newTarget)
}

func (v *kollectClusterTargetValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterTarget,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectClusterTargetValidator) validate(target *kollectdevv1alpha1.KollectClusterTarget) error {
	errs := validation.ValidateClusterTargetSpec(&target.Spec)
	if len(errs) > 0 {
		return validation.ClusterTargetInvalid(target.Name, errs)
	}

	return nil
}
