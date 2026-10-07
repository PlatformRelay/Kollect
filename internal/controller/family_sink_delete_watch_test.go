// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink"
)

// Tests for the family-sink delete hook (BEP-1): the DeleteFunc body the
// generic FamilySinkReconciler wires for each of the three sink kinds. The
// hook must forward the deleted object's identity to the pool's
// EvictBackendPoolForSink seam verbatim — the seam decides UID eviction and
// its namespace/name fallback — and a deleted sink's entry must never come
// back. Pooling here rides the production envelope path (RunExportEnvelope),
// the same one the reconcilers use.

type deleteWatchSpyBackend struct {
	closes atomic.Int32
}

func (b *deleteWatchSpyBackend) Type() string { return "counting" }
func (b *deleteWatchSpyBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}
func (b *deleteWatchSpyBackend) Export(context.Context, []byte, string) error { return nil }
func (b *deleteWatchSpyBackend) Close() error {
	b.closes.Add(1)

	return nil
}

// newDeleteWatchPool wires a fake client and a counting registry whose spy
// backend is rebuilt on every acquire, and returns an export closure over the
// production envelope path (RunExportEnvelope — the path the reconcilers use)
// plus the build counter: an entry is pooled exactly when two exports build
// once, and nothing is pooled when consecutive exports each build anew.
func newDeleteWatchPool(t *testing.T, spy *deleteWatchSpyBackend) (func(uid types.UID, ns, name string) error, *atomic.Int32) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	reg := sink.NewRegistry()
	builds := &atomic.Int32{}
	reg.Register("counting", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		builds.Add(1)

		return spy, nil
	})

	envelope, err := export.MarshalEnvelope([]collect.Item{}, export.Metadata{})
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}

	return func(uid types.UID, ns, name string) error {
		_, err := sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
			Ctx:           context.Background(),
			Client:        cl,
			Registry:      reg,
			SinkNamespace: ns,
			SinkName:      name,
			SinkUID:       uid,
			Envelope:      envelope,
			SinkSpec:      kollectdevv1alpha1.KollectSinkSpec{Type: "counting"},
		})

		return err
	}, builds
}

func assertPooled(t *testing.T, builds *atomic.Int32) {
	t.Helper()

	if builds.Load() != 1 {
		t.Fatalf("entry is not pooled: %d builds for two exports", builds.Load())
	}
}

func TestEvictBackendPoolOnSinkDelete_evictsPooledEntryByObjectIdentity(t *testing.T) {
	sink.EnableBackendPoolForTest()
	t.Cleanup(func() {
		sink.DisableBackendPoolForTest()
		sink.ResetBackendPoolForTest()
	})

	spy := &deleteWatchSpyBackend{}
	exportOnce, builds := newDeleteWatchPool(t, spy)

	const (
		uid  = types.UID("watch-uid-1")
		ns   = "team-a"
		name = "watch-sink"
	)

	if err := exportOnce(uid, ns, name); err != nil {
		t.Fatalf("pooling export: %v", err)
	}
	if err := exportOnce(uid, ns, name); err != nil {
		t.Fatalf("pool reuse export: %v", err)
	}
	assertPooled(t, builds)

	evictBackendPoolOnSinkDelete(&kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{UID: uid, Namespace: ns, Name: name},
	})

	if spy.closes.Load() != 1 {
		t.Fatalf("delete hook did not Close the pooled backend (closes=%d, want 1)", spy.closes.Load())
	}

	// No rebuild for the deleted sink's UID: the next two exports must each
	// build anew (nothing pooled) instead of reusing an entry.
	if err := exportOnce(uid, ns, name); err != nil {
		t.Fatalf("post-eviction export 1: %v", err)
	}
	if err := exportOnce(uid, ns, name); err != nil {
		t.Fatalf("post-eviction export 2: %v", err)
	}

	if builds.Load() != 3 {
		t.Fatalf("acquire after eviction re-pooled an entry for the deleted sink (builds=%d, want 3)", builds.Load())
	}

	if spy.closes.Load() != 3 {
		t.Fatalf("post-eviction builds were not discarded (closes=%d, want 3)", spy.closes.Load())
	}
}

func TestEvictBackendPoolOnSinkDelete_forwardsEmptyUIDForFallback(t *testing.T) {
	sink.EnableBackendPoolForTest()
	t.Cleanup(func() {
		sink.DisableBackendPoolForTest()
		sink.ResetBackendPoolForTest()
	})

	spy := &deleteWatchSpyBackend{}
	exportOnce, builds := newDeleteWatchPool(t, spy)

	const (
		ns   = "team-a"
		name = "fallback-watch"
	)

	if err := exportOnce("", ns, name); err != nil {
		t.Fatalf("pooling export: %v", err)
	}
	if err := exportOnce("", ns, name); err != nil {
		t.Fatalf("pool reuse export: %v", err)
	}
	assertPooled(t, builds)

	// A delete event whose object carries no UID (DeleteStateUnknown
	// tombstone): the hook must forward the empty UID verbatim so the seam's
	// namespace/name fallback evicts the entry.
	evictBackendPoolOnSinkDelete(&kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
	})

	if spy.closes.Load() != 1 {
		t.Fatalf("delete hook did not fall back to the namespace/name key (closes=%d, want 1)", spy.closes.Load())
	}

	if err := exportOnce("", ns, name); err != nil {
		t.Fatalf("post-eviction export: %v", err)
	}

	if builds.Load() != 2 {
		t.Fatalf("acquire after the empty-UID eviction re-pooled an entry (builds=%d, want 2)", builds.Load())
	}
}
