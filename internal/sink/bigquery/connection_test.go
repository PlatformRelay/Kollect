// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestTestConnection_configValidationBeforeDial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    kollectdevv1alpha1.KollectSinkSpec
		secret  map[string][]byte
		wantSub string
	}{
		{
			name:    "wrong sink type",
			spec:    kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"},
			wantSub: "expected bigquery sink",
		},
		{
			name: "missing dataset",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: TypeName,
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					Project: "demo-project",
					Table:   "items",
				},
			},
			wantSub: "dataset",
		},
		{
			name: "invalid credentials json",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: TypeName,
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					Project: "demo-project",
					Dataset: "inventory",
					Table:   "items",
					SecretRef: &kollectdevv1alpha1.SecretReference{
						Name: "bq-creds",
					},
				},
			},
			secret: map[string][]byte{
				CredentialsJSONKey: []byte("{not-json}"),
			},
			wantSub: "credentials.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := TestConnection(t.Context(), tt.spec, tt.secret)
			if err == nil {
				t.Fatal("TestConnection() error = nil, want config validation error")
			}
			if tt.wantSub != "" && !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("TestConnection() error = %v, want substring %q", err, tt.wantSub)
			}
		})
	}
}

func TestTestConnection_unreachableEndpoint(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := TestConnection(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		BigQuery: &kollectdevv1alpha1.BigQuerySpec{
			Project: "demo-project",
			Dataset: "inventory",
			Table:   "items",
		},
	}, nil)
	if err == nil {
		t.Fatal("TestConnection() error = nil, want connect or metadata failure")
	}
}
