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
