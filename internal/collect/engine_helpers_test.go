// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestEngineNamespaceMatches pins the K-06 contract: an empty effective set
// under a supplied scope ceiling fails closed (nothing dispatches), while a
// ceiling-free target keeps the selector/pin fallback and an enforced ceiling
// with a non-empty effective set keeps exact membership semantics.
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
	if !e.namespaceMatches(target, effective, true, "team-a") {
		t.Fatal("expected effective namespace match")
	}
	if e.namespaceMatches(target, effective, true, "team-b") {
		t.Fatal("expected effective namespace miss")
	}

	// K-06: ceiling supplied but the effective set is empty (the ceiling
	// excluded every matched namespace, or the selector matched nothing at
	// registration) — the ceiling must NOT silently switch off.
	labelTarget := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "a"},
			},
		},
	}
	if e.namespaceMatches(labelTarget, nil, true, "team-a") {
		t.Fatal("enforced ceiling with empty effective set must deny selector-matching namespaces (K-06)")
	}
	if e.namespaceMatches(labelTarget, nil, true, "team-b") {
		t.Fatal("enforced ceiling with empty effective set must deny every non-pinned namespace (K-06)")
	}
	if e.namespaceMatches(&kollectdevv1alpha1.KollectTarget{}, nil, true, "team-a") {
		t.Fatal("enforced ceiling with empty effective set must deny even unknown namespaces (K-06)")
	}

	// The metadata.name pin (cluster-synthetic registrations) stays valid under
	// enforcement, and only for its own namespace.
	pinned := &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{corev1.LabelMetadataName: "team-a"},
			},
		},
	}
	if !e.namespaceMatches(pinned, nil, true, "team-a") {
		t.Fatal("expected metadata.name pin match under enforcement")
	}
	if e.namespaceMatches(pinned, nil, true, "team-b") {
		t.Fatal("expected metadata.name pin miss under enforcement")
	}

	// Without a ceiling the historical fallback is unchanged: selector matches
	// live namespace labels, pin matches its own namespace.
	if !e.namespaceMatches(pinned, nil, false, "team-a") {
		t.Fatal("expected metadata.name pin match without ceiling")
	}
	if e.namespaceMatches(pinned, nil, false, "team-b") {
		t.Fatal("expected metadata.name pin miss without ceiling")
	}
	if !e.namespaceMatches(labelTarget, nil, false, "team-a") {
		t.Fatal("expected label selector match without ceiling")
	}
	if e.namespaceMatches(labelTarget, nil, false, "missing") {
		t.Fatal("expected miss for unknown namespace without ceiling")
	}
}
