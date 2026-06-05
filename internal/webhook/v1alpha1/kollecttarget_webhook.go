// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollecttarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollecttargets,verbs=create;update,versions=v1alpha1,name=vkollecttarget.kb.io,admissionReviewVersions=v1

type kollectTargetValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectTarget] = &kollectTargetValidator{}

func setupKollectTargetWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectTarget{}).
		WithValidator(&kollectTargetValidator{}).
		Complete()
}

func (v *kollectTargetValidator) ValidateCreate(
	_ context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) (admission.Warnings, error) {
	return nil, v.validate(target)
}

func (v *kollectTargetValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectTarget,
	newTarget *kollectdevv1alpha1.KollectTarget,
) (admission.Warnings, error) {
	return nil, v.validate(newTarget)
}

func (v *kollectTargetValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectTarget,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectTargetValidator) validate(target *kollectdevv1alpha1.KollectTarget) error {
	if errs := validation.ValidateTargetSpec(&target.Spec); len(errs) > 0 {
		return validation.TargetInvalid(target.Name, errs)
	}

	mode := target.Spec.WatchMode
	if mode == "" {
		return nil
	}

	switch mode {
	case kollectdevv1alpha1.WatchModeAll, kollectdevv1alpha1.WatchModeOptIn:
		return nil
	default:
		return fmt.Errorf("spec.watchMode %q is invalid; allowed values: %q, %q",
			mode, kollectdevv1alpha1.WatchModeAll, kollectdevv1alpha1.WatchModeOptIn)
	}
}
