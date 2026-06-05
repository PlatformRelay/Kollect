// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestEngineSetNamespaceDefaultsAndSnapshots(t *testing.T) {
	t.Parallel()

	e := &Engine{
		targets:   make(map[string]targetState),
		nsMeta:    map[string]namespaceMeta{"team-a": {Labels: labels.Set{"team": "a"}}},
		forbidden: make(map[string]struct{}),
		defaults:  NamespaceDefaults{Excluded: []string{"kube-system"}},
	}

	e.SetNamespaceDefaults(NamespaceDefaults{Included: []string{"team-a"}})
	if got := e.NamespaceDefaultsSnapshot(); len(got.Included) != 1 || got.Included[0] != "team-a" {
		t.Fatalf("defaults = %#v", got)
	}

	snap := e.NamespaceMetaSnapshot()
	if len(snap) != 1 || snap["team-a"].Labels["team"] != "a" {
		t.Fatalf("meta snapshot = %#v", snap)
	}
}

func TestEngineNamespaceMatches(t *testing.T) {
	t.Parallel()

	e := &Engine{
		nsMeta: map[string]namespaceMeta{
			"team-a": {Labels: labels.Set{"team": "a"}},
			"team-b": {Labels: labels.Set{"team": "b"}},
		},
	}

	effective := map[string]struct{}{"team-a": {}}
	target := &kollectdevv1alpha1.KollectTarget{}
	if !e.namespaceMatches(target, effective, "team-a") {
		t.Fatal("expected effective namespace match")
	}
	if e.namespaceMatches(target, effective, "team-b") {
		t.Fatal("expected effective namespace miss")
	}

	pinned := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{corev1.LabelMetadataName: "team-a"},
			},
		},
	}
	if !e.namespaceMatches(pinned, nil, "team-a") {
		t.Fatal("expected metadata.name pin match")
	}
	if e.namespaceMatches(pinned, nil, "team-b") {
		t.Fatal("expected metadata.name pin miss")
	}

	selectorTarget := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "a"},
			},
		},
	}
	if !e.namespaceMatches(selectorTarget, nil, "team-a") {
		t.Fatal("expected label selector match")
	}
	if e.namespaceMatches(selectorTarget, nil, "missing") {
		t.Fatal("expected miss for unknown namespace")
	}
}

func TestToUnstructured(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "demo"}}}
	if got := toUnstructured(obj); got != obj {
		t.Fatal("expected direct unstructured passthrough")
	}

	tomb := cache.DeletedFinalStateUnknown{Obj: obj}
	if got := toUnstructured(tomb); got != obj {
		t.Fatal("expected tombstone unwrap")
	}

	if got := toUnstructured("not-a-resource"); got != nil {
		t.Fatalf("expected nil for unknown type, got %#v", got)
	}

	badTomb := cache.DeletedFinalStateUnknown{Obj: "bad"}
	if got := toUnstructured(badTomb); got != nil {
		t.Fatal("expected nil for bad tombstone payload")
	}
}
