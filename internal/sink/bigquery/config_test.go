// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConfigFromSpec_requiresBlock(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: TypeName}, nil)
	if err == nil {
		t.Fatal("expected error without bigquery block")
	}
}

func TestConfigFromSpec_resolvesFields(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		Cluster: "prod-a",
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "fleet-analytics",
			Dataset: "inventory",
			Table:   "items",
		},
	}, nil)
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}

	if cfg.Project != "fleet-analytics" || cfg.Dataset != "inventory" || cfg.Table != "items" {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
	if cfg.Cluster != "prod-a" {
		t.Fatalf("cluster = %q, want prod-a", cfg.Cluster)
	}
}

func TestConfigFromSpec_credentialsOptional(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "fleet-analytics",
			Dataset: "inventory",
			Table:   "items",
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CredentialsJSON) != 0 {
		t.Fatalf("expected no credentials json, got %q", cfg.CredentialsJSON)
	}
}

func TestConfigFromSpec_credentialsRequiredWhenSecretRefSet(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project:   "fleet-analytics",
			Dataset:   "inventory",
			Table:     "items",
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "bq-creds"},
		},
	}, map[string][]byte{})
	if err == nil {
		t.Fatal("expected credentials.json error")
	}
}

func TestConfigFromSpec_validationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec kollectdevv1alpha1.KollectSinkSpec
	}{
		{
			name: "wrong type",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     "postgres",
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{Project: "p", Dataset: "d", Table: "t"},
			},
		},
		{
			name: "missing project",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     TypeName,
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{Dataset: "d", Table: "t"},
			},
		},
		{
			name: "missing dataset",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     TypeName,
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{Project: "p", Table: "t"},
			},
		},
		{
			name: "missing table",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     TypeName,
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{Project: "p", Dataset: "d"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ConfigFromSpec(tt.spec, nil); err == nil {
				t.Fatalf("ConfigFromSpec(%s) = nil error, want validation error", tt.name)
			}
		})
	}
}

func TestConfigFromSpec_credentialsResolvedFromSecret(t *testing.T) {
	t.Parallel()

	creds := []byte(`{"type":"service_account"}`)
	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project:   "fleet-analytics",
			Dataset:   "inventory",
			Table:     "items",
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "bq-creds"},
		},
	}, map[string][]byte{CredentialsJSONKey: creds})
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}
	if string(cfg.CredentialsJSON) != string(creds) {
		t.Fatalf("CredentialsJSON = %q, want %q", cfg.CredentialsJSON, creds)
	}
}

func TestConfigFromSpec_secretMissingCredentialsKey(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project:   "fleet-analytics",
			Dataset:   "inventory",
			Table:     "items",
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "bq-creds"},
		},
	}, map[string][]byte{"wrong-key": []byte("x")})
	if err == nil {
		t.Fatal("expected error when secret lacks credentials.json key")
	}
}
