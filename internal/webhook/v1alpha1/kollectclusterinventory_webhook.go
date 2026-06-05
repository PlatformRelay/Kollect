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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclusterinventory,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclusterinventories,verbs=create;update,versions=v1alpha1,name=vkollectclusterinventory.kb.io,admissionReviewVersions=v1

type kollectClusterInventoryValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterInventory] = &kollectClusterInventoryValidator{}

func setupKollectClusterInventoryWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterInventory{}).
		WithValidator(&kollectClusterInventoryValidator{}).
		Complete()
}

func (v *kollectClusterInventoryValidator) ValidateCreate(
	_ context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) (admission.Warnings, error) {
	return nil, v.validate(inv)
}

func (v *kollectClusterInventoryValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterInventory,
	newInv *kollectdevv1alpha1.KollectClusterInventory,
) (admission.Warnings, error) {
	if newInv.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(newInv)
}

func (v *kollectClusterInventoryValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterInventory,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectClusterInventoryValidator) validate(inv *kollectdevv1alpha1.KollectClusterInventory) error {
	errs := validation.ValidateClusterInventorySpec(&inv.Spec)
	if len(errs) > 0 {
		return validation.ClusterInventoryInvalid(inv.Name, errs)
	}

	return nil
}
