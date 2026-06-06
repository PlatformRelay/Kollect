// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink/cap"
)

func TestRunExportItems_circuitBreakerTripsAfterRepeatedFailures(t *testing.T) {
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

	req := ExportItemsRequest{
		Ctx:           t.Context(),
		Client:        cl,
		Registry:      reg,
		SinkNamespace: sinkNamespace,
		SinkName:      sinkName,
		ObjectPath:    sinkNamespace + "/inv.json",
		Items:         []collect.Item{{Name: "demo"}},
	}

	for range circuitBreakerTripAt {
		err := RunExportItems(req)
		if err == nil || kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
			t.Fatalf("RunExportItems() before trip = %v (%v), want transient", err, kollecterrors.ClassOf(err))
		}
	}

	err := RunExportItems(req)
	if err == nil {
		t.Fatal("expected circuit breaker open error")
	}
	if kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
		t.Fatalf("open breaker error class = %v, want transient", kollecterrors.ClassOf(err))
	}
}
