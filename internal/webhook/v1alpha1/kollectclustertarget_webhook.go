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
	"github.com/konih/kollect/internal/operator"
	"github.com/konih/kollect/internal/validation"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectclustertarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectclustertargets,verbs=create;update,versions=v1alpha1,name=vkollectclustertarget.kb.io,admissionReviewVersions=v1

type kollectClusterTargetValidator struct {
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectClusterTarget] = &kollectClusterTargetValidator{}

func setupKollectClusterTargetWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectClusterTarget{}).
		WithValidator(&kollectClusterTargetValidator{client: mgr.GetClient()}).
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

func (v *kollectClusterTargetValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectClusterTarget,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectClusterTargetValidator) validate(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectClusterTarget,
) error {
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
	return scope.ValidateClusterScopeNamespaces(binding.Scope, intentNS)
}

func resolveClusterTargetProfileForWebhook(
	ctx context.Context,
	c client.Client,
	profileRef string,
) (*kollectdevv1alpha1.KollectProfile, error) {
	var clusterProfile kollectdevv1alpha1.KollectClusterProfile
	if err := c.Get(ctx, client.ObjectKey{Name: profileRef}, &clusterProfile); err == nil {
		return &kollectdevv1alpha1.KollectProfile{
			Spec: kollectdevv1alpha1.KollectProfileSpec{TargetGVK: clusterProfile.Spec.TargetGVK},
		}, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("load KollectClusterProfile: %w", err)
	}

	var profile kollectdevv1alpha1.KollectProfile
	key := client.ObjectKey{Name: profileRef, Namespace: operator.DefaultSecretNamespace}
	if err := c.Get(ctx, key, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			gr := kollectdevv1alpha1.GroupVersion.WithResource("kollectprofiles").GroupResource()
			return nil, apierrors.NewNotFound(gr, profileRef)
		}

		return nil, err
	}

	return &profile, nil
}
