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

func TestConfigFromSpec_wrongType(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "git"}, nil)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestConfigFromSpec_missingEndpoint(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "s3", Endpoint: "   "}, nil)
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
}

func TestConfigFromSpec_awsEnvKeys(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "s3://bucket",
	}, map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte("keyid"),
		"AWS_SECRET_ACCESS_KEY": []byte("secret"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AccessKeyID != "keyid" || cfg.SecretAccessKey != "secret" {
		t.Fatalf("creds not resolved from AWS env keys: %#v", cfg)
	}
}

func TestParseEndpoint_urlScheme(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	parseEndpoint("http://minio:9000/my-bucket/data/prefix", &cfg)

	if cfg.Endpoint != "http://minio:9000" {
		t.Fatalf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.Bucket != "my-bucket" {
		t.Fatalf("bucket = %q", cfg.Bucket)
	}
	if cfg.Prefix != "data/prefix" {
		t.Fatalf("prefix = %q", cfg.Prefix)
	}
}

func TestParseEndpoint_urlSchemeNoBucket(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	parseEndpoint("http://minio:9000", &cfg)

	if cfg.Endpoint != "http://minio:9000" {
		t.Fatalf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.Bucket != "" {
		t.Fatalf("bucket should be empty, got %q", cfg.Bucket)
	}
}

func TestParseEndpoint_bareBucket(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	parseEndpoint("only-bucket", &cfg)

	if cfg.Bucket != "only-bucket" {
		t.Fatalf("bucket = %q", cfg.Bucket)
	}
	if cfg.Prefix != "" {
		t.Fatalf("prefix should be empty, got %q", cfg.Prefix)
	}
}

func TestParseEndpoint_bareBucketWithPrefix(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	parseEndpoint("my-bucket/sub/path/", &cfg)

	if cfg.Bucket != "my-bucket" {
		t.Fatalf("bucket = %q", cfg.Bucket)
	}
	if cfg.Prefix != "sub/path" {
		t.Fatalf("prefix = %q", cfg.Prefix)
	}
}

func TestParseEndpoint_s3NoBucketPrefix(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	parseEndpoint("s3://just-bucket", &cfg)

	if cfg.Bucket != "just-bucket" {
		t.Fatalf("bucket = %q", cfg.Bucket)
	}
	if cfg.Prefix != "" {
		t.Fatalf("prefix should be empty, got %q", cfg.Prefix)
	}
}
