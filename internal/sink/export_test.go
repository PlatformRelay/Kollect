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

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/cap"
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
		SinkFamily:    kollectdevv1alpha1.SinkFamilySnapshot,
		ObjectPath:    "team-a/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	})
	if err == nil {
		t.Fatal("expected error for missing sink")
	}
}

func TestRunExportItems_exportsSnapshot(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-sink", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit, SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/inventory.git"}},
	}

	reg := NewRegistry()
	stub := &stubBackend{caps: cap.SnapshotStore()}
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "git-sink",
		SinkFamily:    kollectdevv1alpha1.SinkFamilySnapshot,
		ObjectPath:    "team-a/platform.json",
		Items: []collect.Item{
			{Name: "demo", Namespace: "apps", Kind: "Deployment", Version: "v1"},
		},
		Meta: export.Metadata{Cluster: "local", Generation: 1},
	})
	if err != nil {
		t.Fatalf("RunExportItems() = %v", err)
	}

	// Git sinks default to a human-readable YAML inventory document (ADR-0419). The stub backend
	// does not implement FileExporter, so the pipeline falls back to a single-document Export at
	// the resolved .yaml path with a YAML Items list (not the legacy JSON envelope).
	const wantPath = "inventory/team-a/platform.yaml"
	if stub.lastPath != wantPath {
		t.Fatalf("export path = %q, want %q", stub.lastPath, wantPath)
	}

	if len(stub.lastBody) == 0 || !strings.Contains(string(stub.lastBody), "kind: Deployment") {
		t.Fatalf("expected YAML inventory payload, got %q", stub.lastBody)
	}
}

func TestRunExportItems_skipsEmptySnapshotStream(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-sink", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectEventSinkSpec{
			Type: kollectdevv1alpha1.EventSinkTypeKafka,
			Kafka: &kollectdevv1alpha1.KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "inventory",
			},
		},
	}

	reg := NewRegistry()
	stub := &stubBackend{caps: cap.StreamEmitter()}
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "kafka-sink",
		SinkFamily:    kollectdevv1alpha1.SinkFamilyEvent,
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

	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-sink", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
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

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "close-err", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit, SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"}},
	}

	reg := NewRegistry()
	stub := &closableStubBackend{
		stubBackend: stubBackend{caps: cap.SnapshotStore()},
		closeErr:    errors.New("pool close failed"),
	}
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	t.Cleanup(func() { EvictBackendPool("team-a", "close-err") })

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "close-err",
		SinkFamily:    kollectdevv1alpha1.SinkFamilySnapshot,
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

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-close", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit, SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"}},
	}

	reg := NewRegistry()
	stub := &closableStubBackend{stubBackend: stubBackend{caps: cap.SnapshotStore()}}
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	t.Cleanup(func() { EvictBackendPool("team-a", "pool-close") })

	if err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "pool-close",
		SinkFamily:    kollectdevv1alpha1.SinkFamilySnapshot,
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

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "export-fail", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit, SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"}},
	}

	reg := NewRegistry()
	stub := &stubBackend{
		caps:      cap.SnapshotStore(),
		exportErr: errors.New("network down"),
	}
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	t.Cleanup(func() { EvictBackendPool("team-a", "export-fail") })

	err := RunExportItems(ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "export-fail",
		SinkFamily:    kollectdevv1alpha1.SinkFamilySnapshot,
		ObjectPath:    "team-a/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
		t.Fatalf("RunExportItems() = %v (%v), want transient export error", err, kollecterrors.ClassOf(err))
	}
}

func TestRunExportEnvelope_guards(t *testing.T) {
	t.Parallel()

	// nil registry → terminal error (line 125-127)
	err := RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:      context.Background(),
		Registry: nil,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("nil registry: want terminal error, got %v", err)
	}
	if !strings.Contains(err.Error(), "registry") {
		t.Fatalf("nil registry error = %q, want registry mention", err)
	}

	// empty sink type → terminal error (line 129-131)
	err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:      context.Background(),
		Registry: NewRegistry(),
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("empty type: want terminal error, got %v", err)
	}
}

func TestSnapshotStoreCapabilities(t *testing.T) {
	t.Parallel()

	caps := SnapshotStoreCapabilities()
	if caps.ObjectStore || caps.Stream || caps.SupportsDelete {
		t.Fatalf("SnapshotStoreCapabilities must leave all projection flags unset: %+v", caps)
	}
}

func TestRelationalStoreCapabilities(t *testing.T) {
	t.Parallel()

	caps := RelationalStoreCapabilities()
	if !caps.SupportsDelete {
		t.Fatal("RelationalStoreCapabilities must set SupportsDelete for stale-row pruning")
	}
	if caps.Stream || caps.ObjectStore {
		t.Fatalf("unexpected flags: %+v", caps)
	}
}

func TestClassifyExportFailure_terminalStaysTerminal(t *testing.T) {
	t.Parallel()

	// A terminal cause must be wrapped without being re-classified as retryable.
	terminal := kollecterrors.Terminal(errors.New("bad layout"))
	got := classifyExportFailure("git-sink", terminal)

	if !kollecterrors.IsTerminal(got) {
		t.Fatalf("classifyExportFailure(terminal) class = %s, want terminal", kollecterrors.ClassOf(got))
	}
	if !errors.Is(got, kollecterrors.ErrTerminal) {
		t.Fatal("classifyExportFailure(terminal) should satisfy errors.Is(ErrTerminal)")
	}
	if errors.Is(got, kollecterrors.ErrTransient) {
		t.Fatal("terminal failure must not become transient")
	}
	if !strings.Contains(got.Error(), "git-sink") {
		t.Fatalf("error should name the sink: %v", got)
	}
}

func TestClassifyExportFailure_nonTerminalBecomesTransient(t *testing.T) {
	t.Parallel()

	// A plain (unclassified) failure is retryable: wrap as transient.
	got := classifyExportFailure("git-sink", errors.New("network down"))

	if !errors.Is(got, kollecterrors.ErrTransient) {
		t.Fatalf("classifyExportFailure(plain) class = %s, want transient", kollecterrors.ClassOf(got))
	}
	if !strings.Contains(got.Error(), "git-sink") {
		t.Fatalf("error should name the sink: %v", got)
	}
}

func TestObjectStoreSnapshotCapabilities(t *testing.T) {
	t.Parallel()

	caps := ObjectStoreSnapshotCapabilities()
	if !caps.ObjectStore {
		t.Fatal("ObjectStoreSnapshotCapabilities must set ObjectStore")
	}
	if caps.SupportsDelete || caps.Stream {
		t.Fatalf("unexpected flags: %+v", caps)
	}
}

func TestStreamEmitterCapabilities(t *testing.T) {
	t.Parallel()

	caps := StreamEmitterCapabilities()
	if !caps.Stream {
		t.Fatal("StreamEmitterCapabilities must set Stream")
	}
	if caps.ObjectStore || caps.SupportsDelete {
		t.Fatalf("unexpected flags: %+v", caps)
	}
}
