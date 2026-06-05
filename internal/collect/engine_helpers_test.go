// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

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
