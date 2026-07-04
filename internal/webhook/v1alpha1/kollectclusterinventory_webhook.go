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
	"github.com/konih/kollect/internal/operator"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclusterinventory,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclusterinventories,verbs=create;update,versions=v1alpha1,name=vkollectclusterinventory.kb.io,admissionReviewVersions=v1

type kollectClusterInventoryValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectClusterInventory]
	client     client.Client
	tenantMode bool
}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterInventory] = &kollectClusterInventoryValidator{}

func setupKollectClusterInventoryWebhook(mgr ctrl.Manager, tenantMode bool) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterInventory{}).
		WithValidator(&kollectClusterInventoryValidator{client: mgr.GetClient(), tenantMode: tenantMode}).
		Complete()
}

func (v *kollectClusterInventoryValidator) ValidateCreate(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) (admission.Warnings, error) {
	return nil, v.validate(ctx, inv)
}

func (v *kollectClusterInventoryValidator) ValidateUpdate(
	ctx context.Context,
	_ *kollectdevv1alpha1.KollectClusterInventory,
	newInv *kollectdevv1alpha1.KollectClusterInventory,
) (admission.Warnings, error) {
	if newInv.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(ctx, newInv)
}

func (v *kollectClusterInventoryValidator) validate(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) error {
	if v.tenantMode {
		return clusterKindRejectedInTenantMode("KollectClusterInventory", inv.Name)
	}

	errs := validation.ValidateClusterInventorySpec(&inv.Spec)
	if len(errs) > 0 {
		return validation.ClusterInventoryInvalid(inv.Name, errs)
	}

	sinkNS := inv.Spec.SinkNamespace
	if sinkNS == "" {
		sinkNS = operator.DefaultSecretNamespace
	}

	binding, err := scope.Load(ctx, v.client, sinkNS)
	if err != nil {
		return validation.ClusterInventoryInvalid(inv.Name, validation.ScopeLoadErrors(err))
	}
	if binding.Enforced && binding.Scope != nil {
		floor := validation.ScopeMinExportInterval(binding.Scope)
		errs = validation.ValidateIntervalsAgainstScopeFloor(
			inv.Spec.ExportMinInterval, kollectdevv1alpha1.AllClusterInventorySinkRefLists(&inv.Spec), floor)
		if len(errs) > 0 {
			return validation.ClusterInventoryInvalid(inv.Name, errs)
		}
	}

	return v.validateClusterScope(ctx, inv)
}

func (v *kollectClusterInventoryValidator) validateClusterScope(
	ctx context.Context,
	inv *kollectdevv1alpha1.KollectClusterInventory,
) error {
	binding, err := scope.LoadCluster(ctx, v.client)
	if err != nil {
		return validation.ClusterInventoryInvalid(inv.Name, validation.ScopeLoadErrors(err))
	}

	if !binding.Enforced {
		return nil
	}

	sinkNS := inv.Spec.SinkNamespace
	if sinkNS == "" {
		sinkNS = operator.DefaultSecretNamespace
	}

	for _, ns := range scope.ClusterInventoryStaticRefNamespaces(&inv.Spec, sinkNS) {
		if err := scope.ValidateClusterScopeStaticRefNamespace(binding.Scope, ns); err != nil {
			return validation.ClusterInventoryInvalid(inv.Name, validation.ScopeViolationErrors(err))
		}
	}

	bindings := kollectdevv1alpha1.CollectClusterInventorySinkBindings(&inv.Spec)
	if err := scope.ValidateClusterInventoryClusterScopeSinkRefs(binding.Scope, bindings); err != nil {
		return validation.ClusterInventoryInvalid(inv.Name, validation.ScopeViolationErrors(err))
	}

	return nil
}
