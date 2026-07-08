// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import "testing"

func TestToKollectSinkSpec_snapshot(t *testing.T) {
	t.Parallel()

	spec := (&KollectSnapshotSinkSpec{
		Type: SnapshotSinkTypeS3,
		SinkCommonFields: SinkCommonFields{
			Endpoint: "s3://bucket",
		},
		Git:         &GitSpec{},
		ObjectStore: &ObjectStoreSpec{Format: "json"},
	}).ToKollectSinkSpec()

	if spec.Type != SnapshotSinkTypeS3 || spec.Endpoint != "s3://bucket" {
		t.Fatalf("spec = %#v", spec)
	}
	if spec.ObjectStore == nil || spec.Git == nil {
		t.Fatalf("expected blocks copied: %#v", spec)
	}
}

func TestToKollectSinkSpec_database(t *testing.T) {
	t.Parallel()

	spec := (&KollectDatabaseSinkSpec{
		Type: DatabaseSinkTypePostgres,
		Postgres: &PostgresSpec{
			DatabaseRef: &SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}).ToKollectSinkSpec()

	if spec.Type != DatabaseSinkTypePostgres || spec.Postgres == nil {
		t.Fatalf("spec = %#v", spec)
	}
}

func TestToKollectSinkSpec_databaseMongo(t *testing.T) {
	t.Parallel()

	spec := (&KollectDatabaseSinkSpec{
		Type: DatabaseSinkTypeMongoDB,
		MongoDB: &MongoSpec{
			DatabaseRef: &SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}).ToKollectSinkSpec()

	if spec.Type != DatabaseSinkTypeMongoDB || spec.MongoDB == nil {
		t.Fatalf("spec = %#v", spec)
	}
	if spec.MongoDB.Collection != "items" {
		t.Fatalf("mongo block not copied: %#v", spec.MongoDB)
	}
}

func TestToKollectSinkSpec_databaseBigQuery(t *testing.T) {
	t.Parallel()

	spec := (&KollectDatabaseSinkSpec{
		Type: DatabaseSinkTypeBigQuery,
		BigQuery: &BigQuerySpec{
			Project: "fleet-analytics",
			Dataset: "inventory",
			Table:   "items",
		},
	}).ToKollectSinkSpec()

	if spec.Type != DatabaseSinkTypeBigQuery || spec.BigQuery == nil {
		t.Fatalf("spec = %#v", spec)
	}
	if spec.BigQuery.Dataset != "inventory" {
		t.Fatalf("bigquery block not copied: %#v", spec.BigQuery)
	}
}

func TestToKollectSinkSpec_event(t *testing.T) {
	t.Parallel()

	spec := (&KollectEventSinkSpec{
		Type:  EventSinkTypeKafka,
		Kafka: &KafkaSpec{Brokers: []string{"kafka:9092"}, Topic: "inventory"},
	}).ToKollectSinkSpec()

	if spec.Type != EventSinkTypeKafka || spec.Kafka == nil {
		t.Fatalf("spec = %#v", spec)
	}
}

func TestToKollectSinkSpec_nilReceiver(t *testing.T) {
	t.Parallel()

	var snap *KollectSnapshotSinkSpec
	if got := snap.ToKollectSinkSpec(); got.Type != "" {
		t.Fatalf("nil snapshot = %#v", got)
	}
}

// TestFamilySinkObject_accessors verifies the FamilySinkObject wiring (AR-08)
// that internal/controller's generic FamilySinkReconciler relies on: each of
// the three family-sink kinds must return its own spec/common/status, not a
// shared or zero value.
func TestFamilySinkObject_accessors(t *testing.T) {
	t.Parallel()

	t.Run("snapshot", func(t *testing.T) {
		t.Parallel()

		obj := &KollectSnapshotSink{
			Spec: KollectSnapshotSinkSpec{
				Type:             SnapshotSinkTypeS3,
				SinkCommonFields: SinkCommonFields{Endpoint: "s3://bucket"},
			},
			Status: FamilySinkStatus{Preview: &SinkPreviewStatus{}},
		}

		var fs FamilySinkObject = obj
		if got := fs.FamilySinkSpec(); got.Type != SnapshotSinkTypeS3 || got.Endpoint != "s3://bucket" {
			t.Fatalf("FamilySinkSpec = %#v", got)
		}
		if got := fs.FamilySinkCommon(); got != &obj.Spec.SinkCommonFields {
			t.Fatalf("FamilySinkCommon = %p, want %p", got, &obj.Spec.SinkCommonFields)
		}
		if got := fs.FamilySinkStatus(); got != &obj.Status {
			t.Fatalf("FamilySinkStatus = %p, want %p", got, &obj.Status)
		}
	})

	t.Run("database", func(t *testing.T) {
		t.Parallel()

		obj := &KollectDatabaseSink{
			Spec: KollectDatabaseSinkSpec{
				Type:             DatabaseSinkTypePostgres,
				SinkCommonFields: SinkCommonFields{Endpoint: "postgres://db"},
			},
			Status: FamilySinkStatus{Preview: &SinkPreviewStatus{}},
		}

		var fs FamilySinkObject = obj
		if got := fs.FamilySinkSpec(); got.Type != DatabaseSinkTypePostgres || got.Endpoint != "postgres://db" {
			t.Fatalf("FamilySinkSpec = %#v", got)
		}
		if got := fs.FamilySinkCommon(); got != &obj.Spec.SinkCommonFields {
			t.Fatalf("FamilySinkCommon = %p, want %p", got, &obj.Spec.SinkCommonFields)
		}
		if got := fs.FamilySinkStatus(); got != &obj.Status {
			t.Fatalf("FamilySinkStatus = %p, want %p", got, &obj.Status)
		}
	})

	t.Run("event", func(t *testing.T) {
		t.Parallel()

		obj := &KollectEventSink{
			Spec: KollectEventSinkSpec{
				Type:             EventSinkTypeNats,
				SinkCommonFields: SinkCommonFields{Endpoint: "nats://events"},
			},
			Status: FamilySinkStatus{Preview: &SinkPreviewStatus{}},
		}

		var fs FamilySinkObject = obj
		if got := fs.FamilySinkSpec(); got.Type != EventSinkTypeNats || got.Endpoint != "nats://events" {
			t.Fatalf("FamilySinkSpec = %#v", got)
		}
		if got := fs.FamilySinkCommon(); got != &obj.Spec.SinkCommonFields {
			t.Fatalf("FamilySinkCommon = %p, want %p", got, &obj.Spec.SinkCommonFields)
		}
		if got := fs.FamilySinkStatus(); got != &obj.Status {
			t.Fatalf("FamilySinkStatus = %p, want %p", got, &obj.Status)
		}
	})
}
