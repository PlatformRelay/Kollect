// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestNewBackendWithClient_usesInjectedClient(t *testing.T) {
	t.Parallel()

	awsCfg, err := awsconfig.LoadDefaultConfig(t.Context(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("key", "secret", "")),
	)
	if err != nil {
		t.Fatal(err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String("http://example.invalid")
		o.UsePathStyle = true
	})

	cfg := Config{Bucket: "inventory", Region: "us-east-1", Endpoint: "http://example.invalid", ForcePathStyle: true}
	b := NewBackendWithClient(cfg, client)
	if b == nil || b.client != client {
		t.Fatal("NewBackendWithClient did not wire the injected client")
	}
	if b.cfg.Bucket != "inventory" {
		t.Fatalf("cfg.Bucket = %q", b.cfg.Bucket)
	}
}
