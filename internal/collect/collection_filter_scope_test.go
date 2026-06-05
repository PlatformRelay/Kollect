// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestScopeCeilingFromScopeAndClusterScope(t *testing.T) {
	t.Parallel()

	if got := ScopeCeilingFromScope(nil); len(got.AllowedNamespaces) != 0 {
		t.Fatalf("nil scope = %#v", got)
	}

	scope := &kollectdevv1alpha1.KollectScope{
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedNamespaces: []string{"team-a"},
				DeniedNamespaces:  []string{"kube-system"},
			},
		},
	}
	if got := ScopeCeilingFromScope(scope); got.AllowedNamespaces[0] != "team-a" ||
		got.DeniedNamespaces[0] != "kube-system" {
		t.Fatalf("scope ceiling = %#v", got)
	}

	clusterScope := &kollectdevv1alpha1.KollectClusterScope{
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedNamespaces: []string{"prod"},
			},
		},
	}
	if got := ScopeCeilingFromClusterScope(clusterScope); got.AllowedNamespaces[0] != "prod" {
		t.Fatalf("cluster scope ceiling = %#v", got)
	}
}

func TestEffectiveNamespaceSetAndComputeFilterStatus(t *testing.T) {
	t.Parallel()

	set := EffectiveNamespaceSet([]string{"b", "a", "a"})
	if len(set) != 2 {
		t.Fatalf("set = %#v", set)
	}
	if EffectiveNamespaceSet(nil) != nil {
		t.Fatal("empty input should return nil set")
	}

	nsMeta := map[string]NamespaceMeta{
		"team-a": {Labels: labels.Set{"team": "a"}},
		"team-b": {Labels: labels.Set{"team": "b"}},
	}
	filter := kollectdevv1alpha1.CollectionFilterSpec{
		IncludedNamespaces: []string{"team-a", "team-b"},
		ResourceRules: []kollectdevv1alpha1.ResourceRule{
			{GVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}},
		},
	}
	ceiling := ScopeCeiling{AllowedNamespaces: []string{"team-a"}}

	matched, effective, rules := ComputeFilterStatus(filter, nil, nsMeta, ceiling, NamespaceDefaults{})
	if len(matched) != 2 || len(effective) != 1 || effective[0] != "team-a" || rules != 1 {
		t.Fatalf("matched=%v effective=%v rules=%d", matched, effective, rules)
	}
}

func TestUnionStringsThroughEffectiveNamespaces(t *testing.T) {
	t.Parallel()

	matched := []string{"team-a", "team-b"}
	ceiling := ScopeCeiling{AllowedNamespaces: []string{"team-b", "team-c"}}
	filter := kollectdevv1alpha1.CollectionFilterSpec{
		IncludedNamespaces: []string{"team-a", "team-c"},
	}

	effective := EffectiveNamespaces(matched, ceiling, filter, NamespaceDefaults{})
	if len(effective) != 1 || effective[0] != "team-b" {
		t.Fatalf("effective = %v", effective)
	}
}

func TestResourceMatchesLegacyNameFilter(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	profile := &kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	target := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			Names: []string{"web", "api"},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "billing"},
			},
		},
	}

	match := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a",
			"labels": map[string]any{"app": "billing"},
		},
	}}
	if !resourceMatchesLegacy(match, target, profile, gvr) {
		t.Fatal("expected name+label match")
	}

	wrongName := match.DeepCopy()
	wrongName.SetName("worker")
	if resourceMatchesLegacy(wrongName, target, profile, gvr) {
		t.Fatal("expected name filter miss")
	}

	wrongGVK := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "pods"}
	if resourceMatchesLegacy(match, target, profile, wrongGVK) {
		t.Fatal("expected GVR mismatch")
	}
}

func TestValidateMatchPolicyExpression(t *testing.T) {
	t.Parallel()

	if err := ValidateMatchPolicyExpression("object.metadata.name == 'demo'"); err != nil {
		t.Fatalf("valid expression: %v", err)
	}

	if err := ValidateMatchPolicyExpression(""); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty expression = %v", err)
	}

	if err := ValidateMatchPolicyExpression("object."); err == nil {
		t.Fatal("expected compile error")
	}
}
