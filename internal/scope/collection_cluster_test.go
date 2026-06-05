// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package scope

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateClusterScopeDeniedNamespaces(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				DeniedNamespaces: []string{"kube-system"},
			},
		},
	}

	if err := ValidateClusterScopeDeniedNamespaces(scope, []string{"team-a"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if err := ValidateClusterScopeDeniedNamespaces(scope, []string{"kube-system"}); err == nil {
		t.Fatal("expected denied namespace violation")
	}

	if err := ValidateClusterScopeDeniedNamespaces(nil, []string{"kube-system"}); err != nil {
		t.Fatalf("nil scope should allow: %v", err)
	}
}

func TestValidateClusterScopeNamespaces(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedNamespaces: []string{"team-a", "team-b"},
				DeniedNamespaces:  []string{"kube-system"},
			},
		},
	}

	if err := ValidateClusterScopeNamespaces(scope, []string{"team-a"}); err != nil {
		t.Fatalf("expected allow: %v", err)
	}

	if err := ValidateClusterScopeNamespaces(scope, []string{"team-c"}); err == nil {
		t.Fatal("expected allowlist violation")
	}

	if err := ValidateClusterScopeNamespaces(scope, []string{"kube-system"}); err == nil {
		t.Fatal("expected deny violation")
	}
}

func TestValidateClusterScopeGVKs(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}

	allowed := kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	if err := ValidateClusterScopeGVKs(scope, allowed); err != nil {
		t.Fatalf("expected allowed GVK: %v", err)
	}

	denied := allowed
	denied.Kind = "Pod"
	if err := ValidateClusterScopeGVKs(scope, denied); err == nil {
		t.Fatal("expected GVK violation")
	}
}

func TestValidateResourceRuleGVKs(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}

	gvks := []kollectdevv1alpha1.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
	}
	if err := ValidateResourceRuleGVKs(scope, gvks); err != nil {
		t.Fatalf("expected allowed: %v", err)
	}

	gvks[0].Kind = "Pod"
	if err := ValidateResourceRuleGVKs(scope, gvks); err == nil {
		t.Fatal("expected violation")
	}
}

func TestNormalizeNamespaceList(t *testing.T) {
	t.Parallel()

	got := NormalizeNamespaceList([]string{" team-a ", "team-b", "team-a", "", "  "})
	if len(got) != 2 || got[0] != "team-a" || got[1] != "team-b" {
		t.Fatalf("got = %#v", got)
	}

	if NormalizeNamespaceList(nil) != nil {
		t.Fatal("nil input should return nil")
	}
}

func TestCollectRuleGVKs_fromRules(t *testing.T) {
	t.Parallel()

	filter := kollectdevv1alpha1.CollectionFilterSpec{
		ResourceRules: []kollectdevv1alpha1.ResourceRule{
			{GVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}},
			{GVK: kollectdevv1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}},
		},
	}

	gvks := CollectRuleGVKs(filter, kollectdevv1alpha1.GroupVersionKind{Kind: "ignored"})
	if len(gvks) != 2 || gvks[0].Kind != "Deployment" || gvks[1].Kind != "Pod" {
		t.Fatalf("gvks = %#v", gvks)
	}
}
