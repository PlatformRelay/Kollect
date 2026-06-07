// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"testing"
)

type failingCloser struct{}

func (failingCloser) Type() string               { return "failing" }
func (failingCloser) Capabilities() Capabilities { return SnapshotStoreCapabilities() }
func (failingCloser) Export(_ context.Context, _ []byte, _ string) error {
	return nil
}
func (failingCloser) Close() error { return errors.New("close failed") }

func TestCloseBackendLogged_swallowsCloseError(t *testing.T) {
	t.Parallel()

	// Must not panic; errors are logged via controller-runtime (EC-P2-11).
	closeBackendLogged(failingCloser{}, "pool reset")
}

func TestCloseBackendLogged_nilBackend(t *testing.T) {
	t.Parallel()

	closeBackendLogged(nil, "explicit eviction")
}
