// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectSnapshotSinkValidator_invalidSpec(t *testing.T) {
	t.Parallel()

	v := &kollectSnapshotSinkValidator{client: fake.NewClientBuilder().Build()}
	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit},
	})
	if err == nil {
		t.Fatal("expected git block required")
	}
}

func TestKollectSnapshotSinkValidator_validSpec(t *testing.T) {
	t.Parallel()

	v := &kollectSnapshotSinkValidator{client: fake.NewClientBuilder().Build()}
	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			Git:  &kollectdevv1alpha1.GitSpec{},
		},
	})
	if err != nil {
		t.Fatalf("valid snapshot sink: %v", err)
	}
}

func TestKollectDatabaseSinkValidator_invalidSpec(t *testing.T) {
	t.Parallel()

	v := &kollectDatabaseSinkValidator{client: fake.NewClientBuilder().Build()}
	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectDatabaseSinkSpec{Type: kollectdevv1alpha1.DatabaseSinkTypePostgres},
	})
	if err == nil {
		t.Fatal("expected postgres block required")
	}
}

func TestKollectEventSinkValidator_validSpec(t *testing.T) {
	t.Parallel()

	v := &kollectEventSinkValidator{client: fake.NewClientBuilder().Build()}
	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "nats", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectEventSinkSpec{
			Type: kollectdevv1alpha1.EventSinkTypeNats,
			Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.>"},
		},
	})
	if err != nil {
		t.Fatalf("valid event sink: %v", err)
	}
}

func TestKollectClusterSnapshotSinkValidator_validate(t *testing.T) {
	t.Parallel()

	v := &kollectClusterSnapshotSinkValidator{}
	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit},
	})
	if err == nil {
		t.Fatal("expected git block required")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:        kollectdevv1alpha1.SnapshotSinkTypeS3,
			ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{Format: "json"},
		},
	})
	if err != nil {
		t.Fatalf("valid cluster snapshot: %v", err)
	}
}

func TestFamilySinkValidators_updateDelete(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	snap := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			Git:  &kollectdevv1alpha1.GitSpec{},
		},
	}
	v := &kollectSnapshotSinkValidator{client: fake.NewClientBuilder().Build()}
	deleting := snap.DeepCopy()
	deleting.DeletionTimestamp = &now
	if _, err := v.ValidateUpdate(context.Background(), snap, deleting); err != nil {
		t.Fatalf("deletion update: %v", err)
	}
	if _, err := v.ValidateDelete(context.Background(), snap); err != nil {
		t.Fatalf("delete: %v", err)
	}

	cluster := &kollectClusterDatabaseSinkValidator{}
	db := &kollectdevv1alpha1.KollectClusterDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory",
			},
		},
	}
	if _, err := cluster.ValidateDelete(context.Background(), db); err != nil {
		t.Fatalf("cluster delete: %v", err)
	}
}
