// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// scriptedS3 serves a fixed key set and can fail list or delete calls and page
// the listing, so the cleanup error and pagination paths are exercised through
// the real S3 client.
type scriptedS3 struct {
	bucket    string
	keys      []string
	failList  bool
	failDel   bool
	pageSize  int
	extraKeys []string // keys returned on the first page even if not prefix-matching
}

func (s *scriptedS3) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if s.failList {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
			if r.URL.Query().Get("list-type") != "2" {
				http.NotFound(w, r)

				return
			}
			prefix := r.URL.Query().Get("prefix")
			token := r.URL.Query().Get("continuation-token")

			var matches []string
			if token == "" {
				matches = append(matches, s.extraKeys...)
			}
			for _, k := range s.keys {
				if strings.HasPrefix(k, prefix) {
					matches = append(matches, k)
				}
			}
			sort.Strings(matches)

			truncated := false
			next := ""
			if s.pageSize > 0 && len(matches) > s.pageSize {
				next = "page-2"
				matches = matches[:s.pageSize]
				truncated = true
			}
			if token == "page-2" && s.pageSize > 0 {
				all := s.keys
				matches = nil
				for _, k := range all {
					if strings.HasPrefix(k, prefix) {
						matches = append(matches, k)
					}
				}
				sort.Strings(matches)
				if len(matches) > s.pageSize {
					matches = matches[s.pageSize:]
				}
				truncated = false
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"+
				"<ListBucketResult xmlns=\"http://s3.amazonaws.com/doc/2006-03-01/\">"+
				"<Name>%s</Name><Prefix>%s</Prefix><KeyCount>%d</KeyCount><MaxKeys>1000</MaxKeys>"+
				"<IsTruncated>%t</IsTruncated>", s.bucket, prefix, len(matches), truncated)
			if truncated {
				fmt.Fprintf(&sb, "<NextContinuationToken>%s</NextContinuationToken>", next)
			}
			for _, k := range matches {
				fmt.Fprintf(&sb, "<Contents><Key>%s</Key><LastModified>2026-01-01T00:00:00.000Z</LastModified>"+
					"<ETag>&quot;x&quot;</ETag><Size>1</Size><StorageClass>STANDARD</StorageClass></Contents>", k)
			}
			sb.WriteString("</ListBucketResult>")
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sb.String()))
		case http.MethodDelete:
			if s.failDel {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
}

func newScriptedS3Backend(t *testing.T, s *scriptedS3, prefix string) *Backend {
	t.Helper()

	srv := httptest.NewServer(s.handler())
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

	return &Backend{
		cfg: Config{
			Bucket:         s.bucket,
			Region:         "us-east-1",
			Endpoint:       srv.URL,
			ForcePathStyle: true,
			Prefix:         prefix,
			Format:         "json",
		},
		client: client,
	}
}

// Blank candidate paths are skipped without any list or delete call.
func TestBackend_DeleteExport_SkipsBlankPaths(t *testing.T) {
	t.Parallel()

	s := &scriptedS3{bucket: "inventory", failList: true}
	b := newScriptedS3Backend(t, s, "")

	deleted, err := b.DeleteExport(t.Context(), []string{"   ", ""})
	if err != nil || deleted != nil {
		t.Fatalf("blank paths = %v/%v, want nil/nil (no list call)", deleted, err)
	}
}

// A list failure is surfaced, not swallowed.
func TestBackend_DeleteExport_ListErrorPropagates(t *testing.T) {
	t.Parallel()

	s := &scriptedS3{bucket: "inventory", failList: true}
	b := newScriptedS3Backend(t, s, "")

	if _, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"}); err == nil {
		t.Fatal("list failure must surface as an error")
	}
}

// A delete failure is surfaced, not swallowed.
func TestBackend_DeleteExport_DeleteErrorPropagates(t *testing.T) {
	t.Parallel()

	s := &scriptedS3{bucket: "inventory", failDel: true, keys: []string{"inventory/team-a/apps.json"}}
	b := newScriptedS3Backend(t, s, "")

	if _, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"}); err == nil {
		t.Fatal("delete failure must surface as an error")
	}
}

// A parquet hive matcher (prefix ending in "/") combined with a bucket prefix
// preserves the trailing slash of the list prefix.
func TestBackend_DeleteExport_ParquetWithBucketPrefix(t *testing.T) {
	t.Parallel()

	s := &scriptedS3{bucket: "inventory", keys: []string{
		"tenant/inventory/cluster=c1/ns=team-a/name=apps/generation=7.parquet",
		"tenant/inventory/cluster=c1/ns=team-b/name=apps/generation=7.parquet",
	}}
	b := newScriptedS3Backend(t, s, "tenant")

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/cluster=c1/ns=team-a/name=apps/generation=7.parquet"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "inventory/cluster=c1/ns=team-a/name=apps/generation=7.parquet" {
		t.Fatalf("deleted = %v, want the team-a partition object", deleted)
	}
}

// A listed key outside the bucket prefix (rel == key) is skipped, not deleted.
func TestBackend_DeleteExport_SkipsKeysOutsidePrefix(t *testing.T) {
	t.Parallel()

	s := &scriptedS3{
		bucket:    "inventory",
		keys:      []string{"tenant/inventory/team-a/apps.json"},
		extraKeys: []string{"inventory/team-a/apps.json"},
	}
	b := newScriptedS3Backend(t, s, "tenant")

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "inventory/team-a/apps.json" {
		t.Fatalf("deleted = %v, want only the prefixed key", deleted)
	}
}

// A paginated listing is followed through the continuation token.
func TestBackend_DeleteExport_FollowsContinuationToken(t *testing.T) {
	t.Parallel()

	s := &scriptedS3{bucket: "inventory", pageSize: 1, keys: []string{
		"inventory/team-a/apps.json",
		"inventory/team-a/apps.part-0001-of-0002.json",
	}}
	b := newScriptedS3Backend(t, s, "")

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("deleted = %v, want both objects across the two pages", deleted)
	}
}
