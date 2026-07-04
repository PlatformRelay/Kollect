// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectClusterScopeValidateDelete_isNoop(t *testing.T) {
	t.Parallel()

	v := &kollectClusterScopeValidator{}
	warns, err := v.ValidateDelete(context.Background(), &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "cscope-a"},
	})
	if err != nil || warns != nil {
		t.Fatalf("ValidateDelete: warns=%v err=%v", warns, err)
	}
}

func TestKollectClusterScopeValidateUpdate_delegatesToValidate(t *testing.T) {
	t.Parallel()

	v := &kollectClusterScopeValidator{}
	scope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "cscope-a"},
	}
	warns, err := v.ValidateUpdate(context.Background(), scope, scope)
	if err != nil || warns != nil {
		t.Fatalf("ValidateUpdate(valid scope): warns=%v err=%v", warns, err)
	}
}
