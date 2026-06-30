// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

//nolint:dupl // webhook validators share boilerplate structure
package webhookv1alpha1

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/scope"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclustertarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclustertargets,verbs=create;update,versions=v1alpha1,name=vkollectclustertarget.kb.io,admissionReviewVersions=v1

type kollectClusterTargetValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectClusterTarget]
	client     client.Client
	tenantMode bool
}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterTarget] = &kollectClusterTargetValidator{}

func setupKollectClusterTargetWebhook(mgr ctrl.Manager, tenantMode bool) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterTarget{}).
		WithValidator(&kollectClusterTargetValidator{client: mgr.GetClient(), tenantMode: tenantMode}).
		Complete()
}

func (v *kollectClusterTargetValidator) ValidateCreate(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectClusterTarget,
) (admission.Warnings, error) {
	return nil, v.validate(ctx, target)
}

func (v *kollectClusterTargetValidator) ValidateUpdate(
	ctx context.Context,
	_ *kollectdevv1alpha1.KollectClusterTarget,
	newTarget *kollectdevv1alpha1.KollectClusterTarget,
) (admission.Warnings, error) {
	if newTarget.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(ctx, newTarget)
}

func (v *kollectClusterTargetValidator) validate(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectClusterTarget,
) error {
	if v.tenantMode {
		return clusterKindRejectedInTenantMode("KollectClusterTarget", target.Name)
	}

	errs := validation.ValidateClusterTargetSpec(&target.Spec)
	if len(errs) > 0 {
		return validation.ClusterTargetInvalid(target.Name, errs)
	}

	return v.validateClusterScope(ctx, target)
}

func (v *kollectClusterTargetValidator) validateClusterScope(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectClusterTarget,
) error {
	binding, err := scope.LoadCluster(ctx, v.client)
	if err != nil {
		return fmt.Errorf("load KollectClusterScope: %w", err)
	}

	if !binding.Enforced {
		return nil
	}

	profile, err := resolveClusterTargetProfileForWebhook(ctx, v.client, target.Spec.ProfileRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	gvks := scope.CollectRuleGVKs(target.Spec.CollectionFilterSpec, profile.Spec.TargetGVK)
	for _, gvk := range gvks {
		if err := scope.ValidateClusterScopeGVKs(binding.Scope, gvk); err != nil {
			return err
		}
	}

	intentNS := scope.NormalizeNamespaceList(target.Spec.IncludedNamespaces)
	if err := scope.ValidateClusterScopeNamespaces(binding.Scope, intentNS); err != nil {
		return err
	}

	return scope.ValidateClusterScopeStaticRefNamespace(binding.Scope, target.Spec.ProfileRef.Namespace)
}

func resolveClusterTargetProfileForWebhook(
	ctx context.Context,
	c client.Client,
	profileRef kollectdevv1alpha1.NamespacedObjectReference,
) (*kollectdevv1alpha1.KollectProfile, error) {
	var profile kollectdevv1alpha1.KollectProfile
	key := client.ObjectKey{Name: profileRef.Name, Namespace: profileRef.Namespace}
	if err := c.Get(ctx, key, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			gr := kollectdevv1alpha1.GroupVersion.WithResource("kollectprofiles").GroupResource()
			return nil, apierrors.NewNotFound(gr, profileRef.Name)
		}

		return nil, err
	}

	return &profile, nil
}
