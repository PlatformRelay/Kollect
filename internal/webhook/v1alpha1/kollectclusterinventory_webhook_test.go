// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func testClusterInventoryValidator(t *testing.T) *kollectClusterInventoryValidator {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	return &kollectClusterInventoryValidator{client: fake.NewClientBuilder().WithScheme(scheme).Build()}
}

func TestKollectClusterInventoryValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	v := testClusterInventoryValidator(t)

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec:       kollectdevv1alpha1.KollectClusterInventorySpec{},
	})
	if err == nil {
		t.Fatal("expected validation error for missing namespaceSelector")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"kollect.dev/tenant": "platform"},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected valid cluster inventory: %v", err)
	}
}

func TestKollectClusterInventoryValidator_ValidateUpdateDeletion(t *testing.T) {
	t.Parallel()

	v := testClusterInventoryValidator(t)
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "a"},
			},
		},
	}
	deleting := inv.DeepCopy()
	deleting.DeletionTimestamp = &now

	if _, err := v.ValidateUpdate(context.Background(), inv, deleting); err != nil {
		t.Fatalf("deletion update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), inv); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
