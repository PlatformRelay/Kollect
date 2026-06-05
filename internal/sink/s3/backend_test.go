// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

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

	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/objectstore"
)

func TestBackend_TypeAndCapabilities(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	if b.Type() != "s3" {
		t.Fatalf("Type() = %q", b.Type())
	}
	if b.Capabilities() != cap.ObjectStoreSnapshot() {
		t.Fatalf("Capabilities() = %#v", b.Capabilities())
	}
}

func TestBackend_Export_rejectsEmptyJSONPayload(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{Bucket: "inventory", Region: "us-east-1"}}
	err := b.Export(context.Background(), nil, "inventory/default/inv.json")
	if err == nil || !strings.Contains(err.Error(), "empty payload") {
		t.Fatalf("Export() = %v, want empty payload error", err)
	}
}

func TestBackend_Export_putsObject(t *testing.T) {
	t.Parallel()

	const bucket = "inventory"
	var gotBucket, gotKey, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			gotBucket = r.URL.Query().Get("bucket")
			if gotBucket == "" {
				// path-style: /bucket/key
				trimmed := strings.TrimPrefix(r.URL.Path, "/")
				parts := strings.SplitN(trimmed, "/", 2)
				if len(parts) == 2 {
					gotBucket = parts[0]
					gotKey = parts[1]
				}
			} else {
				gotKey = strings.TrimPrefix(r.URL.Path, "/")
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

	b := &Backend{
		cfg: Config{
			Bucket:         bucket,
			Region:         "us-east-1",
			Endpoint:       srv.URL,
			ForcePathStyle: true,
			Format:         objectstore.FormatJSON,
		},
		client: client,
	}

	payload := []byte(`{"schemaVersion":"v1alpha1","items":[]}`)
	const objectPath = "inventory/team-a/platform.json"
	if err := b.Export(context.Background(), payload, objectPath); err != nil {
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

func TestBackend_Export_defaultObjectPath(t *testing.T) {
	t.Parallel()

	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			trimmed := strings.TrimPrefix(r.URL.Path, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			if len(parts) == 2 {
				gotKey = parts[1]
			}
			w.WriteHeader(http.StatusOK)
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

	b := &Backend{
		cfg: Config{
			Bucket:         "inventory",
			Region:         "us-east-1",
			Endpoint:       srv.URL,
			ForcePathStyle: true,
		},
		client: client,
	}

	if err := b.Export(context.Background(), []byte(`{"items":[]}`), "  "); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if gotKey != "inventory/latest.json" {
		t.Fatalf("default key = %q, want inventory/latest.json", gotKey)
	}
}

func TestBackend_Export_withPrefix(t *testing.T) {
	t.Parallel()

	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			trimmed := strings.TrimPrefix(r.URL.Path, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			if len(parts) == 2 {
				gotKey = parts[1]
			}
			w.WriteHeader(http.StatusOK)
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

	b := &Backend{
		cfg: Config{
			Bucket:         "inventory",
			Region:         "us-east-1",
			Prefix:         "exports",
			Endpoint:       srv.URL,
			ForcePathStyle: true,
		},
		client: client,
	}

	const objectPath = "inventory/team-a/inv.json"
	if err := b.Export(context.Background(), []byte(`{"items":[]}`), objectPath); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if gotKey != "exports/inventory/team-a/inv.json" {
		t.Fatalf("prefixed key = %q", gotKey)
	}
}
