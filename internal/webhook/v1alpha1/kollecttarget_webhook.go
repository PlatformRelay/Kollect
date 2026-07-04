// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollecttarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollecttargets,verbs=create;update,versions=v1alpha1,name=vkollecttarget.kb.io,admissionReviewVersions=v1

type kollectTargetValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectTarget]
	client client.Client
}

var _ admission.Validator[*kollectdevv1alpha1.KollectTarget] = &kollectTargetValidator{}

func setupKollectTargetWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectTarget{}).
		WithValidator(&kollectTargetValidator{client: mgr.GetClient()}).
		Complete()
}

func (v *kollectTargetValidator) ValidateCreate(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
) (admission.Warnings, error) {
	return nil, v.validate(ctx, target)
}

func (v *kollectTargetValidator) ValidateUpdate(
	ctx context.Context,
	_ *kollectdevv1alpha1.KollectTarget,
	newTarget *kollectdevv1alpha1.KollectTarget,
) (admission.Warnings, error) {
	return nil, v.validate(ctx, newTarget)
}

func (v *kollectTargetValidator) validate(ctx context.Context, target *kollectdevv1alpha1.KollectTarget) error {
	if errs := validation.ValidateTargetSpec(&target.Spec); len(errs) > 0 {
		return validation.TargetInvalid(target.Name, errs)
	}

	mode := target.Spec.WatchMode
	if mode != "" {
		switch mode {
		case kollectdevv1alpha1.WatchModeAll, kollectdevv1alpha1.WatchModeOptIn:
		default:
			return fmt.Errorf("spec.watchMode %q is invalid; allowed values: %q, %q",
				mode, kollectdevv1alpha1.WatchModeAll, kollectdevv1alpha1.WatchModeOptIn)
		}
	}

	return v.validateScope(ctx, target)
}

func (v *kollectTargetValidator) validateScope(ctx context.Context, target *kollectdevv1alpha1.KollectTarget) error {
	binding, err := scope.Load(ctx, v.client, target.Namespace)
	if err != nil {
		return fmt.Errorf("load KollectScope: %w", err)
	}

	if !binding.Enforced {
		return nil
	}

	var profile kollectdevv1alpha1.KollectProfile
	profileKey := client.ObjectKey{Namespace: target.Namespace, Name: target.Spec.ProfileRef}
	if err := v.client.Get(ctx, profileKey, &profile); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("load KollectProfile: %w", err)
	}

	gvks := scope.CollectRuleGVKs(target.Spec.CollectionFilterSpec, profile.Spec.TargetGVK)
	if err := scope.ValidateResourceRuleGVKs(binding.Scope, gvks); err != nil {
		return err
	}

	intentNS := scope.NormalizeNamespaceList(target.Spec.IncludedNamespaces)
	if len(intentNS) == 0 && target.Spec.NamespaceSelector != nil {
		// Static allowlist not set; reconcile validates runtime namespaceSelector matches.
		return scope.ValidateDeniedNamespaces(binding.Scope, intentNS)
	}

	if err := scope.ValidateTargetIncludedNamespaces(binding.Scope, intentNS); err != nil {
		return err
	}

	return scope.ValidateDeniedNamespaces(binding.Scope, intentNS)
}
