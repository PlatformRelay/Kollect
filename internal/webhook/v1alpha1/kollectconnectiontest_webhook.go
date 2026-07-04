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
// +kubebuilder:webhook:path=/validate-kollect-dev-v1alpha1-kollectconnectiontest,mutating=false,failurePolicy=fail,sideEffects=None,groups=kollect.dev,resources=kollectconnectiontests,verbs=create;update,versions=v1alpha1,name=vkollectconnectiontest.kb.io,admissionReviewVersions=v1

type kollectConnectionTestValidator struct {
	noopDelete[*kollectdevv1alpha1.KollectConnectionTest]
}

var _ admission.Validator[*kollectdevv1alpha1.KollectConnectionTest] = &kollectConnectionTestValidator{}

func setupKollectConnectionTestWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &kollectdevv1alpha1.KollectConnectionTest{}).
		WithValidator(&kollectConnectionTestValidator{}).
		Complete()
}

func (v *kollectConnectionTestValidator) ValidateCreate(
	_ context.Context,
	test *kollectdevv1alpha1.KollectConnectionTest,
) (admission.Warnings, error) {
	return nil, v.validate(test)
}

func (v *kollectConnectionTestValidator) ValidateUpdate(
	_ context.Context,
	_ *kollectdevv1alpha1.KollectConnectionTest,
	newTest *kollectdevv1alpha1.KollectConnectionTest,
) (admission.Warnings, error) {
	if newTest.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, v.validate(newTest)
}

func (v *kollectConnectionTestValidator) validate(test *kollectdevv1alpha1.KollectConnectionTest) error {
	errs := validation.ValidateConnectionTestSpec(&test.Spec)
	if len(errs) > 0 {
		return validation.ConnectionTestInvalid(test.Name, errs)
	}

	return nil
}
