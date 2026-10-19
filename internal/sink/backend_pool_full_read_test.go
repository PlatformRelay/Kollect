// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// SCR-2 and SCR-3 on the full read: a Secret that the metadata read saw but the full read finds
// missing retires the sink's pooled backend and fences a build in flight, like a missing Secret
// seen by the metadata read (stage-B finding A1).

// fullReadHookReader runs hook once, before the first full Secret read (not a metadata read).
type fullReadHookReader struct {
	client.Reader
	hook func()
	once sync.Once
}

func (r *fullReadHookReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, full := obj.(*corev1.Secret); full {
		r.once.Do(r.hook)
	}

	return r.Reader.Get(ctx, key, obj, opts...)
}

func waitNotFound(t *testing.T, ch <-chan error) {
	t.Helper()

	select {
	case err := <-ch:
		if err == nil || !strings.Contains(err.Error(), `secret "creds" not found`) {
			t.Fatalf("acquire: err %v, want the not-found error", err)
		}
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for the failing acquire")
	}
}

// A build in flight from the value read before the delete is not pooled once another acquire's
// full read found the Secret missing.
func TestAcquireBackend_fullReadNotFoundFencesBuildInFlight(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	slow := acquireAsync(c, c, reg.Registry, "fullread", drainSpec())
	waitSignal(t, started, "the slow build")

	// A's metadata read sees the Secret and misses; the Secret is deleted before A's full read.
	reading, deleted := make(chan struct{}), make(chan struct{})
	reader := &fullReadHookReader{Reader: c, hook: func() { close(reading); <-deleted }}
	failed := make(chan error, 1)
	go func() { failed <- acquireErr(c, reader, reg.Registry, "fullread", drainSpec()) }()
	waitSignal(t, reading, "the full read")
	deleteSecret(t, c, rotationSinkNS, "creds")
	close(deleted)
	waitNotFound(t, failed)

	close(proceed)
	x := waitAcquire(t, slow)
	if pooledEntryFor("fullread") != nil {
		t.Fatal("a build from before the delete was pooled after an acquire's full read found the Secret missing")
	}
	x.release()
	if closes := x.backend.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("unpooled build closed %d times after its release, want 1", closes)
	}
}

// An entry another acquire stored while A's full read was finding the Secret missing is retired
// by A; its holder keeps it open until it releases.
func TestAcquireBackend_fullReadNotFoundRetiresEntryStoredMeanwhile(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	slow := acquireAsync(c, c, reg.Registry, "fullread", drainSpec())
	waitSignal(t, started, "the slow build")

	// A misses; before A's full read, X stores its value-5 build and the Secret is deleted.
	reading, deleted := make(chan struct{}), make(chan struct{})
	reader := &fullReadHookReader{Reader: c, hook: func() { close(reading); <-deleted }}
	failed := make(chan error, 1)
	go func() { failed <- acquireErr(c, reader, reg.Registry, "fullread", drainSpec()) }()
	waitSignal(t, reading, "the full read")
	close(proceed)
	x := waitAcquire(t, slow)
	if pooledBackendFor("fullread") != x.backend {
		t.Fatal("setup: the slow build was not pooled")
	}
	deleteSecret(t, c, rotationSinkNS, "creds")
	close(deleted)
	waitNotFound(t, failed)

	if pooledEntryFor("fullread") != nil {
		t.Fatal("an acquire whose full read found the Secret missing left the pooled backend in place")
	}
	if closes := x.backend.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("retired backend closed %d times while its holder runs, want 0", closes)
	}
	x.release()
	if closes := x.backend.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("retired backend closed %d times after its holder released, want 1", closes)
	}
}

// credsFullReadFailReader fails the full read of the "creds" Secret with err, after signalling
// reading and waiting for proceed; every other read passes through.
type credsFullReadFailReader struct {
	client.Reader
	reading chan struct{}
	proceed chan struct{}
	err     error
	once    sync.Once
}

func (r *credsFullReadFailReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, full := obj.(*corev1.Secret); full && key.Name == "creds" {
		r.once.Do(func() { close(r.reading); <-r.proceed })

		return r.err
	}

	return r.Reader.Get(ctx, key, obj, opts...)
}

// A CA Secret that only the CA-rules read behind a failed credential full read finds missing
// also fences a build in flight.
func TestAcquireBackend_fullReadCAGoneFencesBuildInFlight(t *testing.T) {
	enableRotationPool(t)

	spec := drainSpec()
	spec.TLS = &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"}}
	c := rotationClient(t,
		rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-5")),
		rotationSecret(rotationSinkNS, "ca", "uid-ca", tokenData("ca.crt", "sentinel-key-ca")),
	)
	reg := newDrainRegistry()
	started, proceed := holdFirstBuild(reg)

	slow := acquireAsync(c, c, reg.Registry, "fullread-ca", spec)
	waitSignal(t, started, "the slow build")

	// A misses; its credential full read times out after the CA Secret is deleted.
	reader := &credsFullReadFailReader{Reader: c, reading: make(chan struct{}), proceed: make(chan struct{}),
		err: context.DeadlineExceeded}
	failed := make(chan error, 1)
	go func() { failed <- acquireErr(c, reader, reg.Registry, "fullread-ca", spec) }()
	waitSignal(t, reader.reading, "the credential full read")
	deleteSecret(t, c, rotationSinkNS, "ca")
	close(reader.proceed)
	select {
	case err := <-failed:
		if err == nil || !strings.Contains(err.Error(), `secret "ca" not found`) {
			t.Fatalf("acquire: err %v, want the CA rule's not-found error", err)
		}
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for the failing acquire")
	}

	close(proceed)
	x := waitAcquire(t, slow)
	defer x.release()
	if pooledEntryFor("fullread-ca") != nil {
		t.Fatal("a build from before the CA delete was pooled after the CA-rules read found the CA Secret missing")
	}
}
