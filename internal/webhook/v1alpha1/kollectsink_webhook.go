// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

//nolint:dupl // webhook validators share boilerplate structure
package webhookv1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectsink,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectsinks,verbs=create;update,versions=v1alpha1,name=vkollectsink.kb.io,admissionReviewVersions=v1

type kollectSinkValidator struct {
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectSink] = &kollectSinkValidator{}

func setupKollectSinkWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectSink{}).
		WithValidator(&kollectSinkValidator{client: mgr.GetClient()}).
		Complete()
}

func (v *kollectSinkValidator) ValidateCreate(
	ctx context.Context,
	ks *kollectdevv1alpha1.KollectSink,
) (admission.Warnings, error) {
	return v.validate(ctx, ks)
}

func (v *kollectSinkValidator) ValidateUpdate(
	ctx context.Context,
	_ *kollectdevv1alpha1.KollectSink,
	newKS *kollectdevv1alpha1.KollectSink,
) (admission.Warnings, error) {
	if newKS.DeletionTimestamp != nil {
		return nil, nil
	}

	return v.validate(ctx, newKS)
}

func (v *kollectSinkValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectSink,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectSinkValidator) validate(
	ctx context.Context,
	ks *kollectdevv1alpha1.KollectSink,
) (admission.Warnings, error) {
	errs := validation.ValidateSinkSpec(&ks.Spec)
	if len(errs) > 0 {
		return nil, validation.SinkInvalid(ks.Name, errs)
	}

	binding, err := scope.Load(ctx, v.client, ks.Namespace)
	if err != nil {
		return nil, validation.SinkInvalid(ks.Name, validation.ScopeLoadErrors(err))
	}
	if binding.Enforced && binding.Scope != nil {
		floor := validation.ScopeMinExportInterval(binding.Scope)
		errs = validation.ValidateSinkIntervalAgainstScopeFloor(ks.Spec.ExportMinInterval, floor)
		if len(errs) > 0 {
			return nil, validation.SinkInvalid(ks.Name, errs)
		}
	}

	return validation.ValidateGitSinkWarnings(&ks.Spec), nil
}
