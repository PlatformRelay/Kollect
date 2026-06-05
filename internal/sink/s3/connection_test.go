// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTestConnection_missingBucket(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "",
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func TestTestConnection_invalidEndpoint(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "s3://",
	}, nil)
	if err == nil {
		t.Fatal("expected error for bucket-less endpoint")
	}
}
