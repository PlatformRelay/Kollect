//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go/modules/minio"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"

	"github.com/konih/kollect/internal/integrationtest"
)

func TestExportMinIO(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx := context.Background()
	container, err := minio.Run(ctx, "minio/minio:latest")
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start minio: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	endpoint, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	accessKey := container.Username
	secretKey := container.Password

	const bucket = "inventory"
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatal(err)
	}

	admin := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String("http://" + endpoint)
		o.UsePathStyle = true
	})

	if _, err := admin.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "http://" + endpoint + "/" + bucket + "/exports",
	}

	backend, err := NewBackend(spec, map[string][]byte{
		"accessKeyID":     []byte(accessKey),
		"secretAccessKey": []byte(secretKey),
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{"ok":true}`)
	if err := backend.Export(ctx, payload, "latest.json"); err != nil {
		t.Fatalf("Export: %v", err)
	}

	out, err := admin.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("exports/latest.json"),
	})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != string(payload) {
		t.Fatalf("object = %q, want %q", data, payload)
	}
}
