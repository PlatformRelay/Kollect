// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestMatchIntentNamespaces_gates(t *testing.T) {
	t.Parallel()

	nsMeta := map[string]NamespaceMeta{
		"team-a":  {Labels: labels.Set{"tier": "app"}},
		"team-b":  {Labels: labels.Set{"tier": "app", "skip": "true"}},
		"infra-x": {Labels: labels.Set{"tier": "infra"}},
	}

	tests := []struct {
		name         string
		filter       kollectdevv1alpha1.CollectionFilterSpec
		namespaceSel *metav1.LabelSelector
		want         []string
	}{
		{
			name:   "include list drops unlisted namespaces",
			filter: kollectdevv1alpha1.CollectionFilterSpec{IncludedNamespaces: []string{"team-a"}},
			want:   []string{"team-a"},
		},
		{
			name:         "namespaceSelector drops non-matching namespaces",
			namespaceSel: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "app"}},
			want:         []string{"team-a", "team-b"},
		},
		{
			name: "namespaceExcludeSelector drops matching namespaces",
			filter: kollectdevv1alpha1.CollectionFilterSpec{
				NamespaceExcludeSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"skip": "true"}},
			},
			want: []string{"infra-x", "team-a"},
		},
		{
			name:   "excluded list beats include list",
			filter: kollectdevv1alpha1.CollectionFilterSpec{ExcludedNamespaces: []string{"infra-x"}},
			want:   []string{"team-a", "team-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MatchIntentNamespaces(tt.filter, tt.namespaceSel, nsMeta, NamespaceDefaults{})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MatchIntentNamespaces() = %v, want %v (sorted)", got, tt.want)
			}
		})
	}
}

func TestCompileResourceRules_errors(t *testing.T) {
	t.Parallel()

	env := matchPolicyEnv(t)
	validGVK := kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}

	tests := []struct {
		name    string
		rule    kollectdevv1alpha1.ResourceRule
		wantMsg string
	}{
		{
			name:    "missing version and kind",
			rule:    kollectdevv1alpha1.ResourceRule{},
			wantMsg: "version and kind are required",
		},
		{
			name: "invalid namespaceSelector",
			rule: kollectdevv1alpha1.ResourceRule{
				GVK: validGVK,
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k", Operator: "Bogus"}},
				},
			},
			wantMsg: "namespaceSelector",
		},
		{
			name: "invalid matchExpressions",
			rule: kollectdevv1alpha1.ResourceRule{
				GVK: validGVK,
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "k", Operator: "NotAnOperator"},
				},
			},
			wantMsg: "resourceRules[0]",
		},
		{
			name: "invalid matchPolicy CEL",
			rule: kollectdevv1alpha1.ResourceRule{
				GVK:         validGVK,
				MatchPolicy: "object.metadata ==",
			},
			wantMsg: "matchPolicy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := CompileResourceRules([]kollectdevv1alpha1.ResourceRule{tt.rule}, env)
			if err == nil {
				t.Fatalf("CompileResourceRules() = %v, want error containing %q", compiled, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestResourceMatchesRules_skipBranches(t *testing.T) {
	t.Parallel()

	env := matchPolicyEnv(t)
	cmGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	secretsGVR := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

	rules, err := CompileResourceRules([]kollectdevv1alpha1.ResourceRule{{
		GVK:               kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "app"}},
	}}, env)
	if err != nil {
		t.Fatalf("CompileResourceRules: %v", err)
	}

	target := &kollectdevv1alpha1.KollectTarget{}
	profile := &kollectdevv1alpha1.KollectProfile{}
	nsMeta := map[string]NamespaceMeta{
		"team-a": {Labels: labels.Set{"tier": "app"}},
		"infra":  {Labels: labels.Set{"tier": "infra"}},
	}

	objIn := func(ns string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "o"}}}
		if ns != "" {
			u.SetNamespace(ns)
		}

		return u
	}

	if ResourceMatchesRules(objIn("team-a"), secretsGVR, target, profile, rules, nsMeta) {
		t.Fatal("GVR mismatch must not match")
	}

	if ResourceMatchesRules(objIn("infra"), cmGVR, target, profile, rules, nsMeta) {
		t.Fatal("namespaceSelector mismatch must not match")
	}

	if ResourceMatchesRules(objIn("unknown-ns"), cmGVR, target, profile, rules, nsMeta) {
		t.Fatal("namespace absent from meta has no labels and must not match a label-gated rule")
	}

	// Cluster-scoped objects fall back to the "default" namespace for rule matching.
	defaultLabeled := map[string]NamespaceMeta{"default": {Labels: labels.Set{"tier": "app"}}}
	if !ResourceMatchesRules(objIn(""), cmGVR, target, profile, rules, defaultLabeled) {
		t.Fatal("empty-namespace object must be evaluated against the default namespace meta")
	}
}

func TestResourceMatchesLegacy_labelSelectorBranches(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	profile := &kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Secret"},
		},
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":   "s1",
			"labels": map[string]any{"app": "web"},
		},
	}}

	invalidTarget := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k", Operator: "Nope"}},
			},
		},
	}
	if ResourceMatchesRules(obj, gvr, invalidTarget, profile, nil, nil) {
		t.Fatal("invalid legacy label selector must fail closed (no match)")
	}

	mismatchTarget := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
		},
	}
	if ResourceMatchesRules(obj, gvr, mismatchTarget, profile, nil, nil) {
		t.Fatal("non-matching legacy label selector must not match")
	}
}

// stubCELProgram lets tests drive evalMatchPolicy with results a real compiled
// expression cannot produce (nil values, injected eval errors).
type stubCELProgram struct {
	val ref.Val
	err error
}

func (s stubCELProgram) Eval(any) (ref.Val, *cel.EvalDetails, error) { return s.val, nil, s.err }

func (s stubCELProgram) ContextEval(context.Context, any) (ref.Val, *cel.EvalDetails, error) {
	return s.val, nil, s.err
}

func TestEvalMatchPolicy_resultShapes(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{}}

	t.Run("eval error propagates", func(t *testing.T) {
		t.Parallel()

		_, err := evalMatchPolicy(stubCELProgram{err: errors.New("boom")}, obj)
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want propagated eval error", err)
		}
	})

	t.Run("bool-convertible ref.Val is accepted", func(t *testing.T) {
		t.Parallel()

		got, err := evalMatchPolicy(stubCELProgram{val: types.String("true")}, obj)
		if err != nil {
			t.Fatalf("evalMatchPolicy error = %v", err)
		}
		if !got {
			t.Fatal("String(\"true\") must convert to bool true")
		}
	})

	t.Run("nil result is rejected", func(t *testing.T) {
		t.Parallel()

		_, err := evalMatchPolicy(stubCELProgram{}, obj)
		if err == nil || !strings.Contains(err.Error(), "not bool") {
			t.Fatalf("err = %v, want not-bool rejection for nil result", err)
		}
	})
}

func TestSortedUniqueStrings(t *testing.T) {
	t.Parallel()

	if got := sortedUniqueStrings(nil); got != nil {
		t.Fatalf("sortedUniqueStrings(nil) = %v, want nil", got)
	}

	got := sortedUniqueStrings([]string{"zeta", "", "alpha", "zeta", "mid", ""})
	want := []string{"alpha", "mid", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedUniqueStrings() = %v, want %v (sorted, deduped, no empties)", got, want)
	}
}

func TestScopeCeilingFromClusterScope(t *testing.T) {
	t.Parallel()

	empty := ScopeCeilingFromClusterScope(nil)
	if len(empty.AllowedNamespaces) != 0 || len(empty.DeniedNamespaces) != 0 {
		t.Fatalf("nil cluster scope must yield empty ceiling, got %+v", empty)
	}

	scope := &kollectdevv1alpha1.KollectClusterScope{}
	scope.Spec.AllowedNamespaces = []string{"a"}
	scope.Spec.DeniedNamespaces = []string{"kube-system"}

	got := ScopeCeilingFromClusterScope(scope)
	if !reflect.DeepEqual(got.AllowedNamespaces, []string{"a"}) ||
		!reflect.DeepEqual(got.DeniedNamespaces, []string{"kube-system"}) {
		t.Fatalf("ScopeCeilingFromClusterScope() = %+v", got)
	}

	got.AllowedNamespaces[0] = "mutated"
	if scope.Spec.AllowedNamespaces[0] != "a" {
		t.Fatal("ceiling must copy, not alias, the scope spec slices")
	}
}
