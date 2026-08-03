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

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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

// TestNamespacedSinkValidators_updateDuringDeletion covers the deletion-timestamp
// short-circuit branch of the database and event ValidateUpdate methods: an update to an
// object already being deleted skips validation and is admitted (COV-90-S05).
func TestNamespacedSinkValidators_updateDuringDeletion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := newFakeSinkClient(t)
	now := metav1.Now()

	db := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Spec:       validDatabaseSpec(),
	}
	deletingDB := db.DeepCopy()
	deletingDB.DeletionTimestamp = &now
	dbv := &kollectDatabaseSinkValidator{client: c}
	if _, err := dbv.ValidateUpdate(ctx, db, deletingDB); err != nil {
		t.Fatalf("database deletion update: %v", err)
	}

	ev := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "nats", Namespace: "default"},
		Spec:       validEventSpec(),
	}
	deletingEV := ev.DeepCopy()
	deletingEV.DeletionTimestamp = &now
	evv := &kollectEventSinkValidator{client: c}
	if _, err := evv.ValidateUpdate(ctx, ev, deletingEV); err != nil {
		t.Fatalf("event deletion update: %v", err)
	}
}
