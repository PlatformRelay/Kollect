// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestNamespaceMatchesSelector(t *testing.T) {
	t.Parallel()

	sel := &metav1.LabelSelector{
		MatchLabels: map[string]string{"team": "platform"},
	}

	if !namespaceMatchesSelector(sel, labels.Set{"team": "platform"}) {
		t.Fatal("expected match")
	}

	if namespaceMatchesSelector(sel, labels.Set{"team": "other"}) {
		t.Fatal("expected no match")
	}

	if !namespaceMatchesSelector(nil, labels.Set{}) {
		t.Fatal("nil selector should match all")
	}
}

func TestWatchNamespaceForGVR_singleNamespace(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	engine := &Engine{
		targets: map[string]targetState{
			"ops/demo": {
				target: kollectdevv1alpha1.KollectTarget{
					Spec: kollectdevv1alpha1.KollectTargetSpec{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "prod"},
						},
					},
				},
				profile: kollectdevv1alpha1.KollectProfile{
					Spec: kollectdevv1alpha1.KollectProfileSpec{
						TargetGVK: kollectdevv1alpha1.GroupVersionKind{
							Group: "apps", Version: "v1", Kind: "Deployment",
						},
					},
				},
			},
		},
		nsMeta: map[string]namespaceMeta{
			"team-a": {Labels: labels.Set{"env": "prod"}},
			"team-b": {Labels: labels.Set{"env": "dev"}},
		},
	}

	if got := engine.watchNamespaceForGVR(gvr); got != "team-a" {
		t.Fatalf("watchNamespace = %q, want team-a", got)
	}
}

func TestWatchNamespaceForGVR_allNamespaces(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	engine := &Engine{
		targets: map[string]targetState{
			"ops/a": {
				profile: kollectdevv1alpha1.KollectProfile{
					Spec: kollectdevv1alpha1.KollectProfileSpec{
						TargetGVK: kollectdevv1alpha1.GroupVersionKind{
							Group: "apps", Version: "v1", Kind: "Deployment",
						},
					},
				},
			},
		},
		nsMeta: map[string]namespaceMeta{
			"team-a": {},
			"team-b": {},
		},
	}

	if got := engine.watchNamespaceForGVR(gvr); got != metav1.NamespaceAll {
		t.Fatalf("watchNamespace = %q, want all namespaces", got)
	}
}
