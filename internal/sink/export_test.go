// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/objectstore"
)

type stubBackend struct {
	caps      cap.Capabilities
	exportErr error
	lastPath  string
	lastBody  []byte
}

func (s *stubBackend) Type() string { return "stub" }

func (s *stubBackend) Capabilities() cap.Capabilities { return s.caps }

func (s *stubBackend) Export(_ context.Context, payload []byte, path string) error {
	s.lastPath = path
	s.lastBody = append([]byte(nil), payload...)

	return s.exportErr
}

type closableStubBackend struct {
	stubBackend
	closed   bool
	closeErr error
}

func (c *closableStubBackend) Close() error {
	c.closed = true

	return c.closeErr
}

func TestExportErrorReason(t *testing.T) {
	t.Parallel()

	if ExportErrorReason(nil) != "unknown" {
		t.Fatal("nil error should map to unknown")
	}

	if ExportErrorReason(kollecterrors.Terminal(errors.New("bad"))) != "terminal" {
		t.Fatal("terminal error label")
	}

	if ExportErrorReason(kollecterrors.Forbidden(errors.New("denied"))) != "forbidden" {
		t.Fatal("forbidden error label")
	}

	if ExportErrorReason(kollecterrors.Transient(errors.New("retry"))) != "transient" {
		t.Fatal("transient error label")
	}
}

func TestRunExportItems_nilRegistry(t *testing.T) {
	t.Parallel()

	err := RunExportItems(ExportItemsRequest{Ctx: t.Context()})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("RunExportItems() = %v, want terminal registry error", err)
	}
}

func TestRunExportItems_sinkNotFound(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).Build(),
		Registry:      NewRegistry(),
		SinkNamespace: "team-a",
		SinkName:      "missing",
		ObjectPath:    "team-a/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	})
	if err == nil {
		t.Fatal("expected error for missing KollectSink")
	}
}

func TestRunExportItems_exportsSnapshot(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-sink", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "stub",
			Endpoint: "https://example.com/inventory.git",
		},
	}

	reg := NewRegistry()
	stub := &stubBackend{caps: cap.SnapshotStore()}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "git-sink",
		ObjectPath:    "team-a/platform.json",
		Items: []collect.Item{
			{Name: "demo", Namespace: "apps", Kind: "Deployment", Version: "v1"},
		},
		Meta: export.Metadata{Cluster: "local", Generation: 1},
	})
	if err != nil {
		t.Fatalf("RunExportItems() = %v", err)
	}

	invNS, invName := objectstore.InventoryFromObjectPath("team-a/platform.json")
	wantPath := objectstore.ObjectPath(sinkObj.Spec, invNS, invName, 1)
	if stub.lastPath != wantPath {
		t.Fatalf("export path = %q, want %q", stub.lastPath, wantPath)
	}

	if len(stub.lastBody) == 0 || !strings.Contains(string(stub.lastBody), "schemaVersion") {
		t.Fatalf("expected envelope payload, got %q", stub.lastBody)
	}
}

func TestRunExportItems_skipsEmptySnapshotStream(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-sink", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "stub",
			Kafka: &kollectdevv1alpha1.KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "inventory",
			},
		},
	}

	reg := NewRegistry()
	stub := &stubBackend{caps: cap.StreamEmitter()}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "kafka-sink",
		ObjectPath:    "team-a/events",
		Items:         nil,
	}); err != nil {
		t.Fatalf("RunExportItems() = %v, want skip without error", err)
	}

	if stub.lastBody != nil {
		t.Fatal("stream emitter should skip empty snapshot export")
	}
}

func TestRunExportItems_postgresExportsEmptySnapshot(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-sink", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "items",
			},
		},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "team-a"},
		Data:       map[string][]byte{"dsn": []byte("postgres://localhost/db")},
	}

	reg := NewRegistry()
	stub := &stubBackend{caps: cap.RelationalStore()}
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext,
	) (Backend, error) {
		return stub, nil
	})

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj, pgSecret).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "pg-sink",
		ObjectPath:    "team-a/inv",
		Items:         nil,
	}); err != nil {
		t.Fatalf("RunExportItems() = %v", err)
	}

	if len(stub.lastBody) == 0 {
		t.Fatal("relational store should export empty snapshot for delete reconciliation")
	}
}

func TestRunExportItems_closeErrorDoesNotFailExport(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "close-err", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "stub", Endpoint: "https://example.com/repo.git"},
	}

	reg := NewRegistry()
	stub := &closableStubBackend{
		stubBackend: stubBackend{caps: cap.SnapshotStore()},
		closeErr:    errors.New("pool close failed"),
	}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	t.Cleanup(func() { EvictBackendPool("team-a", "close-err") })

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "close-err",
		ObjectPath:    "team-a/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	}); err != nil {
		t.Fatalf("RunExportItems() = %v, want export success despite close error", err)
	}

	EvictBackendPool("team-a", "close-err")
	if !stub.closed {
		t.Fatal("expected backend Close on pool eviction")
	}
}

func TestRunExportItems_poolsBackendUntilEvict(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-close", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "stub", Endpoint: "https://example.com/repo.git"},
	}

	reg := NewRegistry()
	stub := &closableStubBackend{stubBackend: stubBackend{caps: cap.SnapshotStore()}}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	t.Cleanup(func() { EvictBackendPool("team-a", "pool-close") })

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "pool-close",
		ObjectPath:    "team-a/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	}); err != nil {
		t.Fatalf("RunExportItems() = %v", err)
	}

	if stub.closed {
		t.Fatal("pooled backend must not Close after each export")
	}

	EvictBackendPool("team-a", "pool-close")
	if !stub.closed {
		t.Fatal("expected backend Close on pool eviction")
	}
}

func TestRunExportItems_exportFailureTransient(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "export-fail", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "stub", Endpoint: "https://example.com/repo.git"},
	}

	reg := NewRegistry()
	stub := &stubBackend{
		caps:      cap.SnapshotStore(),
		exportErr: errors.New("network down"),
	}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	t.Cleanup(func() { EvictBackendPool("team-a", "export-fail") })

	err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "export-fail",
		ObjectPath:    "team-a/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
		t.Fatalf("RunExportItems() = %v (%v), want transient export error", err, kollecterrors.ClassOf(err))
	}
}
