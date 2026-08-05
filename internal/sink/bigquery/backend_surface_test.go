// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

func TestBackend_TypeCapabilitiesClose(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	if b.Type() != TypeName {
		t.Fatalf("Type() = %q, want %q", b.Type(), TypeName)
	}
	if b.Capabilities() != cap.RelationalStore() {
		t.Fatalf("Capabilities() = %#v", b.Capabilities())
	}

	// Close with nil client must not panic.
	b.Close()
}

func TestBackend_Close_invokesClientClose(t *testing.T) {
	t.Parallel()

	client, err := bigquery.NewClient(
		t.Context(),
		"test-project",
		option.WithoutAuthentication(),
		option.WithEndpoint("http://127.0.0.1:9"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	b := &Backend{client: client}
	b.Close()
}

func TestNewBackend_configValidationBeforeDial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    kollectdevv1alpha1.KollectSinkSpec
		secret  map[string][]byte
		wantSub string
	}{
		{
			name:    "missing bigquery spec",
			spec:    kollectdevv1alpha1.KollectSinkSpec{Type: TypeName},
			wantSub: "requires spec.bigquery",
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

			_, err := NewBackend(t.Context(), tt.spec, tt.secret)
			if err == nil {
				t.Fatal("NewBackend() error = nil, want config validation error")
			}
			if tt.wantSub != "" && !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("NewBackend() error = %v, want substring %q", err, tt.wantSub)
			}
		})
	}
}
