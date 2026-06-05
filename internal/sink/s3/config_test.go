// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConfigFromSpec(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "s3://my-bucket/inventory/prefix",
	}, map[string][]byte{
		"accessKeyID":     []byte("a"),
		"secretAccessKey": []byte("b"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Bucket != "my-bucket" {
		t.Fatalf("bucket = %q", cfg.Bucket)
	}

	if cfg.Prefix != "inventory/prefix" {
		t.Fatalf("prefix = %q", cfg.Prefix)
	}
}
