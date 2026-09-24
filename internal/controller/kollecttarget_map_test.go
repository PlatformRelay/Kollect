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

func TestKollectTargetReconciler_mapProfileToTargets(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: "team-a"},
	}
	match := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployments"},
	}
	other := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "pods", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "pods"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(match, other).Build()
	r := &KollectTargetReconciler{Client: cl}

	reqs := r.mapProfileToTargets(context.Background(), profile)
	if len(reqs) != 1 || reqs[0].Name != "deploys" {
		t.Fatalf("reqs = %#v", reqs)
	}

	if got := r.mapProfileToTargets(context.Background(), match); got != nil {
		t.Fatalf("non-profile object should return nil, got %#v", got)
	}
}

// K-08: a KollectScope write must enqueue every KollectTarget in the scope's
// namespace (all of them — which scope wins is decided by scope.Load's
// lowest-name rule, so creation or renaming of any scope can change the
// answer), and nothing in other namespaces.
func TestKollectTargetReconciler_mapScopeToTargets(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	scopeObj := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-ceiling", Namespace: "team-a"},
	}
	inNS := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-a"},
	}
	alsoInNS := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "pods", Namespace: "team-a"},
	}
	otherNS := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-b"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(scopeObj, inNS, alsoInNS, otherNS).Build()
	r := &KollectTargetReconciler{Client: cl}

	reqs := r.mapScopeToTargets(context.Background(), scopeObj)
	if len(reqs) != 2 {
		t.Fatalf("reqs = %#v, want the two team-a targets only", reqs)
	}
	names := map[string]struct{}{}
	for _, req := range reqs {
		if req.Namespace != "team-a" {
			t.Fatalf("request %#v outside the scope namespace", req)
		}
		names[req.Name] = struct{}{}
	}
	if len(names) != 2 {
		t.Fatalf("want both team-a targets exactly once, got %#v", reqs)
	}
	for _, want := range []string{"deploys", "pods"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("target %q not enqueued, got %#v", want, reqs)
		}
	}

	if got := r.mapScopeToTargets(context.Background(), inNS); got != nil {
		t.Fatalf("non-scope object should return nil, got %#v", got)
	}
}
