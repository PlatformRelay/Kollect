// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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

// voidCloserBackend exercises closeBackend's Close()-without-error interface branch.
type voidCloserBackend struct {
	stubBackend
	closed bool
}

func (v *voidCloserBackend) Close() { v.closed = true }

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

	if ExportErrorReason(kollecterrors.Terminal(ErrSpillRequired)) != "spill_required" {
		t.Fatal("spill-required error label")
	}
}

func TestRunExportEnvelope_guards(t *testing.T) {
	t.Parallel()

	// nil registry → terminal error (RunExportEnvelope, export.go:75-77)
	_, err := RunExportEnvelope(ExportEnvelopeRequest{
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

	// empty sink type → terminal error (RunExportEnvelope, export.go:79-81)
	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:      context.Background(),
		Registry: NewRegistry(),
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("empty type: want terminal error, got %v", err)
	}
}

func TestRunExportEnvelope_skipsEmptySnapshotStream(t *testing.T) {
	t.Parallel()

	envelope, err := export.MarshalEnvelope([]collect.Item{}, export.Metadata{Generation: 1})
	if err != nil {
		t.Fatal(err)
	}

	stub := &stubBackend{caps: cap.StreamEmitter()}
	reg := NewRegistry()
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})
	t.Cleanup(func() { EvictBackendPool("team-a", "skip-empty-stream") })

	paths, err := RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "skip-empty-stream",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec:      kollectdevv1alpha1.KollectSinkSpec{Type: "stub", Endpoint: "https://example.com/repo.git"},
	})
	if err != nil {
		t.Fatalf("RunExportEnvelope() error = %v", err)
	}
	if paths != nil {
		t.Fatalf("paths = %v, want nil for a skipped empty snapshot", paths)
	}
	if stub.lastBody != nil {
		t.Fatalf("backend received body %q, want no export", stub.lastBody)
	}
}

func mustStubEnvelopeRegistry(t *testing.T, stub Backend) *Registry {
	t.Helper()
	reg := NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})
	reg.Register(kollectdevv1alpha1.SinkTypePostgres, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext,
	) (Backend, error) {
		return stub, nil
	})
	reg.Register(kollectdevv1alpha1.SnapshotSinkTypeS3, func(
		_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext,
	) (Backend, error) {
		return stub, nil
	})
	t.Cleanup(func() {
		EvictBackendPool("team-a", "env-sink")
		EvictBackendPool("team-a", "pg-env")
		EvictBackendPool("team-a", "layout-fail")
		EvictBackendPool("team-a", "spill-skip")
		EvictBackendPool("team-a", "spill-store")
		EvictBackendPool("team-a", "void-close")
		EvictBackendPool("team-a", "bad-envelope")
	})

	return reg
}

// TestRunExportEnvelope_configFaultStaysTerminal pins the envelope side of
// the persisted-sink upgrade contract (ADR-0803, upgrading.md): an acquire
// error that construction already classified TERMINAL keeps its class through
// the envelope, so the sink's conditions report terminal instead of
// requeueing the deterministic fault as transient forever.
func TestRunExportEnvelope_configFaultStaysTerminal(t *testing.T) {
	t.Parallel()

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return nil, kollecterrors.Terminal(errors.New(`unsupported git engine "cli": the git sink exports through go-git only`))
	})

	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "cli-fault-sink",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec:      kollectdevv1alpha1.KollectSinkSpec{Type: "git"},
	})
	t.Cleanup(func() { EvictBackendPool("team-a", "cli-fault-sink") })

	if err == nil {
		t.Fatal("expected acquire-backend failure")
	}
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("envelope acquire fault class = %q, want terminal (the envelope must not demote a terminal construction fault to transient): %v", kollecterrors.ClassOf(err), err)
	}
	if !strings.Contains(err.Error(), "acquire backend") || !strings.Contains(err.Error(), "go-git") {
		t.Fatalf("error = %q, want the acquire-backend wrap and the go-git name", err)
	}
}

func TestRunExportEnvelope_acquireBackendFailure(t *testing.T) {
	t.Parallel()

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      NewRegistry(),
		SinkNamespace: "team-a",
		SinkName:      "unknown-type",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec:      kollectdevv1alpha1.KollectSinkSpec{Type: "no-such-backend"},
	})
	if err == nil {
		t.Fatal("expected acquire-backend failure for unknown type")
	}
	if !strings.Contains(err.Error(), "acquire backend") {
		t.Fatalf("error = %q, want acquire backend mention", err)
	}
}

func TestRunExportEnvelope_invalidEnvelopeItems(t *testing.T) {
	t.Parallel()

	stub := &stubBackend{caps: cap.SnapshotStore()}
	reg := mustStubEnvelopeRegistry(t, stub)

	_, err := RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "bad-envelope",
		ObjectPath:    "team-a/inv.json",
		Envelope:      []byte(`{"schemaVersion":"kollect.dev/v1alpha1","items":"not-an-array"}`),
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     kollectdevv1alpha1.SnapshotSinkTypeGit,
			Endpoint: "https://example.com/repo.git",
		},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("RunExportEnvelope() = %v, want terminal items decode error", err)
	}
	if stub.lastBody != nil {
		t.Fatal("invalid envelope must not reach Export")
	}
}

func TestRunExportEnvelope_relationalRemashalsNullItemsPreservingMeta(t *testing.T) {
	t.Parallel()

	// null items → ItemsJSONFromEnvelope yields "null"; SupportsDelete normalizes to
	// "[]" (length change), remashing while preserving generation/cluster/parts and
	// filling a zero ExportedAt so relational delete-reconcile still attributes the part.
	stub := &stubBackend{caps: cap.RelationalStore()}
	reg := mustStubEnvelopeRegistry(t, stub)

	envelope := []byte(`{
		"schemaVersion":"kollect.dev/v1alpha1",
		"generation":7,
		"cluster":"prod-west",
		"partIndex":1,
		"partTotal":2,
		"itemCount":0,
		"checksum":"deadbeef",
		"items":null
	}`)

	_, err := RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "pg-env",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				Table: "items",
			},
		},
	})
	if err != nil {
		t.Fatalf("RunExportEnvelope() = %v", err)
	}
	if len(stub.lastBody) == 0 {
		t.Fatal("relational remashal must still Export the empty snapshot")
	}

	var got collect.ExportEnvelope
	if err := json.Unmarshal(stub.lastBody, &got); err != nil {
		t.Fatalf("exported payload: %v", err)
	}
	if got.Generation != 7 || got.Cluster != "prod-west" || got.PartIndex != 1 || got.PartTotal != 2 {
		t.Fatalf("preserved meta = gen=%d cluster=%q part=%d/%d, want 7/prod-west/1/2",
			got.Generation, got.Cluster, got.PartIndex, got.PartTotal)
	}
	if got.ExportedAt == "" {
		t.Fatal("zero ExportedAt must be filled on remashal")
	}
	if got.ItemCount != 0 || len(got.Items) != 0 {
		t.Fatalf("want empty items after remashal, got count=%d len=%d", got.ItemCount, len(got.Items))
	}
}

func TestRunExportEnvelope_oversizedNonObjectStoreFailsLoudly(t *testing.T) {
	t.Parallel()

	stub := &stubBackend{caps: cap.SnapshotStore()}
	reg := mustStubEnvelopeRegistry(t, stub)

	blob := strings.Repeat("x", int(export.SpillMandatoryBytes)+64)
	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo", Attributes: map[string]any{"blob": blob}}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(envelope)) <= export.SpillMandatoryBytes {
		t.Fatalf("fixture envelope size %d must exceed spill threshold %d",
			len(envelope), export.SpillMandatoryBytes)
	}

	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "spill-skip",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     kollectdevv1alpha1.SnapshotSinkTypeGit,
			Endpoint: "https://example.com/repo.git",
		},
	})
	if err == nil {
		t.Fatal("RunExportEnvelope() = nil, want loud ErrSpillRequired (K-01: no silent drop)")
	}
	if !errors.Is(err, ErrSpillRequired) {
		t.Fatalf("error = %v, want errors.Is ErrSpillRequired", err)
	}
	if kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("error class = %q, want terminal", kollecterrors.ClassOf(err))
	}
	if stub.lastBody != nil {
		t.Fatal("non-object-store sink must not Export above spill threshold")
	}
}

func TestRunExportEnvelope_oversizedObjectStoreExports(t *testing.T) {
	t.Parallel()

	stub := &stubBackend{caps: cap.ObjectStoreSnapshot()}
	reg := mustStubEnvelopeRegistry(t, stub)

	blob := strings.Repeat("x", int(export.SpillMandatoryBytes)+64)
	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo", Attributes: map[string]any{"blob": blob}}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "spill-store",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     kollectdevv1alpha1.SnapshotSinkTypeS3,
			Endpoint: "https://s3.example.com/bucket",
		},
	})
	if err != nil {
		t.Fatalf("object-store export above spill threshold must succeed, got %v", err)
	}
	if stub.lastBody == nil {
		t.Fatal("object-store sink must Export above spill threshold")
	}
}

func TestRunExportEnvelope_resolveLayoutFailure(t *testing.T) {
	t.Parallel()

	stub := &stubBackend{caps: cap.SnapshotStore()}
	reg := mustStubEnvelopeRegistry(t, stub)

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{
			Namespace:  "team-a",
			Name:       "api",
			Kind:       "Deployment",
			Attributes: map[string]any{"image": "nginx"},
		}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "layout-fail",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     kollectdevv1alpha1.SnapshotSinkTypeGit,
			Endpoint: "https://example.com/repo.git",
			Layout: &kollectdevv1alpha1.LayoutSpec{
				Mode:    kollectdevv1alpha1.LayoutModePerResource,
				Content: kollectdevv1alpha1.LayoutContentManifest,
			},
		},
	})
	if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTerminal {
		t.Fatalf("RunExportEnvelope() = %v, want terminal layout resolve error", err)
	}
	if !strings.Contains(err.Error(), "resolve layout") {
		t.Fatalf("error = %q, want resolve layout mention", err)
	}
	if stub.lastBody != nil {
		t.Fatal("layout failure must not reach Export")
	}
}

func TestRunExportEnvelope_voidCloserReleasedWhenPoolDisabled(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() {
		EnableBackendPoolForTest()
		ResetBackendPoolForTest()
	})

	stub := &voidCloserBackend{stubBackend: stubBackend{caps: cap.SnapshotStore()}}
	reg := NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      reg,
		SinkNamespace: "team-a",
		SinkName:      "void-close",
		ObjectPath:    "team-a/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     kollectdevv1alpha1.SnapshotSinkTypeGit,
			Endpoint: "https://example.com/repo.git",
		},
	})
	if err != nil {
		t.Fatalf("RunExportEnvelope() = %v", err)
	}
	if !stub.closed {
		t.Fatal("pool-disabled release must invoke void Close()")
	}
	if len(stub.lastBody) == 0 {
		t.Fatal("expected successful Export before Close")
	}
}

func TestCloseBackend_voidCloser(t *testing.T) {
	t.Parallel()

	stub := &voidCloserBackend{stubBackend: stubBackend{caps: cap.SnapshotStore()}}
	if err := closeBackend(stub); err != nil {
		t.Fatalf("closeBackend() = %v", err)
	}
	if !stub.closed {
		t.Fatal("void Close() must be invoked")
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

// RED (retryable-vs-terminal routing): an export error that is ALREADY classified
// transient — e.g. a transient network failure surfaced by the backend — must stay
// retryable through classifyExportFailure and never be promoted to terminal. This
// locks the non-terminal branch for a pre-classified input (distinct from the plain
// case above, which enters the same branch from an unclassified error).
func TestClassifyExportFailure_transientNetworkStaysRetryable(t *testing.T) {
	t.Parallel()

	transient := kollecterrors.Transient(errors.New("dial tcp: connection reset by peer"))
	got := classifyExportFailure("git-sink", transient)

	if !errors.Is(got, kollecterrors.ErrTransient) {
		t.Fatalf("transient network failure must remain retryable, class = %s", kollecterrors.ClassOf(got))
	}
	if errors.Is(got, kollecterrors.ErrTerminal) {
		t.Fatal("a retryable network failure must not be promoted to terminal")
	}
	if !kollecterrors.IsTransient(got) {
		t.Fatalf("IsTransient(got) = false, want true: %v", got)
	}
	if !strings.Contains(got.Error(), "git-sink") {
		t.Fatalf("error should name the sink: %v", got)
	}
}
