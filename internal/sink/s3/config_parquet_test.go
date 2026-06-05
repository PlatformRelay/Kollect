// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConfigFromSpecParquet(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Cluster:  "prod-west",
		Endpoint: "s3://inventory-bucket/prefix",
		ObjectStore: &kollectdevv1alpha1.ObjectStoreSpec{
			Format:        "parquet",
			HotAttributes: []string{"image", "version"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Format != "parquet" {
		t.Fatalf("format = %q", cfg.Format)
	}

	if cfg.Cluster != "prod-west" {
		t.Fatalf("cluster = %q", cfg.Cluster)
	}

	if len(cfg.HotAttributes) != 2 || cfg.HotAttributes[0] != "image" {
		t.Fatalf("hotAttributes = %v", cfg.HotAttributes)
	}
}
