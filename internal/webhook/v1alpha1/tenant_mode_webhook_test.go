// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// In tenantMode the operator only holds namespaced RBAC, so cluster-scoped
// reconciled kinds must be rejected at admission (ADR-0208).
func TestKollectClusterTargetValidator_tenantModeRejectsCreateAndUpdate(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectClusterTargetValidator{
		client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		tenantMode: true,
	}

	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: platformProfileRef(),
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
	}

	_, err := v.ValidateCreate(context.Background(), target)
	if err == nil {
		t.Fatal("expected tenantMode create rejection for KollectClusterTarget")
	}
	if !strings.Contains(err.Error(), "tenantMode") {
		t.Fatalf("error should mention tenantMode: %v", err)
	}

	if _, err := v.ValidateUpdate(context.Background(), target, target.DeepCopy()); err == nil {
		t.Fatal("expected tenantMode update rejection for KollectClusterTarget")
	}
}

// tenantMode rejection must not block deletion of pre-existing cluster kinds.
func TestKollectClusterTargetValidator_tenantModeAllowsDeletion(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectClusterTargetValidator{
		client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		tenantMode: true,
	}

	now := metav1.Now()
	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: platformProfileRef(),
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
	}
	deleting := target.DeepCopy()
	deleting.DeletionTimestamp = &now

	if _, err := v.ValidateUpdate(context.Background(), target, deleting); err != nil {
		t.Fatalf("tenantMode deletion update should pass: %v", err)
	}
	if _, err := v.ValidateDelete(context.Background(), target); err != nil {
		t.Fatalf("tenantMode delete should pass: %v", err)
	}
}

func TestKollectClusterInventoryValidator_tenantModeRejectsCreateAndUpdate(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectClusterInventoryValidator{
		client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		tenantMode: true,
	}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "ci"},
	}

	_, err := v.ValidateCreate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected tenantMode create rejection for KollectClusterInventory")
	}
	if !strings.Contains(err.Error(), "tenantMode") {
		t.Fatalf("error should mention tenantMode: %v", err)
	}

	if _, err := v.ValidateUpdate(context.Background(), inv, inv.DeepCopy()); err == nil {
		t.Fatal("expected tenantMode update rejection for KollectClusterInventory")
	}
}

func TestKollectClusterInventoryValidator_tenantModeAllowsDeletion(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectClusterInventoryValidator{
		client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		tenantMode: true,
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "ci"},
	}
	deleting := inv.DeepCopy()
	deleting.DeletionTimestamp = &now

	if _, err := v.ValidateUpdate(context.Background(), inv, deleting); err != nil {
		t.Fatalf("tenantMode deletion update should pass: %v", err)
	}
	if _, err := v.ValidateDelete(context.Background(), inv); err != nil {
		t.Fatalf("tenantMode delete should pass: %v", err)
	}
}
