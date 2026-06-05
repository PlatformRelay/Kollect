// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

//nolint:lll // kubebuilder webhook marker must stay on one line
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollecthub,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollecthubs,verbs=create;update,versions=v1alpha1,name=vkollecthub.kb.io,admissionReviewVersions=v1

type kollectHubValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectHub] = &kollectHubValidator{}

func setupKollectHubWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectHub{}).
		WithValidator(&kollectHubValidator{}).
		Complete()
}

func (v *kollectHubValidator) ValidateCreate(
	_ context.Context,
	hub *kollectdevv1alpha1.KollectHub,
) (admission.Warnings, error) {
	return nil, v.validate(hub)
}

func (v *kollectHubValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectHub,
	newHub *kollectdevv1alpha1.KollectHub,
) (admission.Warnings, error) {
	return nil, v.validate(newHub)
}

func (v *kollectHubValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectHub,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectHubValidator) validate(hub *kollectdevv1alpha1.KollectHub) error {
	transportType := strings.TrimSpace(hub.Spec.Transport.Type)
	if transportType == "" {
		return fmt.Errorf("spec.transport.type is required")
	}

	if transportType == "redis" {
		if hub.Spec.Transport.Redis == nil || strings.TrimSpace(hub.Spec.Transport.Redis.URL) == "" {
			return fmt.Errorf("spec.transport.redis.url is required when type is redis")
		}
	}

	return nil
}
