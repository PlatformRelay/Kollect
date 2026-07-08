// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func newTestBackend(t *testing.T, outputDir string) *Backend {
	t.Helper()

	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: TypeName, Endpoint: outputDir}, nil)
	if err != nil {
		t.Fatalf("NewBackend() error = %v", err)
	}

	return b
}

func TestExport_writesNewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := newTestBackend(t, dir)

	payload := []byte(`{"items":[]}`)
	if err := b.Export(context.Background(), payload, "default/inv.yaml"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "default/inv.yaml")) //nolint:gosec // G304: path built from t.TempDir() + fixed literal
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("written content = %q, want %q", got, payload)
	}
}

func TestExport_skipsIdenticalContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := newTestBackend(t, dir)

	payload := []byte(`{"items":[]}`)
	if err := b.Export(context.Background(), payload, "default/inv.yaml"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	full := filepath.Join(dir, "default/inv.yaml")
	before, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}

	// Force mtime backwards so a real rewrite would be detectable.
	past := time.Now().Add(-time.Hour)
	if chtimesErr := os.Chtimes(full, past, past); chtimesErr != nil {
		t.Fatal(chtimesErr)
	}

	if exportErr := b.Export(context.Background(), payload, "default/inv.yaml"); exportErr != nil {
		t.Fatalf("second Export() error = %v", exportErr)
	}

	after, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(past) {
		t.Errorf("expected identical-content export to skip write (mtime unchanged), before=%v after=%v", before.ModTime(), after.ModTime())
	}
}

func TestExport_overwritesDifferentContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := newTestBackend(t, dir)

	if err := b.Export(context.Background(), []byte("A"), "default/inv.yaml"); err != nil {
		t.Fatal(err)
	}
	if err := b.Export(context.Background(), []byte("B"), "default/inv.yaml"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "default/inv.yaml")) //nolint:gosec // G304: path built from t.TempDir() + fixed literal
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "B" {
		t.Errorf("content = %q, want %q", got, "B")
	}
}

func TestExport_createsNestedDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := newTestBackend(t, dir)

	if err := b.Export(context.Background(), []byte("x"), "cluster-a/prod/default/inv.json"); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cluster-a/prod/default/inv.json")); err != nil {
		t.Errorf("expected nested file to exist: %v", err)
	}
}

func TestExport_rejectsPathTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := newTestBackend(t, dir)

	err := b.Export(context.Background(), []byte("x"), "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "escapes output directory") {
		t.Errorf("expected error to mention escaping output directory, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "etc", "passwd")); statErr == nil {
		t.Error("expected no file to be written outside outputDir")
	}
}

func TestExport_rejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := newTestBackend(t, dir)

	if err := b.Export(context.Background(), []byte("x"), "/etc/shadow"); err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestNewBackend_emptyEndpointReturnsError(t *testing.T) {
	t.Parallel()

	_, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{Type: TypeName, Endpoint: ""}, nil)
	if err == nil {
		t.Fatal("expected error for empty outputDir, got nil")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	b := newTestBackend(t, t.TempDir())

	c := b.Capabilities()
	if c.Stream {
		t.Error("expected Stream = false for a snapshot store")
	}
}

func TestType(t *testing.T) {
	t.Parallel()

	b := newTestBackend(t, t.TempDir())
	if b.Type() != TypeName {
		t.Errorf("Type() = %q, want %q", b.Type(), TypeName)
	}
}
