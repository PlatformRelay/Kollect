// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func TestFamilySinkConditions_AllFamiliesAndScopes(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cond := metav1.Condition{Type: kollectdevv1alpha1.ConditionConnectionVerified, Status: metav1.ConditionTrue}
	snapshot := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "snapshot-ns", Namespace: "team-a"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{cond}},
	}
	clusterSnapshot := &kollectdevv1alpha1.KollectClusterSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "snapshot-cluster"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{cond}},
	}
	database := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "database-ns", Namespace: "team-a"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{cond}},
	}
	clusterDatabase := &kollectdevv1alpha1.KollectClusterDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "database-cluster"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{cond}},
	}
	event := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "event-ns", Namespace: "team-a"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{cond}},
	}
	clusterEvent := &kollectdevv1alpha1.KollectClusterEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "event-cluster"},
		Status:     kollectdevv1alpha1.FamilySinkStatus{Conditions: []metav1.Condition{cond}},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(snapshot, clusterSnapshot, database, clusterDatabase, event, clusterEvent).
		Build()

	cases := []sink.ResolvedSink{
		{Family: kollectdevv1alpha1.SinkFamilySnapshot, Namespace: "team-a", Name: "snapshot-ns"},
		{Family: kollectdevv1alpha1.SinkFamilySnapshot, ClusterScoped: true, Name: "snapshot-cluster"},
		{Family: kollectdevv1alpha1.SinkFamilyDatabase, Namespace: "team-a", Name: "database-ns"},
		{Family: kollectdevv1alpha1.SinkFamilyDatabase, ClusterScoped: true, Name: "database-cluster"},
		{Family: kollectdevv1alpha1.SinkFamilyEvent, Namespace: "team-a", Name: "event-ns"},
		{Family: kollectdevv1alpha1.SinkFamilyEvent, ClusterScoped: true, Name: "event-cluster"},
	}
	for _, tc := range cases {
		t.Run(tc.Family+"/"+tc.Name, func(t *testing.T) {
			t.Parallel()

			conds, err := familySinkConditions(context.Background(), cl, &tc)
			if err != nil {
				t.Fatalf("familySinkConditions() error = %v", err)
			}
			if len(conds) != 1 || conds[0].Type != kollectdevv1alpha1.ConditionConnectionVerified {
				t.Fatalf("familySinkConditions() = %#v", conds)
			}
		})
	}
}

func TestFamilySinkConditions_UnknownFamily(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := familySinkConditions(context.Background(), cl, &sink.ResolvedSink{
		Family: "unsupported",
		Name:   "sink-a",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown sink family") {
		t.Fatalf("familySinkConditions() error = %v, want unknown family", err)
	}
}
