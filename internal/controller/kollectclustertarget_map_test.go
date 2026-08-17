// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestKollectClusterTargetReconciler_mapFunctions(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-deployments", Namespace: "kollect-system"},
	}
	match := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct-match"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "platform-deployments", Namespace: "kollect-system"},
		},
	}
	other := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct-other"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "other", Namespace: "kollect-system"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(match, other).Build()
	r := &KollectClusterTargetReconciler{Client: cl}

	reqs := r.mapProfileToClusterTargets(context.Background(), profile)
	if len(reqs) != 1 || reqs[0].Name != "ct-match" {
		t.Fatalf("profile map reqs = %#v", reqs)
	}

	reqs = r.mapNamespaceToClusterTargets(context.Background(), nil)
	if len(reqs) != 2 {
		t.Fatalf("namespace map reqs = %#v", reqs)
	}

	if got := r.mapProfileToClusterTargets(context.Background(), match); got != nil {
		t.Fatalf("non-profile object should return nil, got %#v", got)
	}
}

func TestKollectClusterTargetReconciler_mapClusterScopeToClusterTargets(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	first := &kollectdevv1alpha1.KollectClusterTarget{ObjectMeta: metav1.ObjectMeta{Name: "ct-a"}}
	second := &kollectdevv1alpha1.KollectClusterTarget{ObjectMeta: metav1.ObjectMeta{Name: "ct-b"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(first, second).Build()
	r := &KollectClusterTargetReconciler{Client: cl}

	// Any KollectClusterScope write can change the enforced ceiling — LoadCluster
	// picks the lowest-named object of all of them — so every target re-reconciles
	// regardless of which scope object was written.
	scopeObj := &kollectdevv1alpha1.KollectClusterScope{ObjectMeta: metav1.ObjectMeta{Name: "zz-not-enforced"}}
	reqs := r.mapClusterScopeToClusterTargets(context.Background(), scopeObj)
	if len(reqs) != 2 {
		t.Fatalf("cluster scope map reqs = %#v, want one per cluster target", reqs)
	}

	names := map[string]bool{}
	for _, req := range reqs {
		if req.Namespace != "" {
			t.Fatalf("cluster-scoped request must not carry a namespace: %#v", req)
		}
		names[req.Name] = true
	}
	if !names["ct-a"] || !names["ct-b"] {
		t.Fatalf("cluster scope map reqs = %#v, want ct-a and ct-b", reqs)
	}

	if got := r.mapClusterScopeToClusterTargets(context.Background(), first); got != nil {
		t.Fatalf("non-scope object should return nil, got %#v", got)
	}
}
