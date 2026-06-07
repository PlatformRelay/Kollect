// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"sync/atomic"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()
	t.Cleanup(func() { EvictBackendPool("team-a", "pool") })

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if b1 != b2 {
		t.Fatal("expected same pooled backend instance")
	}
}

func TestAcquireBackend_reusesPooledInstanceByUID(t *testing.T) {
	backendPoolDisabled.Store(false)
	t.Cleanup(func() { ResetBackendPoolForTest() })

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()
	const uid = types.UID("pool-uid-abc")
	t.Cleanup(func() { EvictBackendPoolByUID(uid) })

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "renamed-a", uid, spec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "renamed-b", uid, spec)
	if err != nil {
		t.Fatalf("second acquire after rename: %v", err)
	}
	release2()

	if b1 != b2 {
		t.Fatal("expected same pooled backend instance keyed by sink UID")
	}
}

func TestAcquireBackend_disabledPoolCreatesNewEachTime(t *testing.T) {
	DisableBackendPoolForTest()
	t.Cleanup(func() {
		backendPoolDisabled.Store(false)
		ResetBackendPoolForTest()
	})

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "counting"}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := NewRegistry()
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ BuildContext) (Backend, error) {
		cb := &countingBackend{}
		cb.created.Add(1)

		return cb, nil
	})

	ctx := context.Background()

	b1, release1, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	release1()

	b2, release2, err := acquireBackend(ctx, cl, reg, "team-a", "pool", "", spec)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release2()

	if b1 == b2 {
		t.Fatal("disabled pool must not reuse backend instances")
	}
}
