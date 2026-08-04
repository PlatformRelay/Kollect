// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"

	"google.golang.org/api/googleapi"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

type fakeTableHandle struct {
	metaErr   error
	createErr error
	metaCalls int
	creates   []*bigquery.TableMetadata
}

func (f *fakeTableHandle) Metadata(context.Context) (*bigquery.TableMetadata, error) {
	f.metaCalls++
	if f.metaErr != nil {
		return nil, f.metaErr
	}
	return &bigquery.TableMetadata{Name: "items"}, nil
}

func (f *fakeTableHandle) Create(_ context.Context, metadata *bigquery.TableMetadata) error {
	f.creates = append(f.creates, metadata)
	return f.createErr
}

func TestVerifyTable_existingModeSuccess(t *testing.T) {
	t.Parallel()
	ft := &fakeTableHandle{}
	b := &Backend{
		cfg:   Config{Dataset: "ds", Table: "items", ProvisioningMode: kollectdevv1alpha1.ProvisioningModeExisting},
		table: ft,
	}
	if err := b.verifyTable(t.Context()); err != nil {
		t.Fatalf("verifyTable() = %v, want nil", err)
	}
	if ft.metaCalls != 1 {
		t.Fatalf("metaCalls = %d, want 1", ft.metaCalls)
	}
}

func TestVerifyTable_missingTableClassified(t *testing.T) {
	t.Parallel()
	ft := &fakeTableHandle{metaErr: errors.New("not found")}
	b := &Backend{
		cfg:   Config{Dataset: "ds", Table: "items"},
		table: ft,
	}
	err := b.verifyTable(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "provisioning.mode=existing") {
		t.Fatalf("error = %v, want provisioning.mode=existing context", err)
	}
}

func TestEnsureTable_createsExpectedSchema(t *testing.T) {
	t.Parallel()
	ft := &fakeTableHandle{}
	b := &Backend{
		cfg:   Config{Dataset: "ds", Table: "items"},
		table: ft,
	}
	if err := b.ensureTable(t.Context()); err != nil {
		t.Fatalf("ensureTable() = %v", err)
	}
	if len(ft.creates) != 1 {
		t.Fatalf("creates = %d, want 1", len(ft.creates))
	}
	md := ft.creates[0]
	if md.TimePartitioning == nil || md.TimePartitioning.Field != "exported_at" {
		t.Fatalf("partitioning = %#v", md.TimePartitioning)
	}
	names := make([]string, 0, len(md.Schema))
	for _, f := range md.Schema {
		names = append(names, f.Name)
	}
	want := []string{
		"inventory_namespace", "inventory_name", "target_name", "source_uid",
		"cluster", "resource_namespace", "payload", "exported_at",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("schema fields = %v, want %v", names, want)
	}
}

func TestEnsureTable_duplicateCreateIsOK(t *testing.T) {
	t.Parallel()
	dup := &googleapi.Error{Code: 409, Errors: []googleapi.ErrorItem{{Reason: "duplicate"}}}
	ft := &fakeTableHandle{createErr: dup}
	b := &Backend{cfg: Config{Dataset: "ds", Table: "items"}, table: ft}
	if err := b.ensureTable(t.Context()); err != nil {
		t.Fatalf("ensureTable() = %v, want nil for duplicate create", err)
	}
}

func TestEnsureTable_createErrorClassified(t *testing.T) {
	t.Parallel()
	ft := &fakeTableHandle{createErr: &googleapi.Error{Code: 403, Errors: []googleapi.ErrorItem{{Reason: "accessDenied"}}}}
	b := &Backend{cfg: Config{Dataset: "ds", Table: "items"}, table: ft}
	err := b.ensureTable(t.Context())
	if err == nil {
		t.Fatal("expected ensureTable error")
	}
	if !strings.Contains(err.Error(), "bigquery ensure table") {
		t.Fatalf("error = %v, want ensure table wrap", err)
	}
}
