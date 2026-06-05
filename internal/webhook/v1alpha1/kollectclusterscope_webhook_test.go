// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectClusterScopeValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	v := &kollectClusterScopeValidator{}

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Version: "v1", Kind: ""},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for missing kind")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "dup-sinks"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			SinkRefs: []string{"git-a", "git-a"},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for duplicate sinkRefs")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
				AllowedNamespaces: []string{"team-a"},
			},
			SinkRefs: []string{"git-inventory"},
		},
	})
	if err != nil {
		t.Fatalf("expected valid cluster scope: %v", err)
	}
}

func TestKollectClusterScopeValidator_ValidateUpdateDelete(t *testing.T) {
	t.Parallel()

	v := &kollectClusterScopeValidator{}
	scope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}

	if _, err := v.ValidateUpdate(context.Background(), scope, scope); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), scope); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
