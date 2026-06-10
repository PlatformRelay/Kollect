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

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

func TestCheckInventorySinksReachable_connectionFailed(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sink := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-sink", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectDatabaseSinkSpec{Type: kollectdevv1alpha1.DatabaseSinkTypePostgres},
		Status: kollectdevv1alpha1.FamilySinkStatus{
			Conditions: []metav1.Condition{{
				Type:    kollectdevv1alpha1.ConditionConnectionVerified,
				Status:  metav1.ConditionFalse,
				Message: "TLS handshake failed",
			}},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sink).WithStatusSubresource(sink).Build()

	ok, reason, _ := checkInventorySinksReachable(context.Background(), cl, "default", []kollectdevv1alpha1.InventorySinkBinding{{Name: "bad-sink", Family: kollectdevv1alpha1.SinkFamilyDatabase}})
	if ok {
		t.Fatal("expected sinks unreachable")
	}
	if reason != reasonSinkUnreachable {
		t.Fatalf("reason = %q, want %s", reason, reasonSinkUnreachable)
	}
}

func TestCheckInventorySinksReachable_forbidden(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollectdatabasesinks"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return apierrors.NewForbidden(gr, "warehouse", errors.New("RBAC denied"))
		},
	}).Build()

	ok, reason, _ := checkInventorySinksReachable(
		context.Background(), cl, "kollect-system",
		[]kollectdevv1alpha1.InventorySinkBinding{{Name: "warehouse", Family: kollectdevv1alpha1.SinkFamilyDatabase}},
	)
	if ok {
		t.Fatal("expected sinks unreachable on forbidden")
	}
	if reason != reasonSinkForbidden {
		t.Fatalf("reason = %q, want %s", reason, reasonSinkForbidden)
	}
}

func TestCheckTargetNamespaceSinksReachable_noInventory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	ok, reason, _ := checkTargetNamespaceSinksReachable(context.Background(), cl, "team-a")
	if !ok {
		t.Fatal("expected reachable when no inventory sink refs")
	}
	if reason != "NoSinksInNamespace" {
		t.Fatalf("reason = %q, want NoSinksInNamespace", reason)
	}
}

func TestSetSinkReachableFromExport(t *testing.T) {
	t.Parallel()

	var conds []metav1.Condition
	setSinkReachableFromExport(&conds, 3, nil)

	c := apimeta.FindStatusCondition(conds, kollectdevv1alpha1.ConditionSinkReachable)
	if c == nil || c.Status != metav1.ConditionTrue || c.Reason != "ExportSucceeded" {
		t.Fatalf("success condition: %+v", c)
	}

	setSinkReachableFromExport(&conds, 3, kollecterrors.Terminal(context.DeadlineExceeded))
	c = apimeta.FindStatusCondition(conds, kollectdevv1alpha1.ConditionSinkReachable)
	if c == nil || c.Status != metav1.ConditionFalse || c.Reason != kollectdevv1alpha1.ReasonExportTerminal {
		t.Fatalf("terminal export condition: %+v", c)
	}
}

func TestUniqueInventorySinkBindings_DedupesByFamilyAndName(t *testing.T) {
	t.Parallel()

	inventories := []kollectdevv1alpha1.KollectInventory{
		{
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("snap-a"),
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("db-a"),
			},
		},
		{
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				SnapshotSinkRefs: kollectdevv1alpha1.NewSinkRefList("snap-a"),
				EventSinkRefs:    kollectdevv1alpha1.NewSinkRefList("events-a"),
			},
		},
	}

	got := uniqueInventorySinkBindings(inventories)
	if len(got) != 3 {
		t.Fatalf("uniqueInventorySinkBindings len = %d, want 3", len(got))
	}
}

func TestFamilySinkConditions_AllFamiliesAndScopes(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	snapshot := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "snap", Namespace: "default"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}},
	}
	database := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}},
	}
	event := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "events", Namespace: "default"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(snapshot, database, event).
		WithStatusSubresource(snapshot, database, event).
		Build()

	tests := []struct {
		name     string
		resolved *sink.ResolvedSink
	}{
		{name: "snapshot namespaced", resolved: &sink.ResolvedSink{Family: kollectdevv1alpha1.SinkFamilySnapshot, Namespace: "default", Name: "snap"}},
		{name: "database namespaced", resolved: &sink.ResolvedSink{Family: kollectdevv1alpha1.SinkFamilyDatabase, Namespace: "default", Name: "db"}},
		{name: "event namespaced", resolved: &sink.ResolvedSink{Family: kollectdevv1alpha1.SinkFamilyEvent, Namespace: "default", Name: "events"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			conds, err := familySinkConditions(context.Background(), cl, tc.resolved)
			if err != nil {
				t.Fatalf("familySinkConditions: %v", err)
			}
			if len(conds) == 0 {
				t.Fatal("familySinkConditions returned no conditions")
			}
		})
	}
}

func TestFamilySinkConditions_UnknownFamily(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := familySinkConditions(context.Background(), cl, &sink.ResolvedSink{Family: "custom"})
	if err == nil {
		t.Fatal("familySinkConditions returned nil error, want unknown family")
	}
}

func TestSetSinkReachableFromExport_NonTerminalErrorReason(t *testing.T) {
	t.Parallel()

	var conds []metav1.Condition
	setSinkReachableFromExport(&conds, 4, errors.New("temporary outage"))

	c := apimeta.FindStatusCondition(conds, kollectdevv1alpha1.ConditionSinkReachable)
	if c == nil {
		t.Fatal("sink reachable condition missing")
	}
	if c.Status != metav1.ConditionFalse || c.Reason != reasonExportFailed {
		t.Fatalf("condition = %+v, want false/%s", c, reasonExportFailed)
	}
}
