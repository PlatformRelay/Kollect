// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakeS3 struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

func (f *fakeS3) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.keys[key]

	return ok
}

func (f *fakeS3) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.keys))
	for k := range f.keys {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}

func (f *fakeS3) handler(t *testing.T, bucket string) http.Handler {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/"+bucket+"/")
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("list-type") != "2" {
				http.NotFound(w, r)

				return
			}
			prefix := r.URL.Query().Get("prefix")
			f.mu.Lock()
			var matches []string
			for k := range f.keys {
				if strings.HasPrefix(k, prefix) {
					matches = append(matches, k)
				}
			}
			f.mu.Unlock()
			sort.Strings(matches)

			var sb strings.Builder
			head := fmt.Sprintf(
				"<Name>%s</Name><Prefix>%s</Prefix><KeyCount>%d</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>",
				bucket, prefix, len(matches))
			sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` +
				`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">` + head)
			for _, k := range matches {
				fmt.Fprintf(&sb,
					"<Contents><Key>%s</Key><LastModified>2026-01-01T00:00:00.000Z</LastModified>"+
						"<ETag>&quot;x&quot;</ETag><Size>1</Size><StorageClass>STANDARD</StorageClass></Contents>", k)
			}
			sb.WriteString("</ListBucketResult>")
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sb.String()))
		case http.MethodDelete:
			f.mu.Lock()
			delete(f.keys, key)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
}

func newFakeS3Backend(t *testing.T, f *fakeS3, bucket, keyPrefix string) *Backend {
	t.Helper()

	srv := httptest.NewServer(f.handler(t, bucket))
	t.Cleanup(srv.Close)

	awsCfg, err := awsconfig.LoadDefaultConfig(t.Context(),
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
			Bucket:         bucket,
			Region:         "us-east-1",
			Endpoint:       srv.URL,
			ForcePathStyle: true,
			Prefix:         keyPrefix,
			Format:         "json",
		},
		client: client,
	}
}

// K-28 test lock (tombstone): inventory deletion removes the object the export
// wrote plus its deterministic part siblings — and nothing else.
func TestBackend_DeleteExport_removesObjectAndPartSiblingsOnly(t *testing.T) {
	t.Parallel()

	f := &fakeS3{keys: map[string]struct{}{}}
	for _, k := range []string{
		"inventory/team-a/apps.json",
		"inventory/team-a/apps.part-0001-of-0002.json",
		"inventory/team-a/apps.part-0002-of-0002.json",
		"inventory/team-a/apps-v2.json",
		"inventory/team-a/apps.json.bak",
		"inventory/team-a/appsonly.json",
		"inventory/team-b/apps.json",
	} {
		f.keys[k] = struct{}{}
	}

	b := newFakeS3Backend(t, f, "inventory", "")

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("deleted = %v, want the object and its two part siblings", deleted)
	}

	for _, gone := range []string{
		"inventory/team-a/apps.json",
		"inventory/team-a/apps.part-0001-of-0002.json",
		"inventory/team-a/apps.part-0002-of-0002.json",
	} {
		if f.has(gone) {
			t.Errorf("key %q should have been deleted, keys=%v", gone, f.snapshot())
		}
	}

	for _, kept := range []string{
		"inventory/team-a/apps-v2.json",
		"inventory/team-a/apps.json.bak",
		"inventory/team-a/appsonly.json",
		"inventory/team-b/apps.json",
	} {
		if !f.has(kept) {
			t.Errorf("key %q must survive cleanup, keys=%v", kept, f.snapshot())
		}
	}
}

func TestBackend_DeleteExport_sweepsParquetHivePartition(t *testing.T) {
	t.Parallel()

	f := &fakeS3{keys: map[string]struct{}{}}
	for _, k := range []string{
		"inventory/cluster=c1/ns=team-a/name=apps/generation=7.parquet",
		"inventory/cluster=c1/ns=team-a/name=apps/generation=3.parquet",
		"inventory/cluster=c1/ns=team-a/name=apps/generation=9.part-0001-of-0002.parquet",
		"inventory/cluster=c1/ns=team-a/name=apps-v2/generation=7.parquet",
		"inventory/cluster=c1/ns=team-b/name=apps/generation=7.parquet",
	} {
		f.keys[k] = struct{}{}
	}

	b := newFakeS3Backend(t, f, "inventory", "")

	deleted, err := b.DeleteExport(t.Context(), []string{
		"inventory/cluster=c1/ns=team-a/name=apps/generation=7.parquet",
	})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("deleted = %v, want all generations and part variants in the owned partition", deleted)
	}

	if got, want := f.snapshot(), []string{
		"inventory/cluster=c1/ns=team-a/name=apps-v2/generation=7.parquet",
		"inventory/cluster=c1/ns=team-b/name=apps/generation=7.parquet",
	}; len(got) != len(want) {
		t.Fatalf("surviving keys = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("surviving keys = %v, want %v", got, want)
			}
		}
	}
}

func TestBackend_DeleteExport_withBucketPrefix(t *testing.T) {
	t.Parallel()

	f := &fakeS3{keys: map[string]struct{}{
		"tenant/inventory/team-a/apps.json":                   {},
		"tenant/inventory/team-a/apps.part-0001-of-0002.json": {},
		"tenant/inventory/team-a/apps-v2.json":                {},
	}}

	b := newFakeS3Backend(t, f, "inventory", "tenant")

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/apps.json"})
	if err != nil {
		t.Fatalf("DeleteExport: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("deleted = %v, want the object and its present part sibling (relative to the bucket prefix)", deleted)
	}
	for _, rel := range deleted {
		if strings.HasPrefix(rel, "tenant/") {
			t.Fatalf("deleted path %q must be relative to the sink prefix", rel)
		}
	}

	if f.has("tenant/inventory/team-a/apps.json") || f.has("tenant/inventory/team-a/apps.part-0001-of-0002.json") {
		t.Fatalf("owned keys should be deleted, keys=%v", f.snapshot())
	}
	if !f.has("tenant/inventory/team-a/apps-v2.json") {
		t.Fatalf("sibling inventory key must survive, keys=%v", f.snapshot())
	}
}

// Cleanup is retried: an empty backend (or one that already lost the objects)
// must succeed, not wedge the finalizer.
func TestBackend_DeleteExport_missingObjectsSucceed(t *testing.T) {
	t.Parallel()

	f := &fakeS3{keys: map[string]struct{}{"unrelated/key.json": {}}}
	b := newFakeS3Backend(t, f, "inventory", "")

	if deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/gone.json"}); err != nil || deleted != nil {
		t.Fatalf("DeleteExport on empty backend = %v/%v, want nil/nil", deleted, err)
	}
	if deleted, err := b.DeleteExport(t.Context(), nil); err != nil || deleted != nil {
		t.Fatalf("DeleteExport(nil paths) = %v/%v, want nil/nil", deleted, err)
	}
}
