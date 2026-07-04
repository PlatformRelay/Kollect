// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TestFamilySinkConnection_StatusConflictOnFailedTest_Requeues guards against EC-P2-08:
// a ResourceVersion conflict while persisting the "connection test failed" Degraded
// status must propagate so the reconciler requeues, not be swallowed into a nil error
// that silently drops the status write (GUIDELINES §2: Conflict -> requeue, not error-loud).
func TestFamilySinkConnection_StatusConflictOnFailedTest_Requeues(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	obj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "bad-sink",
			Namespace:   "default",
			Annotations: map[string]string{kollectdevv1alpha1.AnnotationTestConnection: "true"},
		},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: "postgres",
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-secret"},
			},
		},
	}

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

	err := conn.reconcile(
		context.Background(), obj, obj.Spec.ToKollectSinkSpec(),
		&obj.Spec.SinkCommonFields, &obj.Status.Conditions, &obj.Status.Preview,
	)
	if err == nil {
		t.Fatal("reconcile() = nil error on status-update conflict; want the conflict to propagate so the caller requeues instead of silently dropping the Degraded status write")
	}
	if !apierrors.IsConflict(err) {
		t.Fatalf("reconcile() error = %v, want a conflict error to propagate unchanged", err)
	}
}
