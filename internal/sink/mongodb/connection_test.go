// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTestConnection_missingDatabaseRef(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{},
	}, nil)
	if err == nil {
		t.Fatal("expected error when databaseRef is missing")
	}
}

func TestTestConnection_invalidURI(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := TestConnection(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}, map[string][]byte{"uri": []byte("mongodb://127.0.0.1:1/unreachable")})
	if err == nil {
		t.Fatal("expected connect error for unreachable host")
	}
	if !strings.Contains(err.Error(), "mongodb") {
		t.Fatalf("error = %q, want mongodb context", err)
	}
}
