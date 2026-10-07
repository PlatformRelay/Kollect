// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

// The breaker pair drives the live RunExportEnvelope path — production calls
// exportThroughBreaker inside RunExportEnvelope (export.go). The same trip/reset
// semantics were previously asserted through the deleted items-level runner.
func TestRunExportEnvelope_circuitBreakerTripsAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	const (
		sinkNamespace = "cb-test-ns"
		sinkName      = "cb-test-sink"
	)

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: sinkNamespace},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             "stub",
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()

	reg := NewRegistry()
	stub := &stubBackend{
		caps:      cap.SnapshotStore(),
		exportErr: errors.New("network down"),
	}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	req := ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Client:        cl,
		Registry:      reg,
		SinkNamespace: sinkNamespace,
		SinkName:      sinkName,
		ObjectPath:    sinkNamespace + "/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "stub",
			Endpoint: "https://example.com/repo.git",
		},
	}

	for range circuitBreakerTripAt {
		_, err = RunExportEnvelope(req)
		if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
			t.Fatalf("RunExportEnvelope() before trip = %v (%v), want transient", err, kollecterrors.ClassOf(err))
		}
	}

	_, err = RunExportEnvelope(req)
	if err == nil {
		t.Fatal("expected circuit breaker open error")
	}
	if kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
		t.Fatalf("open breaker error class = %v, want transient", kollecterrors.ClassOf(err))
	}
}

func TestResetBreakersForTest_clearsOpenBreaker(t *testing.T) {
	t.Parallel()

	const (
		sinkNamespace = "cb-reset-ns"
		sinkName      = "cb-reset-sink"
	)

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: sinkNamespace},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             "stub",
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()

	reg := NewRegistry()
	stub := &stubBackend{
		caps:      cap.SnapshotStore(),
		exportErr: errors.New("network down"),
	}
	reg.Register("stub", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		return stub, nil
	})

	envelope, err := export.MarshalEnvelope(
		[]collect.Item{{Name: "demo"}},
		export.Metadata{Generation: 1},
	)
	if err != nil {
		t.Fatal(err)
	}

	req := ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Client:        cl,
		Registry:      reg,
		SinkNamespace: sinkNamespace,
		SinkName:      sinkName,
		ObjectPath:    sinkNamespace + "/inv.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "stub",
			Endpoint: "https://example.com/repo.git",
		},
	}

	for range circuitBreakerTripAt {
		_, _ = RunExportEnvelope(req)
	}
	if _, err := RunExportEnvelope(req); err == nil {
		t.Fatal("expected open breaker before reset")
	}

	ResetBreakersForTest()
	stub.exportErr = nil
	if _, err := RunExportEnvelope(req); err != nil {
		t.Fatalf("RunExportEnvelope after ResetBreakersForTest: %v", err)
	}
}
