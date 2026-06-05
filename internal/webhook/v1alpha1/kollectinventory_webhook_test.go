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

func testInventoryValidator(t *testing.T) *kollectInventoryValidator {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	return &kollectInventoryValidator{client: fake.NewClientBuilder().WithScheme(scheme).Build()}
}

func TestKollectInventoryValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	v := testInventoryValidator(t)

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SinkRefs: kollectdevv1alpha1.NewSinkRefList("other-ns/sink"),
		},
	})
	if err == nil {
		t.Fatal("expected validation error for cross-namespace sinkRef")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{},
	})
	if err != nil {
		t.Fatalf("expected valid inventory: %v", err)
	}
}

func TestKollectInventoryValidator_ValidateUpdateDeletion(t *testing.T) {
	t.Parallel()

	v := testInventoryValidator(t)
	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{},
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
