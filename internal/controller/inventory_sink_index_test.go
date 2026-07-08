// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// AR-09: mapFamilySinkToInventories must use an indexed field lookup (client.MatchingFields)
// instead of a full client.List of every KollectInventory in the namespace followed by an
// in-memory filter — so a watch event on one sink doesn't cost O(all inventories in namespace).
// The mapper drops its in-memory filter, so this exact-set assertion holds only if the index
// key actually restricts the List result to the bound inventory.
func TestMapFamilySinkToInventories_UsesFieldIndex(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	bound := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "bound-inv", Namespace: "ns-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "my-sink"}},
		},
	}
	unbound := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "unbound-inv", Namespace: "ns-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "other-sink"}},
		},
	}
	otherFamily := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "other-family-inv", Namespace: "ns-a"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "my-sink"}},
		},
	}
	otherNamespace := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "other-ns-inv", Namespace: "ns-b"},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "my-sink"}},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&kollectdevv1alpha1.KollectInventory{}, inventorySinkFieldIndex, indexInventorySinkBindings).
		WithObjects(bound, unbound, otherFamily, otherNamespace).
		Build()

	r := &KollectInventoryReconciler{Client: fakeClient}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sink", Namespace: "ns-a"},
	}

	reqs := r.mapSnapshotSinkToInventories(context.Background(), sinkObj)

	names := make([]string, 0, len(reqs))
	for _, req := range reqs {
		if req.Namespace != "ns-a" {
			t.Fatalf("unexpected namespace in request: %#v", req)
		}
		names = append(names, req.Name)
	}
	sort.Strings(names)

	if len(names) != 1 || names[0] != "bound-inv" {
		t.Fatalf("requests = %v, want exactly [bound-inv]", names)
	}
}

// AR-09 (cluster variant): same indexed-lookup fix for KollectClusterInventory's sink-watch
// mappers, which additionally resolve each binding's effective namespace (spec.sinkNamespace,
// falling back to sink.DefaultSecretNamespace) before matching.
func TestMapClusterFamilySinkToInventories_UsesFieldIndex(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	bound := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "bound-cinv"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace:    "ns-a",
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "my-sink"}},
		},
	}
	wrongNamespace := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "wrong-ns-cinv"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace:    "ns-b",
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "my-sink"}},
		},
	}
	unbound := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "unbound-cinv"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace:    "ns-a",
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "other-sink"}},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(
			&kollectdevv1alpha1.KollectClusterInventory{},
			clusterInventorySinkFieldIndex, indexClusterInventorySinkBindings,
		).
		WithObjects(bound, wrongNamespace, unbound).
		Build()

	r := &KollectClusterInventoryReconciler{Client: fakeClient}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sink", Namespace: "ns-a"},
	}

	reqs := r.mapClusterSnapshotSinkToInventories(context.Background(), sinkObj)

	names := make([]string, 0, len(reqs))
	for _, req := range reqs {
		names = append(names, req.Name)
	}
	sort.Strings(names)

	if len(names) != 1 || names[0] != "bound-cinv" {
		t.Fatalf("requests = %v, want exactly [bound-cinv]", names)
	}
}
