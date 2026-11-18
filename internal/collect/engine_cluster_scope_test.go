// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// A cluster target registers one synthetic target per workload namespace.
// A Node has no namespace. It must still be stored once when none of those
// namespaces is "default", and it must not be copied onto every namespace.
func TestProcessDispatch_clusterScopedObjectWithoutDefaultNamespace(t *testing.T) {
	t.Parallel()

	store := NewStore()
	engine, err := NewEngine(nil, nil, store, EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(engine.stopFn)

	gvk := kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Node"}
	gvr := gvrFromProfile(gvk)
	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{TargetGVK: gvk},
	}

	keys := make([]string, 0, 2)
	for _, ns := range []string{"team-b", "team-a"} {
		target := kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "nodes", Namespace: ns},
			Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "nodes"},
		}
		key := targetKey(ns, target.Name)
		keys = append(keys, key)
		engine.targets[key] = targetState{
			target:              target,
			profile:             profile,
			effectiveNamespaces: map[string]struct{}{ns: {}},
			scopeEnforced:       true,
		}
	}
	engine.targetsByGVR[gvr] = keys

	engine.processDispatch(context.Background(), gvr, unstructuredNode(), false)

	owner := store.SnapshotTarget("team-a", "nodes")
	if len(owner) != 1 || owner[0].Name != "node-1" || owner[0].Namespace != "" {
		t.Fatalf("owner items = %+v, want one cluster-scoped node-1", owner)
	}
	if got := store.CountForTarget("team-b", "nodes"); got != 0 {
		t.Fatalf("non-owner count = %d, want 0", got)
	}
}

func unstructuredNode() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("Node")
	u.SetName("node-1")
	u.SetUID(types.UID("uid-node-1"))

	return u
}
