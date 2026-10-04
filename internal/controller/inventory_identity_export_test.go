// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
)

// TestKollectInventory_exportCarriesIdentity (IEI-1): the namespaced reconciler's tree export
// reaches the ownership engine with the kind-qualified owner of its own kind, namespace and name.
func TestKollectInventory_exportCarriesIdentity(t *testing.T) {
	t.Parallel()

	rec, backend, invKey := newYAMLGitSetup(t, 1, 1<<20)
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: invKey}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	const want = `["v2","KollectInventory","default","default","team-inventory"]`
	requireOneOwnedCall(t, backend, want)
}

// TestKollectClusterInventory_exportCarriesIdentity (IEI-1, IEI-3): a KollectClusterInventory's tree
// export reaches the ownership engine with owner ["v2","KollectClusterInventory",cluster,"",name] and
// prune requested, not the namespace-"cluster" triple a KollectInventory in namespace cluster shares.
func TestKollectClusterInventory_exportCarriesIdentity(t *testing.T) {
	t.Parallel()

	store, engine, ns, target := readyClusterFixture(t)
	const sinkNS = sink.DefaultSecretNamespace

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "https://example.com/inventory.git",
				Layout:   &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
			},
		},
	}
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: target.Spec.NamespaceSelector,
			TargetRefs:        []string{target.Name},
			SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList("git-platform"),
			SinkNamespace:     sinkNS,
		},
	}

	scheme := clusterRollupScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ns, target, sinkObj, inv).
		WithStatusSubresource(target, sinkObj, inv).
		Build()

	backend := &treeBackend{}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return backend, nil
	})

	rec := &KollectClusterInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Engine: engine, Registry: reg}
	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "platform"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	const want = `["v2","KollectClusterInventory","default","","platform"]`
	requireOneOwnedCall(t, backend, want)
}

func requireOneOwnedCall(t *testing.T, backend *treeBackend, wantOwner string) {
	t.Helper()
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.calls) != 1 {
		t.Fatalf("ExportFiles calls = %d, want 1", len(backend.calls))
	}
	if call := backend.calls[0]; call.pruneOwner != wantOwner || !call.prune {
		t.Fatalf("ExportFiles owner = %s (prune %t), want owner %s with prune requested", call.pruneOwner, call.prune, wantOwner)
	}
}
