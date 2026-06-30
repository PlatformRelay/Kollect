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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectinventory,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectinventories,verbs=create;update,versions=v1alpha1,name=vkollectinventory.kb.io,admissionReviewVersions=v1

type kollectInventoryValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectInventory]
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectInventory] = &kollectInventoryValidator{}

func setupKollectInventoryWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectInventory{}).
		WithValidator(&kollectInventoryValidator{client: mgr.GetClient()}).
		Complete()
}

func (v *kollectInventoryValidator) ValidateCreate(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectInventory,
) (admission.Warnings, error) {
	return nil, v.validate(ctx, inv)
}

func (v *kollectInventoryValidator) ValidateUpdate(
	ctx context.Context,
	_ *kollectdevv1alpha1.KollectInventory,
	newInv *kollectdevv1alpha1.KollectInventory,
) (admission.Warnings, error) {
	if newInv.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(ctx, newInv)
}

func (v *kollectInventoryValidator) validate(ctx context.Context, inv *kollectdevv1alpha1.KollectInventory) error {
	errs := validation.ValidateInventorySpec(&inv.Spec)
	if len(errs) > 0 {
		return validation.InventoryInvalid(inv.Name, errs)
	}

	binding, err := scope.Load(ctx, v.client, inv.Namespace)
	if err != nil {
		return validation.InventoryInvalid(inv.Name, validation.ScopeLoadErrors(err))
	}
	if binding.Enforced && binding.Scope != nil {
		floor := validation.ScopeMinExportInterval(binding.Scope)
		errs = validation.ValidateIntervalsAgainstScopeFloor(
			inv.Spec.ExportMinInterval, kollectdevv1alpha1.AllInventorySinkRefLists(&inv.Spec), floor)
		if len(errs) > 0 {
			return validation.InventoryInvalid(inv.Name, errs)
		}
	}

	return nil
}
