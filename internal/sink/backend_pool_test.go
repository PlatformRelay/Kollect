// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type countingBackend struct {
	created atomic.Int32
}

func (c *countingBackend) Type() string               { return "counting" }
func (c *countingBackend) Capabilities() Capabilities { return SnapshotStoreCapabilities() }
func (c *countingBackend) Export(context.Context, []byte, string) error {
	return nil
}

func TestAcquireBackend_reusesPooledInstance(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "counting"},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()
	t.Cleanup(func() { EvictBackendPool("team-a", "pool") })

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "pool")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "pool")
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if b1 != b2 {
		t.Fatal("expected same pooled backend instance")
	}
}
