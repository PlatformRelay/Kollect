// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectTargetValidator_validateWatchMode(t *testing.T) {
	t.Parallel()

	v := &kollectTargetValidator{}

	if err := v.validate(&kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{WatchMode: ""},
	}); err != nil {
		t.Fatalf("empty watchMode: %v", err)
	}

	if err := v.validate(&kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{WatchMode: kollectdevv1alpha1.WatchModeAll},
	}); err != nil {
		t.Fatalf("All watchMode: %v", err)
	}

	if err := v.validate(&kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{WatchMode: kollectdevv1alpha1.WatchModeOptIn},
	}); err != nil {
		t.Fatalf("OptIn watchMode: %v", err)
	}

	if err := v.validate(&kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{WatchMode: "Maybe"},
	}); err == nil {
		t.Fatal("expected error for invalid watchMode")
	}
}
