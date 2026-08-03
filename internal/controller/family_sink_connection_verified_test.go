// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func boolPtr(b bool) *bool { return &b }

func databaseSinkWithTestAnnotation(name string) *kollectdevv1alpha1.KollectDatabaseSink {
	return &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Generation:  1,
			Annotations: map[string]string{kollectdevv1alpha1.AnnotationTestConnection: "true"},
		},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{Type: "postgres"},
	}
}

// TestSetConnectionVerified_ClearsOneShotAnnotation covers the shouldClear branch of
// setConnectionVerified (family_sink_connection.go:123-131) together with
// shouldClearFamilyTestConnectionAnnotation returning true: a one-shot annotation-driven
// test (connectionTest not enabled on the spec) has its trigger annotation removed once the
// probe succeeds, and the ConnectionVerified condition is set True.
func TestSetConnectionVerified_ClearsOneShotAnnotation(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	obj := databaseSinkWithTestAnnotation("one-shot")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		WithStatusSubresource(obj).
		Build()

	conn := familySinkConnection{client: cl}
	// ConnectionTest=false => not spec-enabled => the one-shot annotation should be cleared.
	common := &kollectdevv1alpha1.SinkCommonFields{ConnectionTest: boolPtr(false)}

	err := conn.setConnectionVerified(
		context.Background(), obj, obj.Spec.ToKollectSinkSpec(), common, &obj.Status.Conditions, "ok",
	)
	if err != nil {
		t.Fatalf("setConnectionVerified() error = %v", err)
	}

	cond := apimeta.FindStatusCondition(obj.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("ConnectionVerified = %+v, want True", cond)
	}

	var got kollectdevv1alpha1.KollectDatabaseSink
	if getErr := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), &got); getErr != nil {
		t.Fatalf("Get sink: %v", getErr)
	}
	if _, ok := got.Annotations[kollectdevv1alpha1.AnnotationTestConnection]; ok {
		t.Fatal("one-shot test-connection annotation was not cleared after a successful verify")
	}
}

// TestSetConnectionVerified_KeepsAnnotationWhenSpecEnabled covers the shouldClear=false
// branch (spec.connectionTest enabled): the standing annotation is left in place because the
// probe is spec-driven, not one-shot.
func TestSetConnectionVerified_KeepsAnnotationWhenSpecEnabled(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	obj := databaseSinkWithTestAnnotation("spec-enabled")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		WithStatusSubresource(obj).
		Build()

	conn := familySinkConnection{client: cl}
	// ConnectionTest=true => spec-enabled => shouldClear returns false, annotation retained.
	common := &kollectdevv1alpha1.SinkCommonFields{ConnectionTest: boolPtr(true)}

	if err := conn.setConnectionVerified(
		context.Background(), obj, obj.Spec.ToKollectSinkSpec(), common, &obj.Status.Conditions, "ok",
	); err != nil {
		t.Fatalf("setConnectionVerified() error = %v", err)
	}

	var got kollectdevv1alpha1.KollectDatabaseSink
	if getErr := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), &got); getErr != nil {
		t.Fatalf("Get sink: %v", getErr)
	}
	if _, ok := got.Annotations[kollectdevv1alpha1.AnnotationTestConnection]; !ok {
		t.Fatal("spec-enabled connection test must not strip the annotation")
	}
}

// TestSetConnectionVerified_StatusUpdateError_Propagates covers the status-update error
// return (family_sink_connection.go:119-120): a failed status write propagates so the caller
// can requeue instead of silently claiming the sink verified.
func TestSetConnectionVerified_StatusUpdateError_Propagates(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	obj := databaseSinkWithTestAnnotation("status-broken")
	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollectdatabasesinks"}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		WithStatusSubresource(obj).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(
				_ context.Context, _ client.Client, _ string, o client.Object, _ ...client.SubResourceUpdateOption,
			) error {
				return apierrors.NewConflict(gr, o.GetName(), errors.New("resourceVersion conflict"))
			},
		}).
		Build()

	conn := familySinkConnection{client: cl}
	common := &kollectdevv1alpha1.SinkCommonFields{ConnectionTest: boolPtr(false)}

	err := conn.setConnectionVerified(
		context.Background(), obj, obj.Spec.ToKollectSinkSpec(), common, &obj.Status.Conditions, "ok",
	)
	if err == nil {
		t.Fatal("setConnectionVerified() = nil on a failed status update; want the error to propagate for a requeue")
	}
	if !apierrors.IsConflict(err) {
		t.Fatalf("setConnectionVerified() error = %v, want the conflict to propagate unchanged", err)
	}
}
