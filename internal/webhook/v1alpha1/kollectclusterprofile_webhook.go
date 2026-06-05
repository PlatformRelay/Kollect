// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

//nolint:dupl // cluster profile validators share boilerplate structure
package webhookv1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclusterprofile,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclusterprofiles,verbs=create;update,versions=v1alpha1,name=vkollectclusterprofile.kb.io,admissionReviewVersions=v1

type kollectClusterProfileValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterProfile] = &kollectClusterProfileValidator{}

func setupKollectClusterProfileWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterProfile{}).
		WithValidator(&kollectClusterProfileValidator{}).
		Complete()
}

func (v *kollectClusterProfileValidator) ValidateCreate(
	_ context.Context,
	profile *kollectdevv1alpha1.KollectClusterProfile,
) (admission.Warnings, error) {
	return v.validate(profile)
}

func (v *kollectClusterProfileValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterProfile,
	newProfile *kollectdevv1alpha1.KollectClusterProfile,
) (admission.Warnings, error) {
	if newProfile.DeletionTimestamp != nil {
		return nil, nil
	}

	return v.validate(newProfile)
}

func (v *kollectClusterProfileValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterProfile,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectClusterProfileValidator) validate(
	profile *kollectdevv1alpha1.KollectClusterProfile,
) (admission.Warnings, error) {
	errs := validation.ValidateClusterProfile(profile)
	if len(errs) > 0 {
		return nil, validation.ClusterProfileInvalid(profile.Name, errs)
	}

	return validation.ClusterProfileWarnings(profile), nil
}
