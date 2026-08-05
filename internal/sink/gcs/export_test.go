// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/s3"
)

func TestBackend_Export_putsObject(t *testing.T) {
	t.Parallel()

	const bucket = "inventory"
	var gotBucket, gotKey, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			trimmed := strings.TrimPrefix(r.URL.Path, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			if len(parts) == 2 {
				gotBucket = parts[0]
				gotKey = parts[1]
			}
			gotContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			gotBody = body
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("key", "secret", "")),
	)
	if err != nil {
		t.Fatal(err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.UsePathStyle = true
	})

	b := &Backend{inner: s3.NewBackendWithClient(s3.Config{
		Bucket:         bucket,
		Region:         "us-east-1",
		Endpoint:       srv.URL,
		ForcePathStyle: true,
	}, client)}

	payload := []byte(`{"schemaVersion":"v1alpha1","items":[]}`)
	const objectPath = "inventory/team-a/platform.json"
	if err := b.Export(t.Context(), payload, objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if gotBucket != bucket {
		t.Fatalf("bucket = %q, want %q", gotBucket, bucket)
	}
	if gotKey != objectPath {
		t.Fatalf("key = %q, want %q", gotKey, objectPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if string(gotBody) != string(payload) {
		t.Fatalf("body = %q, want %q", gotBody, payload)
	}
}

func TestTestConnection_missingBucket(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}
