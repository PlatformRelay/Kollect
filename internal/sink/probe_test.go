// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestRunConnectionTest_unsupportedType(t *testing.T) {
	t.Parallel()

	_, err := RunConnectionTest(t.Context(), kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"}, BuildContext{})
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestRunConnectionTest_configErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec kollectdevv1alpha1.KollectSinkSpec
	}{
		{
			name: "git missing endpoint",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "git"},
		},
		{
			name: "gitlab missing endpoint",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "gitlab"},
		},
		{
			name: "postgres missing databaseRef",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     kollectdevv1alpha1.SinkTypePostgres,
				Postgres: &kollectdevv1alpha1.PostgresSpec{Table: "items"},
			},
		},
		{
			name: "bigquery missing project",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: kollectdevv1alpha1.SinkTypeBigQuery,
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					Dataset: "inventory",
					Table:   "items",
				},
			},
		},
		{
			name: "kafka missing brokers",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:  "kafka",
				Kafka: &kollectdevv1alpha1.KafkaSpec{Topic: "inventory"},
			},
		},
		{
			name: "nats missing url",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "nats",
				Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.events"},
			},
		},
		{
			name: "s3 missing bucket",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "s3"},
		},
		{
			name: "gcs missing bucket",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "gcs"},
		},
		{
			// mongodb.TestConnection would dial with a 15s timeout if config passed;
			// an absent spec.mongodb fails ConfigFromSpec before any network probe.
			name: "mongo missing spec",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.DatabaseSinkTypeMongoDB},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := RunConnectionTest(t.Context(), tt.spec, BuildContext{})
			if err == nil {
				t.Fatal("expected configuration error before network probe")
			}
		})
	}
}

func TestRunConnectionTest_gitInvalidTLS(t *testing.T) {
	t.Parallel()

	_, err := RunConnectionTest(t.Context(), kollectdevv1alpha1.KollectSinkSpec{
		Type:     "git",
		Endpoint: "https://example.com/inventory.git",
		TLS: &kollectdevv1alpha1.TLSSpec{
			CABundle:    []byte("not-pem"),
			CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"},
		},
	}, BuildContext{})
	if err == nil {
		t.Fatal("expected ambiguous TLS config error")
	}
}
