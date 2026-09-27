// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/platformrelay/kollect/internal/sink/s3"
)

// A GCS backend retracts its exported objects through the shared S3-compatible
// cleanup path.
func TestBackend_DeleteExport_removesObjects(t *testing.T) {
	t.Parallel()

	const bucket = "inventory"
	keys := map[string]struct{}{
		"inventory/team-a/apps.json":                   {},
		"inventory/team-a/apps.part-0001-of-0002.json": {},
		"inventory/team-b/apps.json":                   {},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/"+bucket+"/")
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("list-type") != "2" {
				http.NotFound(w, r)

				return
			}
			prefix := r.URL.Query().Get("prefix")
			var sb strings.Builder
			sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` +
				`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">` +
				`<Name>` + bucket + `</Name><Prefix>` + prefix + `</Prefix>` +
				`<KeyCount>0</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>`)
			for k := range keys {
				if strings.HasPrefix(k, prefix) {
					fmt.Fprintf(&sb, "<Contents><Key>%s</Key><LastModified>2026-01-01T00:00:00.000Z</LastModified>"+
						"<ETag>&quot;x&quot;</ETag><Size>1</Size><StorageClass>STANDARD</StorageClass></Contents>", k)
				}
			}
			sb.WriteString("</ListBucketResult>")
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sb.String()))
		case http.MethodDelete:
			delete(keys, key)
			w.WriteHeader(http.StatusNoContent)
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

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("deleted = %v, want the object and its part sibling", deleted)
	}
	if _, ok := keys["inventory/team-a/apps.json"]; ok {
		t.Fatal("owned object must be deleted")
	}
	if _, ok := keys["inventory/team-b/apps.json"]; !ok {
		t.Fatal("sibling inventory object must survive")
	}
}
