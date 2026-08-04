// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

type fakeCollectionAdmin struct {
	names       []string
	listErr     error
	createErr   error
	indexErr    error
	listCalls   int
	createCalls int
	indexCalls  int
	lastKeys    bson.D
}

func (f *fakeCollectionAdmin) ListCollectionNames(context.Context, interface{}) ([]string, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]string(nil), f.names...), nil
}

func (f *fakeCollectionAdmin) CreateCollection(context.Context, string) error {
	f.createCalls++
	return f.createErr
}

func (f *fakeCollectionAdmin) EnsureUniqueIndex(_ context.Context, keys bson.D) error {
	f.indexCalls++
	f.lastKeys = keys
	return f.indexErr
}

func TestVerifyCollection_found(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{names: []string{"items"}}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	if err := b.verifyCollection(t.Context()); err != nil {
		t.Fatalf("verifyCollection() = %v", err)
	}
}

func TestVerifyCollection_missing(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{names: []string{"other"}}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	err := b.verifyCollection(t.Context())
	if err == nil || !strings.Contains(err.Error(), "provisioning.mode=existing") {
		t.Fatalf("error = %v, want missing collection", err)
	}
}

func TestVerifyCollection_listError(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{listErr: errors.New("boom")}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	err := b.verifyCollection(t.Context())
	if err == nil || !strings.Contains(err.Error(), "mongodb verify collection") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureCollection_createsWhenAbsent(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{names: nil}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	if err := b.ensureCollection(t.Context()); err != nil {
		t.Fatalf("ensureCollection() = %v", err)
	}
	if fa.createCalls != 1 || fa.indexCalls != 1 {
		t.Fatalf("create=%d index=%d, want 1/1", fa.createCalls, fa.indexCalls)
	}
}

func TestEnsureCollection_skipsCreateWhenPresent(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{names: []string{"items"}}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	if err := b.ensureCollection(t.Context()); err != nil {
		t.Fatalf("ensureCollection() = %v", err)
	}
	if fa.createCalls != 0 || fa.indexCalls != 1 {
		t.Fatalf("create=%d index=%d, want 0/1", fa.createCalls, fa.indexCalls)
	}
}

func TestEnsureCollection_indexError(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{names: []string{"items"}, indexErr: errors.New("idx")}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	err := b.ensureCollection(t.Context())
	if err == nil || !strings.Contains(err.Error(), "mongodb ensure index") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureCollection_listError(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{listErr: errors.New("list boom")}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	err := b.ensureCollection(t.Context())
	if err == nil || !strings.Contains(err.Error(), "mongodb list collections") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureCollection_createError(t *testing.T) {
	t.Parallel()
	fa := &fakeCollectionAdmin{names: nil, createErr: errors.New("create boom")}
	b := &Backend{cfg: Config{Database: "db", Collection: "items"}, admin: fa}
	err := b.ensureCollection(t.Context())
	if err == nil || !strings.Contains(err.Error(), "mongodb create collection") {
		t.Fatalf("error = %v", err)
	}
}
