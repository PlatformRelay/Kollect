//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go/modules/minio"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/integrationtest"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/validation"
)

// TestRunExportEnvelope_SpillsOversizedPayloadToMinIO proves the reconcile-time write path
// (sink.RunExportEnvelope, the function every controller calls to export a payload) actually
// writes an oversized ("spill-required", ADR-0103) export envelope to a real object-store
// backend, not just that the policy gate in internal/controller/export_spill.go permits it.
//
// package s3_test (external test package) is required here: internal/sink imports
// internal/sink/s3, so an internal `package s3` test cannot import internal/sink without a
// build cycle. External test packages are exempt from that cycle (same shape as net/http's
// httptest), which lets this test drive the same sink.RunExportEnvelope entry point that
// internal/controller/kollectinventory_controller.go and kollectclusterinventory_controller.go
// call in production.
func TestRunExportEnvelope_SpillsOversizedPayloadToMinIO(t *testing.T) {
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

	// Isolate this test's backend from the process-global backend pool (same pattern as
	// internal/sink/layout_export_integration_test.go): other integration tests must not reuse
	// (or have this test reuse) a pooled *s3.Backend built against a different MinIO container.
	sink.DisableBackendPoolForTest()
	t.Cleanup(func() {
		sink.EnableBackendPoolForTest()
		sink.ResetBackendPoolForTest()
	})

	// RunExportEnvelope resolves S3 credentials via internal/sink.ResolveSecret, which
	// short-circuits (never touches the k8s client) when spec.SecretRef is nil. The S3 backend's
	// AWS client then falls back to the default credential chain, which picks up these env vars —
	// so no fake k8s client/Secret is needed to exercise the real export path end-to-end.
	t.Setenv("AWS_ACCESS_KEY_ID", accessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)

	// Build a payload strictly above export.SpillMandatoryBytes (1 MiB) but comfortably under the
	// global cap (1.5 MiB, ADR-0103) so RunExportEnvelope's spill gate is exercised for the reason
	// this test cares about (mandatory spill), not the separate cap-rejection path.
	blobSize := int(export.SpillMandatoryBytes) + 200*1024
	items := []collect.Item{
		{
			Namespace: "team-a",
			Name:      "api",
			Kind:      "Deployment",
			Version:   "v1",
			UID:       "uid-1",
			Attributes: map[string]any{
				"blob": strings.Repeat("A", blobSize),
			},
		},
	}

	envelope, err := export.MarshalEnvelope(items, export.Metadata{Generation: 1, ExportedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}

	assessment := export.AssessSpill(int64(len(envelope)), validation.MaxExportBytesGlobal())
	if !assessment.RequiresSpill {
		t.Fatalf("test payload (%d bytes) does not require spill; adjust blobSize", len(envelope))
	}
	if assessment.ExceedsCap {
		t.Fatalf("test payload (%d bytes) exceeds the global cap; adjust blobSize", len(envelope))
	}

	const objectPath = "inventory/team-a/apps.json"

	err = sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
		Ctx:           ctx,
		Registry:      sink.NewRegistry(),
		SinkNamespace: "default",
		SinkName:      "spill-s3",
		ObjectPath:    objectPath,
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "s3",
			Endpoint: "http://" + endpoint + "/" + bucket,
		},
	})
	if err != nil {
		t.Fatalf("RunExportEnvelope: %v", err)
	}

	out, err := admin.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectPath),
	})
	if err != nil {
		t.Fatalf("GetObject %q: %v", objectPath, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != len(envelope) {
		t.Fatalf("spilled object size = %d bytes, want %d bytes", len(data), len(envelope))
	}
	if string(data) != string(envelope) {
		t.Fatal("spilled object content does not match the exported envelope")
	}
}
