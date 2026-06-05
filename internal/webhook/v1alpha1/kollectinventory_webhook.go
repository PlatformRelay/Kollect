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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectinventory,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectinventories,verbs=create;update,versions=v1alpha1,name=vkollectinventory.kb.io,admissionReviewVersions=v1

type kollectInventoryValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectInventory] = &kollectInventoryValidator{}

func setupKollectInventoryWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectInventory{}).
		WithValidator(&kollectInventoryValidator{}).
		Complete()
}

func (v *kollectInventoryValidator) ValidateCreate(
	_ context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (admission.Warnings, error) {
	return nil, v.validate(inv)
}

func (v *kollectInventoryValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectInventory,
	newInv *kollectdevv1alpha1.KollectInventory,
) (admission.Warnings, error) {
	if newInv.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(newInv)
}

func (v *kollectInventoryValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectInventory,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectInventoryValidator) validate(inv *kollectdevv1alpha1.KollectInventory) error {
	errs := validation.ValidateInventorySpec(&inv.Spec)
	if len(errs) > 0 {
		return validation.InventoryInvalid(inv.Name, errs)
	}

	return nil
}
