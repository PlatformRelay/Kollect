// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func newScopedFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	sch := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objs...).Build()
}

// TestClusterInventoryValidator_validateClusterScope_sinkRefDenied asserts the
// cluster-inventory validator rejects a family sink ref outside the cluster-scope
// allowlist and names the offending ref (L0 supplement to the envtest admission spec).
func TestClusterInventoryValidator_validateClusterScope_sinkRefDenied(t *testing.T) {
	t.Parallel()

	clusterScope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			DatabaseSinkRefs:           []string{"warehouse"},
			AllowedStaticRefNamespaces: []string{"kollect-system"},
		},
	}
	v := &kollectClusterInventoryValidator{client: newScopedFakeClient(t, clusterScope)}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace: "kollect-system",
			DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "shadow-db", Namespace: "kollect-system"},
			},
		},
	}

	err := v.validateClusterScope(context.Background(), inv)
	if err == nil {
		t.Fatal("expected cluster-scope sink-ref violation for shadow-db")
	}
	if !strings.Contains(err.Error(), "shadow-db") {
		t.Fatalf("error must name offending ref shadow-db, got: %v", err)
	}
}

// TestClusterInventoryValidator_validateClusterScope_staticRefNamespaceDenied asserts a
// sink ref namespace outside allowedStaticRefNamespaces is rejected and named.
func TestClusterInventoryValidator_validateClusterScope_staticRefNamespaceDenied(t *testing.T) {
	t.Parallel()

	clusterScope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			AllowedStaticRefNamespaces: []string{"kollect-system"},
		},
	}
	v := &kollectClusterInventoryValidator{client: newScopedFakeClient(t, clusterScope)}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace: "kollect-system",
			SnapshotSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "git", Namespace: "forbidden-ns"},
			},
		},
	}

	err := v.validateClusterScope(context.Background(), inv)
	if err == nil {
		t.Fatal("expected static ref namespace violation for forbidden-ns")
	}
	if !strings.Contains(err.Error(), "forbidden-ns") {
		t.Fatalf("error must name offending namespace forbidden-ns, got: %v", err)
	}
}

// TestClusterInventoryValidator_validateClusterScope_allowed asserts the happy path:
// refs within the allowlist and allowed namespaces pass.
func TestClusterInventoryValidator_validateClusterScope_allowed(t *testing.T) {
	t.Parallel()

	clusterScope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			DatabaseSinkRefs:           []string{"warehouse"},
			AllowedStaticRefNamespaces: []string{"kollect-system"},
		},
	}
	v := &kollectClusterInventoryValidator{client: newScopedFakeClient(t, clusterScope)}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			SinkNamespace: "kollect-system",
			DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "warehouse", Namespace: "kollect-system"},
			},
		},
	}

	if err := v.validateClusterScope(context.Background(), inv); err != nil {
		t.Fatalf("expected allowed cluster inventory: %v", err)
	}
}

// TestClusterInventoryValidator_validateClusterScope_noEnforcement asserts that when no
// cluster scope exists, validation is a no-op regardless of refs.
func TestClusterInventoryValidator_validateClusterScope_noEnforcement(t *testing.T) {
	t.Parallel()

	v := &kollectClusterInventoryValidator{client: newScopedFakeClient(t)}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "anything", Namespace: "anywhere"},
			},
		},
	}

	if err := v.validateClusterScope(context.Background(), inv); err != nil {
		t.Fatalf("no cluster scope should allow any refs: %v", err)
	}
}

// TestNamespacedSinkScopeFloor_intervalBelowFloorDenied drives the enforced-scope branch
// of validateNamespacedSinkScopeFloor: a snapshot sink whose exportMinInterval is below
// the KollectScope floor is rejected, and the rejection names the offending sink.
func TestNamespacedSinkScopeFloor_intervalBelowFloorDenied(t *testing.T) {
	t.Parallel()

	teamScope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				MinExportInterval: &metav1.Duration{Duration: time.Hour},
			},
		},
	}
	v := &kollectSnapshotSinkValidator{client: newScopedFakeClient(t, teamScope)}

	sink := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "fast-git", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			Git:  &kollectdevv1alpha1.GitSpec{},
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				ExportMinInterval: &metav1.Duration{Duration: time.Second},
			},
		},
	}

	_, err := v.ValidateCreate(context.Background(), sink)
	if err == nil {
		t.Fatal("expected scope-floor violation for fast-git")
	}
	if !strings.Contains(err.Error(), "fast-git") {
		t.Fatalf("error must name offending sink fast-git, got: %v", err)
	}
}
