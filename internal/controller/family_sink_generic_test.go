// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// AR-08/F68: FamilySnapshotSinkReconciler, FamilyDatabaseSinkReconciler, and
// FamilyEventSinkReconciler are all thin wrappers around the same
// guardReconcile + familySinkConnection.reconcile pattern. Before this lane
// only KollectSnapshotSink had direct Reconcile()-entrypoint coverage
// (envtest + panic-recovery); these generic helpers drive the shared
// FamilySinkReconciler[T,PT] for all three kinds so Database and Event get
// the same coverage for free.

func newFamilySinkGenericTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return scheme
}

func runFamilySinkReconcilerPanicRecoveryTest[T any, PT familySinkPtr[T]](t *testing.T, mkObj func() PT) {
	t.Helper()
	scheme := newFamilySinkGenericTestScheme(t)
	obj := mkObj()

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		WithStatusSubresource(obj).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(
				context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption,
			) error {
				panic("injected status update panic")
			},
		}).
		Build()

	rec := &FamilySinkReconciler[T, PT]{Client: cl, Scheme: scheme, Name: "test"}

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil after recovered panic", err)
	}
	if result.RequeueAfter == 0 && !result.Requeue { //nolint:staticcheck // SA1019: guard uses Requeue for immediate requeue
		t.Fatal("expected requeue after recovered panic")
	}
}

func runFamilySinkReconcilerNotFoundTest[T any, PT familySinkPtr[T]](t *testing.T) {
	t.Helper()
	scheme := newFamilySinkGenericTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	rec := &FamilySinkReconciler[T, PT]{Client: cl, Scheme: scheme, Name: "test"}

	result, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "missing", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile err = %v, want nil for NotFound", err)
	}
	if result.Requeue || result.RequeueAfter != 0 { //nolint:staticcheck // SA1019: guard uses Requeue for immediate requeue
		t.Fatalf("result = %+v, want empty", result)
	}
}

func runFamilySinkReconcilerPreviewHappyPathTest[T any, PT familySinkPtr[T]](t *testing.T, mkObj func() PT) {
	t.Helper()
	scheme := newFamilySinkGenericTestScheme(t)
	obj := mkObj()

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).WithStatusSubresource(obj).Build()
	rec := &FamilySinkReconciler[T, PT]{Client: cl, Scheme: scheme, Name: "test"}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()},
	}); err != nil {
		t.Fatalf("Reconcile err = %v", err)
	}

	updated := PT(new(T))
	if err := cl.Get(context.Background(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, updated); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.FamilySinkStatus().Preview == nil {
		t.Fatal("expected status.preview to be rendered from the kollect.dev/preview annotation")
	}
}

func withPreviewAnnotation(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   "default",
		Annotations: map[string]string{kollectdevv1alpha1.AnnotationPreview: "true"},
	}
}

func TestFamilySinkReconciler_Snapshot_recoversReconcilePanic(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerPanicRecoveryTest[kollectdevv1alpha1.KollectSnapshotSink](t,
		func() *kollectdevv1alpha1.KollectSnapshotSink {
			return &kollectdevv1alpha1.KollectSnapshotSink{ObjectMeta: withPreviewAnnotation("snap-panic")}
		})
}

func TestFamilySinkReconciler_Database_recoversReconcilePanic(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerPanicRecoveryTest[kollectdevv1alpha1.KollectDatabaseSink](t,
		func() *kollectdevv1alpha1.KollectDatabaseSink {
			return &kollectdevv1alpha1.KollectDatabaseSink{ObjectMeta: withPreviewAnnotation("db-panic")}
		})
}

func TestFamilySinkReconciler_Event_recoversReconcilePanic(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerPanicRecoveryTest[kollectdevv1alpha1.KollectEventSink](t,
		func() *kollectdevv1alpha1.KollectEventSink {
			return &kollectdevv1alpha1.KollectEventSink{ObjectMeta: withPreviewAnnotation("event-panic")}
		})
}

func TestFamilySinkReconciler_Snapshot_notFound(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerNotFoundTest[kollectdevv1alpha1.KollectSnapshotSink](t)
}

func TestFamilySinkReconciler_Database_notFound(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerNotFoundTest[kollectdevv1alpha1.KollectDatabaseSink](t)
}

func TestFamilySinkReconciler_Event_notFound(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerNotFoundTest[kollectdevv1alpha1.KollectEventSink](t)
}

func TestFamilySinkReconciler_Database_previewHappyPath(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerPreviewHappyPathTest[kollectdevv1alpha1.KollectDatabaseSink](t,
		func() *kollectdevv1alpha1.KollectDatabaseSink {
			return &kollectdevv1alpha1.KollectDatabaseSink{
				ObjectMeta: withPreviewAnnotation("db-preview"),
				Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
					Type: "postgres",
				},
			}
		})
}

func TestFamilySinkReconciler_Event_previewHappyPath(t *testing.T) {
	t.Parallel()
	runFamilySinkReconcilerPreviewHappyPathTest[kollectdevv1alpha1.KollectEventSink](t,
		func() *kollectdevv1alpha1.KollectEventSink {
			return &kollectdevv1alpha1.KollectEventSink{
				ObjectMeta: withPreviewAnnotation("event-preview"),
				Spec: kollectdevv1alpha1.KollectEventSinkSpec{
					Type: "nats",
				},
			}
		})
}
