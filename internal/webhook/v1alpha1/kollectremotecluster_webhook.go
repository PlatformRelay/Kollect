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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectremotecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectremoteclusters,verbs=create;update,versions=v1alpha1,name=vkollectremotecluster.kb.io,admissionReviewVersions=v1

type kollectRemoteClusterValidator struct{}

var _ admission.Validator[*kollectdevv1alpha1.KollectRemoteCluster] = &kollectRemoteClusterValidator{}

func setupKollectRemoteClusterWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectRemoteCluster{}).
		WithValidator(&kollectRemoteClusterValidator{}).
		Complete()
}

func (v *kollectRemoteClusterValidator) ValidateCreate(
	_ context.Context,
	rc *kollectdevv1alpha1.KollectRemoteCluster,
) (admission.Warnings, error) {
	return nil, v.validate(rc)
}

func (v *kollectRemoteClusterValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectRemoteCluster,
	newRC *kollectdevv1alpha1.KollectRemoteCluster,
) (admission.Warnings, error) {
	return nil, v.validate(newRC)
}

func (v *kollectRemoteClusterValidator) ValidateDelete(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectRemoteCluster,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *kollectRemoteClusterValidator) validate(rc *kollectdevv1alpha1.KollectRemoteCluster) error {
	if strings.TrimSpace(rc.Spec.ClusterName) == "" {
		return fmt.Errorf("spec.clusterName is required")
	}

	return nil
}
