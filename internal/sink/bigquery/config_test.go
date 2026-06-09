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
