// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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
