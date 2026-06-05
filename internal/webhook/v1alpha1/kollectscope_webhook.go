// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectscope,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectscopes,verbs=create;update,versions=v1alpha1,name=vkollectscope.kb.io,admissionReviewVersions=v1

type kollectScopeValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectScope] = &kollectScopeValidator{}

func setupKollectScopeWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectScope{}).
		WithValidator(&kollectScopeValidator{}).
		Complete()
}

func (v *kollectScopeValidator) ValidateCreate(
	_ context.Context,
	scope *kollectdevv1alpha1.KollectScope,
) (admission.Warnings, error) {
	return nil, v.validate(scope)
}

func (v *kollectScopeValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectScope,
	newScope *kollectdevv1alpha1.KollectScope,
) (admission.Warnings, error) {
	return nil, v.validate(newScope)
}

func (v *kollectScopeValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectScope,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectScopeValidator) validate(scope *kollectdevv1alpha1.KollectScope) error {
	for i, gvk := range scope.Spec.AllowedGVKs {
		if gvk.Version == "" || gvk.Kind == "" {
			return fmt.Errorf("spec.allowedGVKs[%d]: version and kind are required", i)
		}
	}

	if err := validateUniqueNonEmptyStrings(scope.Spec.AllowedNamespaces, "spec.allowedNamespaces"); err != nil {
		return err
	}

	return validateUniqueNonEmptyStrings(scope.Spec.SinkRefs, "spec.sinkRefs")
}

func validateUniqueNonEmptyStrings(values []string, field string) error {
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		if value == "" {
			return fmt.Errorf("%s[%d]: must not be empty", field, i)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%s[%d]: duplicate entry %q", field, i, value)
		}
		seen[value] = struct{}{}
	}

	return nil
}
