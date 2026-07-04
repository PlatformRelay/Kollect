// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectScopeValidateDelete_isNoop(t *testing.T) {
	t.Parallel()

	v := &kollectScopeValidator{}
	warns, err := v.ValidateDelete(context.Background(), &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "scope-a"},
	})
	if err != nil || warns != nil {
		t.Fatalf("ValidateDelete: warns=%v err=%v", warns, err)
	}
}

func TestKollectScopeValidateUpdate_delegatesToValidate(t *testing.T) {
	t.Parallel()

	v := &kollectScopeValidator{}
	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "scope-a"},
	}
	warns, err := v.ValidateUpdate(context.Background(), scope, scope)
	if err != nil || warns != nil {
		t.Fatalf("ValidateUpdate(valid scope): warns=%v err=%v", warns, err)
	}
}

func TestValidateUniqueNonEmptyStrings(t *testing.T) {
	t.Parallel()

	if err := validateUniqueNonEmptyStrings([]string{"a", "b"}, "snapshotSinkRefs"); err != nil {
		t.Fatalf("unique values: %v", err)
	}

	if err := validateUniqueNonEmptyStrings([]string{""}, "snapshotSinkRefs"); err == nil {
		t.Fatal("expected empty string error")
	}

	if err := validateUniqueNonEmptyStrings([]string{"dup", "dup"}, "snapshotSinkRefs"); err == nil {
		t.Fatal("expected duplicate error")
	}
}
