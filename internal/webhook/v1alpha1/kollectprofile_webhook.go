// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectprofile,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectprofiles,verbs=create;update,versions=v1alpha1,name=vkollectprofile.kb.io,admissionReviewVersions=v1

type kollectProfileValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectProfile] = &kollectProfileValidator{}

func setupKollectProfileWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectProfile{}).
		WithValidator(&kollectProfileValidator{}).
		Complete()
}

func (v *kollectProfileValidator) ValidateCreate(
	_ context.Context,
	profile *kollectdevv1alpha1.KollectProfile,
) (admission.Warnings, error) {
	return nil, v.validate(profile)
}

func (v *kollectProfileValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectProfile,
	newProfile *kollectdevv1alpha1.KollectProfile,
) (admission.Warnings, error) {
	if newProfile.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(newProfile)
}

func (v *kollectProfileValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectProfile,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectProfileValidator) validate(profile *kollectdevv1alpha1.KollectProfile) error {
	errs := validation.ValidateProfile(profile)
	if len(errs) > 0 {
		return validation.ProfileInvalid(profile.Name, errs)
	}

	return nil
}
