// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"sync"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// secretRefNamespaceTestMu serializes tests that mutate the process-global K-04
// allowlist. Production stays on an atomic pointer for concurrent admission
// reads; tests that flip it must not overlap set→assert→restore windows, and
// must NOT run under t.Parallel.
var secretRefNamespaceTestMu sync.Mutex

// withAllowedSecretRefNamespaces installs namespaces as the admission allowlist
// for the duration of t, restoring the deny default on cleanup. Do not call
// t.Parallel in tests that use this helper.
func withAllowedSecretRefNamespaces(t *testing.T, namespaces []string) {
	t.Helper()

	secretRefNamespaceTestMu.Lock()
	SetAllowedSecretRefNamespaces(namespaces)
	t.Cleanup(func() {
		SetAllowedSecretRefNamespaces(nil)
		secretRefNamespaceTestMu.Unlock()
	})
}

func TestValidateSecretRefNamespaces_SameAndEmptyAllowed(t *testing.T) {
	withAllowedSecretRefNamespaces(t, nil)

	spec := &kollectdevv1alpha1.KollectSinkSpec{
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds", Namespace: "team-a"},
		TLS: &kollectdevv1alpha1.TLSSpec{
			CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca", Namespace: "team-a"},
		},
		Git: &kollectdevv1alpha1.GitSpec{
			Auth: &kollectdevv1alpha1.GitAuthSpec{
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "git", Namespace: "team-a"},
			},
		},
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg", Namespace: "team-a"},
		},
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
		},
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "bq", Namespace: "team-a"},
		},
		Nats: &kollectdevv1alpha1.NatsSpec{
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "nats", Namespace: "team-a"},
		},
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "kafka", Namespace: "team-a"},
		},
	}

	if errs := ValidateSecretRefNamespaces(spec, "team-a"); len(errs) != 0 {
		t.Fatalf("same-namespace and empty refs must be allowed, got %v", errs)
	}
}

func TestValidateSecretRefNamespaces_CrossNamespaceRejectedOnEveryPath(t *testing.T) {
	withAllowedSecretRefNamespaces(t, nil)

	cases := []struct {
		name string
		spec *kollectdevv1alpha1.KollectSinkSpec
		path string
	}{
		{
			name: "spec.secretRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
			},
			path: "spec.secretRef.namespace",
		},
		{
			name: "spec.tls.caSecretRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				TLS: &kollectdevv1alpha1.TLSSpec{
					CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
				},
			},
			path: "spec.tls.caSecretRef.namespace",
		},
		{
			name: "spec.git.auth.secretRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				Git: &kollectdevv1alpha1.GitSpec{
					Auth: &kollectdevv1alpha1.GitAuthSpec{
						SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
					},
				},
			},
			path: "spec.git.auth.secretRef.namespace",
		},
		{
			name: "spec.postgres.databaseRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				Postgres: &kollectdevv1alpha1.PostgresSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
				},
			},
			path: "spec.postgres.databaseRef.namespace",
		},
		{
			name: "spec.mongodb.databaseRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				MongoDB: &kollectdevv1alpha1.MongoSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
				},
			},
			path: "spec.mongodb.databaseRef.namespace",
		},
		{
			name: "spec.bigquery.secretRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
				},
			},
			path: "spec.bigquery.secretRef.namespace",
		},
		{
			name: "spec.nats.secretRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				Nats: &kollectdevv1alpha1.NatsSpec{
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
				},
			},
			path: "spec.nats.secretRef.namespace",
		},
		{
			name: "spec.kafka.secretRef",
			spec: &kollectdevv1alpha1.KollectSinkSpec{
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "victim"},
				},
			},
			path: "spec.kafka.secretRef.namespace",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateSecretRefNamespaces(tc.spec, "team-a")
			if len(errs) != 1 {
				t.Fatalf("want exactly one error, got %v", errs)
			}
			if got := errs[0].Field; got != tc.path {
				t.Fatalf("field = %q, want %q", got, tc.path)
			}
			if errs[0].Type != "FieldValueForbidden" {
				t.Fatalf("error type = %q, want FieldValueForbidden", errs[0].Type)
			}
		})
	}
}

func TestValidateSecretRefNamespaces_AllowlistPermits(t *testing.T) {
	withAllowedSecretRefNamespaces(t, []string{"shared-creds", "  padded  "})

	spec := &kollectdevv1alpha1.KollectSinkSpec{
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "shared-creds"},
	}
	if errs := ValidateSecretRefNamespaces(spec, "team-a"); len(errs) != 0 {
		t.Fatalf("allowlisted namespace must be permitted, got %v", errs)
	}

	if !SecretRefNamespaceAllowed("padded") {
		t.Fatal("allowlist entries must be whitespace-trimmed")
	}

	other := &kollectdevv1alpha1.KollectSinkSpec{
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "x", Namespace: "other"},
	}
	if errs := ValidateSecretRefNamespaces(other, "team-a"); len(errs) != 1 {
		t.Fatalf("non-allowlisted namespace must be rejected, got %v", errs)
	}
}

func TestValidateSecretRefNamespaces_NilSpec(t *testing.T) {
	withAllowedSecretRefNamespaces(t, nil)

	if errs := ValidateSecretRefNamespaces(nil, "team-a"); errs != nil {
		t.Fatalf("nil spec must produce no errors, got %v", errs)
	}
}
