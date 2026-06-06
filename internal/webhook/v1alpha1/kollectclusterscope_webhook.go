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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclusterscope,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclusterscopes,verbs=create;update,versions=v1alpha1,name=vkollectclusterscope.kb.io,admissionReviewVersions=v1

type kollectClusterScopeValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterScope] = &kollectClusterScopeValidator{}

func setupKollectClusterScopeWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterScope{}).
		WithValidator(&kollectClusterScopeValidator{}).
		Complete()
}

func (v *kollectClusterScopeValidator) ValidateCreate(
	_ context.Context,
	scope *kollectdevv1alpha1.KollectClusterScope,
) (admission.Warnings, error) {
	return nil, v.validate(scope)
}

func (v *kollectClusterScopeValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterScope,
	newScope *kollectdevv1alpha1.KollectClusterScope,
) (admission.Warnings, error) {
	return nil, v.validate(newScope)
}

func (v *kollectClusterScopeValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterScope,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectClusterScopeValidator) validate(scope *kollectdevv1alpha1.KollectClusterScope) error {
	if errs := validation.ValidateScopeCeilingSpec(&scope.Spec.ScopeCeilingSpec, nil); len(errs) > 0 {
		return validation.ClusterScopeInvalid(scope.Name, errs)
	}

	var errs error
	for _, check := range []struct {
		field string
		vals  []string
	}{
		{"snapshotSinkRefs", scope.Spec.SnapshotSinkRefs},
		{"databaseSinkRefs", scope.Spec.DatabaseSinkRefs},
		{"eventSinkRefs", scope.Spec.EventSinkRefs},
	} {
		if err := validateUniqueNonEmptyStrings(check.vals, check.field); err != nil {
			if errs == nil {
				errs = err
			}
		}
	}
	return errs
}
