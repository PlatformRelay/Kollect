// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// counterpartGetError makes Get of the counterpart kind (the other export
// scope) fail with err; every other Get passes through. lookups counts the
// counterpart lookups so a test can prove none happened.
func counterpartGetError(counterpart client.Object, err error, lookups *int) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if sameKind(obj, counterpart) {
				*lookups++
				if err != nil {
					return err
				}
			}

			return c.Get(ctx, key, obj, opts...)
		},
	}
}

func sameKind(a, b client.Object) bool {
	switch a.(type) {
	case *kollectdevv1alpha1.KollectClusterInventory:
		_, ok := b.(*kollectdevv1alpha1.KollectClusterInventory)
		return ok
	case *kollectdevv1alpha1.KollectInventory:
		_, ok := b.(*kollectdevv1alpha1.KollectInventory)
		return ok
	default:
		return false
	}
}

// namespacedInventoryInClusterNamespace is a deleting KollectInventory "platform"
// in namespace "cluster" bound to a Delete-policy git sink there: its export
// identity is inventory/cluster/platform, the same as KollectClusterInventory
// "platform".
func namespacedInventoryInClusterNamespace(policy string) (*kollectdevv1alpha1.KollectSnapshotSink, *kollectdevv1alpha1.KollectInventory) {
	sinkObj, inv := deletingInventoryWithSnapshotSink("git-shared")
	sinkObj.Namespace = clusterExportNamespace
	sinkObj.Spec.DeletionPolicy = policy
	inv.Namespace = clusterExportNamespace
	inv.Name = "platform"

	return sinkObj, inv
}

type sharedIdentityCase struct {
	lookupErr      error
	opts           RuntimeOptions
	policy         string
	wantLookups    int
	wantRetraction bool
	wantErr        bool
	wantFinalizer  bool
}

func runNamespacedSharedIdentityCase(t *testing.T, tc sharedIdentityCase) {
	t.Helper()

	scheme := cleanupPolicyScheme(t)
	sinkObj, inv := namespacedInventoryInClusterNamespace(tc.policy)

	lookups := 0
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(sinkObj, inv).WithStatusSubresource(sinkObj, inv).
		WithInterceptorFuncs(counterpartGetError(&kollectdevv1alpha1.KollectClusterInventory{}, tc.lookupErr, &lookups)).
		Build()

	tomb := &tombstoneBackend{}
	built := 0
	rec := &KollectInventoryReconciler{
		Client: cl, Scheme: cl.Scheme(), Store: collect.NewStore(),
		Registry: gitRegistryCounting(tomb, &built), Recorder: record.NewFakeRecorder(10),
		Options: tc.opts,
	}

	key := types.NamespacedName{Name: inv.Name, Namespace: inv.Namespace}
	_, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: key})
	assertSharedIdentityOutcome(t, tc, err, lookups, len(tomb.deleted) > 0, func() bool {
		var got kollectdevv1alpha1.KollectInventory
		return cl.Get(context.Background(), key, &got) == nil && containsFinalizer(got.Finalizers, inventoryCleanupFinalizer)
	})
}

func assertSharedIdentityOutcome(t *testing.T, tc sharedIdentityCase, err error, lookups int, retracted bool, finalizerKept func() bool) {
	t.Helper()

	if (err != nil) != tc.wantErr {
		t.Fatalf("Reconcile err = %v, want error %v", err, tc.wantErr)
	}
	if lookups != tc.wantLookups {
		t.Fatalf("counterpart lookups = %d, want %d", lookups, tc.wantLookups)
	}
	if retracted != tc.wantRetraction {
		t.Fatalf("retraction ran = %v, want %v", retracted, tc.wantRetraction)
	}
	if kept := finalizerKept(); kept != tc.wantFinalizer {
		t.Fatalf("finalizer kept = %v, want %v", kept, tc.wantFinalizer)
	}
}

var errLookupTimeout = apierrors.NewServerTimeout(schema.GroupResource{Group: "kollect.dev", Resource: "kollectclusterinventories"}, "get", 1)

// R2-02: the shared-identity lookup decides, per lookup result, whether the
// Delete retraction may run. NotFound and a structurally unreadable other
// scope (Forbidden: the operator cannot have exported it) are "not shared";
// any other error is an unknown that must not cost the retraction: cleanup
// fails transiently and keeps the finalizer.
func TestKollectInventoryReconciler_sharedIdentityLookupOutcomes(t *testing.T) {
	t.Parallel()

	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: "kollect.dev", Resource: "kollectclusterinventories"}, "platform", errors.New("namespaced RBAC"))

	cases := map[string]sharedIdentityCase{
		"not found retracts": {
			lookupErr: nil, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			wantLookups: 1, wantRetraction: true,
		},
		"forbidden counterpart scope retracts": {
			lookupErr: forbidden, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			wantLookups: 1, wantRetraction: true,
		},
		"transient lookup error keeps the finalizer": {
			lookupErr: errLookupTimeout, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			wantLookups: 1, wantErr: true, wantFinalizer: true,
		},
		"tenant mode never looks up the cluster scope": {
			lookupErr: errLookupTimeout, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			opts:        RuntimeOptions{TenantMode: true},
			wantLookups: 0, wantRetraction: true,
		},
		"retain policy needs no lookup": {
			lookupErr: errLookupTimeout, policy: "",
			wantLookups: 0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runNamespacedSharedIdentityCase(t, tc)
		})
	}
}

// R2-02, cluster side: the counterpart KollectInventory lives in namespace
// "cluster". When the operator does not watch that namespace it cannot have
// exported such an inventory, so the identity is not shared and no lookup runs.
func TestKollectClusterInventoryReconciler_sharedIdentityLookupOutcomes(t *testing.T) {
	t.Parallel()

	cases := map[string]sharedIdentityCase{
		"not found retracts": {
			policy: kollectdevv1alpha1.DeletionPolicyDelete, wantLookups: 1, wantRetraction: true,
		},
		"transient lookup error keeps the finalizer": {
			lookupErr: errLookupTimeout, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			wantLookups: 1, wantErr: true, wantFinalizer: true,
		},
		"unwatched counterpart namespace retracts without lookup": {
			lookupErr: errLookupTimeout, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			opts:        RuntimeOptions{WatchNamespaces: []string{"team-a", "kollect-system"}},
			wantLookups: 0, wantRetraction: true,
		},
		"watched counterpart namespace is looked up": {
			lookupErr: errLookupTimeout, policy: kollectdevv1alpha1.DeletionPolicyDelete,
			opts:        RuntimeOptions{WatchNamespaces: []string{clusterExportNamespace}},
			wantLookups: 1, wantErr: true, wantFinalizer: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme := cleanupPolicyScheme(t)
			now := metav1.Now()
			inv := &kollectdevv1alpha1.KollectClusterInventory{
				ObjectMeta: metav1.ObjectMeta{
					Name: "platform", Finalizers: []string{clusterInventoryCleanupFinalizer}, DeletionTimestamp: &now,
				},
				Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
					SinkNamespace:    "kollect-system",
					SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("git-shared"),
				},
			}
			sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
				ObjectMeta: metav1.ObjectMeta{Name: "git-shared", Namespace: "kollect-system"},
				Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
					Type: kollectdevv1alpha1.SnapshotSinkTypeGit, DeletionPolicy: tc.policy,
				},
			}

			lookups := 0
			cl := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(sinkObj, inv).WithStatusSubresource(sinkObj, inv).
				WithInterceptorFuncs(counterpartGetError(&kollectdevv1alpha1.KollectInventory{}, tc.lookupErr, &lookups)).
				Build()

			tomb := &tombstoneBackend{}
			built := 0
			rec := &KollectClusterInventoryReconciler{
				Client: cl, Scheme: cl.Scheme(), Store: collect.NewStore(),
				Registry: gitRegistryCounting(tomb, &built), Recorder: record.NewFakeRecorder(10),
				Options: tc.opts,
			}

			key := types.NamespacedName{Name: inv.Name}
			_, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: key})
			assertSharedIdentityOutcome(t, tc, err, lookups, len(tomb.deleted) > 0, func() bool {
				var got kollectdevv1alpha1.KollectClusterInventory
				return cl.Get(context.Background(), key, &got) == nil && containsFinalizer(got.Finalizers, clusterInventoryCleanupFinalizer)
			})
		})
	}
}
