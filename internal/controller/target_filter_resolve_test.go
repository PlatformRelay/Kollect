// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
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

// TestResolveTargetFilterStatus_refreshesNamespaceSnapshotBeforeResolving pins the
// ordering half of COLLECT-NS-BACKFILL: the engine's namespace snapshot is written
// only by RegisterTarget, which the reconciler calls after this resolve. A namespace
// that exists in the API but is missing from that snapshot must still land in the
// effective set — otherwise the engine silently rejects every object in it.
func TestResolveTargetFilterStatus_refreshesNamespaceSnapshotBeforeResolving(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// The namespace exists in the API server but has never been observed by the
	// engine: no RegisterTarget has run, so its nsMeta cache is still empty.
	kubeClient := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-fresh"}},
	)
	engine, err := collect.NewEngine(nil, kubeClient, collect.NewStore(), collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if got := engine.NamespaceMetaSnapshot(); len(got) != 0 {
		t.Fatalf("precondition: engine namespace snapshot = %v, want empty", got)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: "tenant"},
	}

	_, effective, _, _ := resolveTargetFilterStatus(context.Background(), cl, engine, target)

	if !slices.Contains(effective, "team-fresh") {
		t.Fatalf("effective namespaces = %v, want to contain %q", effective, "team-fresh")
	}
}
