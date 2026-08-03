// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("other-ns/sink"),
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

// TestKollectInventoryValidator_ValidateUpdate_nonDeletionRevalidates asserts a live
// (non-deletion) UPDATE re-runs full spec validation: an update that introduces a
// cross-namespace sinkRef is rejected, while an update to a valid spec is admitted
// (COV-90-S05, drives the non-deletion branch of ValidateUpdate).
func TestKollectInventoryValidator_ValidateUpdate_nonDeletionRevalidates(t *testing.T) {
	t.Parallel()

	v := testInventoryValidator(t)
	old := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{},
	}

	bad := old.DeepCopy()
	bad.Spec.DatabaseSinkRefs = kollectdevv1alpha1.NewSinkRefList("other-ns/sink")
	if _, err := v.ValidateUpdate(context.Background(), old, bad); err == nil {
		t.Fatal("expected non-deletion update to reject cross-namespace sinkRef")
	}

	good := old.DeepCopy()
	good.Spec.DatabaseSinkRefs = kollectdevv1alpha1.NewSinkRefList("sink")
	if _, err := v.ValidateUpdate(context.Background(), old, good); err != nil {
		t.Fatalf("valid non-deletion update: %v", err)
	}
}

// TestKollectInventoryValidator_scopeFloorEnforced drives the enforced-scope branch of
// the inventory validator's validate: with an enforced KollectScope floor in the
// namespace, an inventory whose exportMinInterval is below the floor is rejected
// (COV-90-S05).
func TestKollectInventoryValidator_scopeFloorEnforced(t *testing.T) {
	t.Parallel()

	teamScope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				MinExportInterval: &metav1.Duration{Duration: time.Hour},
			},
		},
	}
	v := &kollectInventoryValidator{client: newScopedFakeClient(t, teamScope)}

	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "too-fast", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &metav1.Duration{Duration: time.Second},
		},
	}

	_, err := v.ValidateCreate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected scope-floor violation for sub-floor exportMinInterval")
	}
	if !strings.Contains(err.Error(), "too-fast") {
		t.Fatalf("error should name the offending inventory, got: %v", err)
	}
}
