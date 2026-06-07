// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestRenderFamilyPreview_setsAndClears(t *testing.T) {
	t.Parallel()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{Schema: "public", Table: "inventory"},
	}

	var target *kollectdevv1alpha1.SinkPreviewStatus

	enabled := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pg",
			Annotations: map[string]string{kollectdevv1alpha1.AnnotationPreview: "true"},
		},
	}
	if !renderFamilyPreview(enabled, spec, &target) {
		t.Fatal("expected change when preview annotation set")
	}
	if target == nil || target.Postgres == nil || target.Postgres.ExpectedDDL == "" {
		t.Fatalf("expected postgres preview to be rendered, got %#v", target)
	}

	disabled := &kollectdevv1alpha1.KollectDatabaseSink{ObjectMeta: metav1.ObjectMeta{Name: "pg"}}
	if !renderFamilyPreview(disabled, spec, &target) {
		t.Fatal("expected change when clearing a previously-rendered preview")
	}
	if target != nil {
		t.Fatalf("expected preview cleared, got %#v", target)
	}

	if renderFamilyPreview(disabled, spec, &target) {
		t.Fatal("expected no change when annotation absent and no prior preview")
	}
}

func TestFamilySinkConnection_persistsPreviewWithoutConnectionTest(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sink := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pg",
			Namespace:   "default",
			Annotations: map[string]string{kollectdevv1alpha1.AnnotationPreview: "true"},
		},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type:     kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{Schema: "public", Table: "inventory"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sink).WithStatusSubresource(sink).Build()

	conn := familySinkConnection{client: cl}
	err := conn.reconcile(
		context.Background(),
		sink,
		sink.Spec.ToKollectSinkSpec(),
		&sink.Spec.SinkCommonFields,
		&sink.Status.Conditions,
		&sink.Status.Preview,
	)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectDatabaseSink
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "pg", Namespace: "default"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Preview == nil || got.Status.Preview.Postgres == nil {
		t.Fatalf("expected persisted postgres preview, got %#v", got.Status.Preview)
	}
}
