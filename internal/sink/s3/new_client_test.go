// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// newClient is construction-time pure: LoadDefaultConfig and NewFromConfig build
// providers lazily, so no network is touched. We assert only the deterministic
// fields newClient sets unconditionally (Region, BaseEndpoint, UsePathStyle) and
// deliberately avoid credential assertions, which can leak in from ambient env.

func TestNewClient_MapsEndpointAndPathStyle(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Region:         "eu-central-1",
		Endpoint:       "http://minio:9000",
		ForcePathStyle: true,
	}

	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}

	opts := client.Options()
	if opts.Region != "eu-central-1" {
		t.Fatalf("region = %q, want eu-central-1", opts.Region)
	}
	if aws.ToString(opts.BaseEndpoint) != "http://minio:9000" {
		t.Fatalf("base endpoint = %q, want http://minio:9000", aws.ToString(opts.BaseEndpoint))
	}
	if !opts.UsePathStyle {
		t.Fatal("UsePathStyle = false, want true when ForcePathStyle set")
	}
}

func TestNewClient_NoEndpointBranch(t *testing.T) {
	t.Parallel()

	// Exercises the else (no-endpoint) branch of newClient. We assert only the
	// deterministic Region: BaseEndpoint can be populated from ambient
	// AWS_ENDPOINT_URL by LoadDefaultConfig, so it is not safe to assert on.
	cfg := Config{Region: "us-east-1"}

	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}

	if got := client.Options().Region; got != "us-east-1" {
		t.Fatalf("region = %q, want us-east-1", got)
	}
}

func TestNewClient_StaticCredentialsAccepted(t *testing.T) {
	t.Parallel()

	// Exercises the static-credentials branch of newClient. We do not assert on
	// resolved credentials (ambient env can leak); only that construction
	// succeeds and the endpoint mapping still holds.
	cfg := Config{
		Region:          "us-west-2",
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secret",
		Endpoint:        "http://localhost:4566",
		ForcePathStyle:  true,
	}

	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if got := aws.ToString(client.Options().BaseEndpoint); got != "http://localhost:4566" {
		t.Fatalf("base endpoint = %q, want http://localhost:4566", got)
	}
}
