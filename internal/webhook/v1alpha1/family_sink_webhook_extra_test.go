// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func newFakeSinkClient(t *testing.T) client.Client {
	t.Helper()
	sch := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(sch).Build()
}

func validDatabaseSpec() kollectdevv1alpha1.KollectDatabaseSinkSpec {
	return kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}
}

func validEventSpec() kollectdevv1alpha1.KollectEventSinkSpec {
	return kollectdevv1alpha1.KollectEventSinkSpec{
		Type: kollectdevv1alpha1.EventSinkTypeNats,
		Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.>"},
	}
}

func validSnapshotSpec() kollectdevv1alpha1.KollectSnapshotSinkSpec {
	return kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type:        kollectdevv1alpha1.SnapshotSinkTypeS3,
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{Format: "json"},
	}
}

// TestNamespacedSinkValidators_updateCreate exercises ValidateCreate/Update on the
// namespaced database and event validators (the non-deletion update path is otherwise
// uncovered) against a fake client with no enforced scope.
func TestNamespacedSinkValidators_updateCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := newFakeSinkClient(t)

	db := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Spec:       validDatabaseSpec(),
	}
	dbv := &kollectDatabaseSinkValidator{client: c}
	if _, err := dbv.ValidateCreate(ctx, db); err != nil {
		t.Fatalf("database create: %v", err)
	}
	if _, err := dbv.ValidateUpdate(ctx, db, db); err != nil {
		t.Fatalf("database update: %v", err)
	}
	if _, err := dbv.ValidateDelete(ctx, db); err != nil {
		t.Fatalf("database delete: %v", err)
	}

	ev := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "nats", Namespace: "default"},
		Spec:       validEventSpec(),
	}
	evv := &kollectEventSinkValidator{client: c}
	if _, err := evv.ValidateUpdate(ctx, ev, ev); err != nil {
		t.Fatalf("event update: %v", err)
	}
	if _, err := evv.ValidateDelete(ctx, ev); err != nil {
		t.Fatalf("event delete: %v", err)
	}
}

// TestClusterSinkValidators_allVerbs exercises every verb on the cluster-scoped
// validators, whose validate paths were uncovered.
func TestClusterSinkValidators_allVerbs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	snap := &kollectdevv1alpha1.KollectClusterSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3"},
		Spec:       validSnapshotSpec(),
	}
	snapv := &kollectClusterSnapshotSinkValidator{}
	if _, err := snapv.ValidateUpdate(ctx, snap, snap); err != nil {
		t.Fatalf("cluster snapshot update: %v", err)
	}

	db := &kollectdevv1alpha1.KollectClusterDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg"},
		Spec:       validDatabaseSpec(),
	}
	dbv := &kollectClusterDatabaseSinkValidator{}
	if _, err := dbv.ValidateCreate(ctx, db); err != nil {
		t.Fatalf("cluster database create: %v", err)
	}
	if _, err := dbv.ValidateUpdate(ctx, db, db); err != nil {
		t.Fatalf("cluster database update: %v", err)
	}

	ev := &kollectdevv1alpha1.KollectClusterEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "nats"},
		Spec:       validEventSpec(),
	}
	evv := &kollectClusterEventSinkValidator{}
	if _, err := evv.ValidateCreate(ctx, ev); err != nil {
		t.Fatalf("cluster event create: %v", err)
	}
	if _, err := evv.ValidateUpdate(ctx, ev, ev); err != nil {
		t.Fatalf("cluster event update: %v", err)
	}
	if _, err := evv.ValidateDelete(ctx, ev); err != nil {
		t.Fatalf("cluster event delete: %v", err)
	}

	// Invalid cluster event spec surfaces the validate error branch.
	bad := &kollectdevv1alpha1.KollectClusterEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec:       kollectdevv1alpha1.KollectEventSinkSpec{Type: kollectdevv1alpha1.EventSinkTypeNats},
	}
	if _, err := evv.ValidateCreate(ctx, bad); err == nil {
		t.Fatal("expected nats block required")
	}
}

// TestClusterDatabaseSink_deletionUpdateSkips confirms the deletion short-circuit on
// cluster validators returns no error even for an otherwise-invalid spec.
func TestClusterDatabaseSink_deletionUpdateSkips(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	deleting := &kollectdevv1alpha1.KollectClusterDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", DeletionTimestamp: &now},
		Spec:       kollectdevv1alpha1.KollectDatabaseSinkSpec{Type: kollectdevv1alpha1.DatabaseSinkTypePostgres},
	}
	v := &kollectClusterDatabaseSinkValidator{}
	if _, err := v.ValidateUpdate(context.Background(), deleting, deleting); err != nil {
		t.Fatalf("deletion update should skip validation: %v", err)
	}
}
