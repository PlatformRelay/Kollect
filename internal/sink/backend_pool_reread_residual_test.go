// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Stage-B finding B1 (accepted): when the first metadata read may predate the pooled entry and
// the re-read fails, the entry is retired even if it is the newer one. The retire drains (no Close
// while held) and the next acquire rebuilds from the current value: one extra build, never a
// stale backend in use.
func TestAcquireBackend_failedReReadAfterStaleReadDrainsNewer(t *testing.T) {
	enableRotationPool(t)

	c := drainClient(t, "sentinel-token-5")
	reg := newDrainRegistry()
	b5, release5 := mustAcquire(t, c, c, reg.Registry, "stalefail", drainSpec())
	release5()

	// A reads the value-5 metadata and pauses; the Secret rotates and C stores B6, still held.
	reader := &rereadFailReader{Reader: c, read: make(chan struct{}), proceed: make(chan struct{}), err: context.DeadlineExceeded}
	failed := make(chan error, 1)
	go func() { failed <- acquireErr(c, reader, reg.Registry, "stalefail", drainSpec()) }()
	waitSignal(t, reader.read, "the stale metadata read")
	rotateTo(t, c, "sentinel-token-6")
	b6, release6 := mustAcquire(t, c, c, reg.Registry, "stalefail", drainSpec())
	if b6 == b5 {
		t.Fatal("setup: the rotation did not rebuild")
	}

	close(reader.proceed)
	select {
	case err := <-failed:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stale acquire: err %v, want the re-read's timeout", err)
		}
	case <-time.After(drainWait):
		t.Fatal("timed out waiting for the stale acquire")
	}

	if closes := b6.(*drainBackend).closes.Load(); closes != 0 {
		t.Fatalf("the newer backend was closed %d times while its holder runs, want 0", closes)
	}
	next, releaseNext := mustAcquire(t, c, c, reg.Registry, "stalefail", drainSpec())
	defer releaseNext()
	if got := next.(*drainBackend).token; got != "sentinel-token-6" {
		t.Fatalf("the acquire after the retire built from %q, want the current value", got)
	}
	release6()
	if next == b6 {
		if closes := b6.(*drainBackend).closes.Load(); closes != 0 {
			t.Fatalf("the shared newer backend closed %d times, want 0", closes)
		}

		return
	}
	if closes := b6.(*drainBackend).closes.Load(); closes != 1 {
		t.Fatalf("the retired newer backend closed %d times after its holder released, want 1", closes)
	}
}
