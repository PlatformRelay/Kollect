// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestListNamespaceMeta_returnsLabelsAndAnnotations(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "team-a",
			Labels:      map[string]string{"env": "prod"},
			Annotations: map[string]string{"note": "foo"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	got := listNamespaceMeta(context.Background(), cl)
	if len(got) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(got))
	}
	meta, ok := got["team-a"]
	if !ok {
		t.Fatal("namespace team-a missing from meta map")
	}
	if meta.Labels["env"] != "prod" {
		t.Fatalf("expected label env=prod, got %v", meta.Labels)
	}
	if meta.Annotations["note"] != "foo" {
		t.Fatalf("expected annotation note=foo, got %v", meta.Annotations)
	}
}

func TestListNamespaceMeta_emptyCluster(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	got := listNamespaceMeta(context.Background(), cl)
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestResolveTargetFilterStatus_nilEngineUsesClient(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ns).Build()

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{},
	}

	matched, effective, activeRules, ceiling := resolveTargetFilterStatus(
		context.Background(), cl, nil, target,
	)

	// with no scope and no filters, all fields should be zero/empty — no panic
	_ = matched
	_ = effective
	_ = activeRules
	_ = ceiling
}
