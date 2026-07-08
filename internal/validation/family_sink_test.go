// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateSnapshotSinkSpec_gitRequiresBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
	})
	if len(errs) == 0 {
		t.Fatal("expected git block required")
	}
}

func TestValidateEventSinkSpec_rejectsParquetSerialization(t *testing.T) {
	t.Parallel()

	errs := ValidateEventSinkSpec(&kollectdevv1alpha1.KollectEventSinkSpec{
		Type:  kollectdevv1alpha1.EventSinkTypeKafka,
		Kafka: &kollectdevv1alpha1.KafkaSpec{Brokers: []string{"localhost:9092"}, Topic: "inv"},
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			Serialization: &kollectdevv1alpha1.SerializationSpec{Format: kollectdevv1alpha1.SerializationFormatParquet},
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected serialization.format=parquet rejected for kafka (ADR-0416 capability matrix)")
	}
}

func TestValidateDatabaseSinkSpec_rejectsSecretLikeOption(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "dsn"},
			Table:       "inventory_items",
		},
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			Options: map[string]string{"password": "hunter2"},
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected secret-like options key rejected (ADR-0416 guardrail)")
	}
}

func TestValidateSnapshotSinkSpec_gitForbidsSiblings(t *testing.T) {
	t.Parallel()

	errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type:   kollectdevv1alpha1.SnapshotSinkTypeGit,
		Git:    &kollectdevv1alpha1.GitSpec{},
		GitLab: &kollectdevv1alpha1.GitLabSpec{},
	})
	if len(errs) == 0 {
		t.Fatal("expected forbidden sibling block")
	}
}

func TestValidateSnapshotSinkSpec_s3AcceptsObjectStore(t *testing.T) {
	t.Parallel()

	errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type:        kollectdevv1alpha1.SnapshotSinkTypeS3,
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{Format: "json"},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateSnapshotSinkSpec_rejectsStubTypes(t *testing.T) {
	t.Parallel()

	// http and azureblob were ADR-0414 stubs without a backend; admission must reject
	// them with an actionable "supported values" message instead of admitting CRs that
	// can never export (EC-P1-04).
	for _, sinkType := range []string{
		kollectdevv1alpha1.SnapshotSinkTypeHTTP,
		kollectdevv1alpha1.SnapshotSinkTypeAzureBlob,
	} {
		errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: sinkType,
			HTTP: &kollectdevv1alpha1.HTTPSinkSpec{Method: "POST"},
		})
		if len(errs) != 1 {
			t.Fatalf("type %s: expected exactly one unsupported-type error, got %v", sinkType, errs)
		}
		if errs[0].Type != field.ErrorTypeNotSupported {
			t.Fatalf("type %s: error type = %s, want NotSupported: %v", sinkType, errs[0].Type, errs[0])
		}
		if !strings.Contains(errs[0].Error(), kollectdevv1alpha1.SnapshotSinkTypeGit) {
			t.Fatalf("type %s: error should list supported values: %v", sinkType, errs[0])
		}
	}
}

func TestValidateDatabaseSinkSpec_postgresRequiresBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
	})
	if len(errs) == 0 {
		t.Fatal("expected postgres block required")
	}
}

func TestValidateDatabaseSinkSpec_bigQueryRequiresBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeBigQuery,
	})
	if len(errs) == 0 {
		t.Fatal("expected bigquery block required")
	}
}

func TestValidateDatabaseSinkSpec_postgresForbidsBigQuery(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{Dataset: "ds"},
	})
	if len(errs) == 0 {
		t.Fatal("expected forbidden bigquery block")
	}
}

func TestValidateDatabaseSinkSpec_bigQueryAcceptsBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeBigQuery,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "fleet-analytics",
			Dataset: "inventory",
			Table:   "items",
		},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors for valid bigquery spec: %v", errs)
	}
}

func TestValidateDatabaseSinkSpec_bigQueryForbidsPostgres(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeBigQuery,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "fleet-analytics",
			Dataset: "inventory",
			Table:   "items",
		},
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected forbidden postgres block for bigquery type")
	}
}

func TestValidateDatabaseSinkSpec_mongoRequiresBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeMongoDB,
	})
	if len(errs) == 0 {
		t.Fatal("expected mongodb block required")
	}
}

func TestValidateDatabaseSinkSpec_mongoAcceptsBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeMongoDB,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors for valid mongodb spec: %v", errs)
	}
}

func TestValidateDatabaseSinkSpec_mongoForbidsPostgres(t *testing.T) {
	t.Parallel()

	errs := ValidateDatabaseSinkSpec(&kollectdevv1alpha1.KollectDatabaseSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypeMongoDB,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected forbidden postgres block for mongodb type")
	}
}

func TestValidateEventSinkSpec_kafkaRequiresBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateEventSinkSpec(&kollectdevv1alpha1.KollectEventSinkSpec{
		Type: kollectdevv1alpha1.EventSinkTypeKafka,
	})
	if len(errs) == 0 {
		t.Fatal("expected kafka block required")
	}
}

func TestValidateEventSinkSpec_natsAcceptsBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateEventSinkSpec(&kollectdevv1alpha1.KollectEventSinkSpec{
		Type: kollectdevv1alpha1.EventSinkTypeNats,
		Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.>"},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateConnectionTestSinkRef_exactlyOne(t *testing.T) {
	t.Parallel()

	errs := ValidateConnectionTestSinkRef(kollectdevv1alpha1.ConnectionTestSinkRef{})
	if len(errs) == 0 {
		t.Fatal("expected required error")
	}

	errs = ValidateConnectionTestSinkRef(kollectdevv1alpha1.ConnectionTestSinkRef{
		SnapshotSinkRef: "a",
		DatabaseSinkRef: "b",
	})
	if len(errs) == 0 {
		t.Fatal("expected mutual exclusivity error")
	}
}

func TestFamilySinkInvalidMessages(t *testing.T) {
	t.Parallel()

	errs := field.ErrorList{field.Required(field.NewPath("spec").Child("type"), "bad")}

	cases := []struct {
		name     string
		err      error
		wantKind string
		wantName string
	}{
		{"snapshot", SnapshotSinkInvalid("snap", errs), "KollectSnapshotSink", "snap"},
		{"database", DatabaseSinkInvalid("db", errs), "KollectDatabaseSink", "db"},
		{"event", EventSinkInvalid("ev", errs), "KollectEventSink", "ev"},
	}

	for _, tc := range cases {
		var fsErr *FamilySinkInvalidError
		if !errors.As(tc.err, &fsErr) {
			t.Fatalf("%s: expected *FamilySinkInvalidError, got %T: %v", tc.name, tc.err, tc.err)
		}
		if fsErr.Kind != tc.wantKind {
			t.Errorf("%s: Kind = %q, want %q", tc.name, fsErr.Kind, tc.wantKind)
		}
		if fsErr.Name != tc.wantName {
			t.Errorf("%s: Name = %q, want %q", tc.name, fsErr.Name, tc.wantName)
		}
		if !strings.Contains(tc.err.Error(), tc.wantKind) {
			t.Errorf("%s: Error() = %q, want substring %q", tc.name, tc.err.Error(), tc.wantKind)
		}
	}
}

func TestValidateSnapshotSinkSpec_rejectsUnknownType(t *testing.T) {
	t.Parallel()

	errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type: "minio",
	})
	if len(errs) == 0 {
		t.Fatal("expected unsupported type")
	}
}

func TestValidateSnapshotSinkSpec_invalidPathTemplate(t *testing.T) {
	t.Parallel()

	errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeGitLab,
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			PathTemplate: "{cluster}/{name}.json",
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected pathTemplate error")
	}
}

func TestValidateEventSinkSpec_kafkaAcceptsBlock(t *testing.T) {
	t.Parallel()

	errs := ValidateEventSinkSpec(&kollectdevv1alpha1.KollectEventSinkSpec{
		Type: kollectdevv1alpha1.EventSinkTypeKafka,
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Topic: "inventory",
		},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateConnectionTestSinkRef_databaseRef(t *testing.T) {
	t.Parallel()

	errs := ValidateConnectionTestSinkRef(kollectdevv1alpha1.ConnectionTestSinkRef{
		DatabaseSinkRef: "warehouse",
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateConnectionTestSinkRef_eventRef(t *testing.T) {
	t.Parallel()

	errs := ValidateConnectionTestSinkRef(kollectdevv1alpha1.ConnectionTestSinkRef{
		EventSinkRef: "audit",
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateConnectionTestSinkRef_invalidName(t *testing.T) {
	t.Parallel()

	errs := ValidateConnectionTestSinkRef(kollectdevv1alpha1.ConnectionTestSinkRef{
		SnapshotSinkRef: "team-a/git",
	})
	if len(errs) == 0 {
		t.Fatal("expected invalid ref name")
	}
}

func TestValidateSnapshotSinkSpec_exportMinInterval(t *testing.T) {
	t.Parallel()

	d := metav1.Duration{Duration: -1}
	errs := ValidateSnapshotSinkSpec(&kollectdevv1alpha1.KollectSnapshotSinkSpec{
		Type: kollectdevv1alpha1.SnapshotSinkTypeGitLab,
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			ExportMinInterval: &d,
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected invalid exportMinInterval")
	}
}
