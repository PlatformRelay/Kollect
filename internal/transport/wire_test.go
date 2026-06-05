// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport_test

import (
	"context"
	"testing"

	"github.com/konih/kollect/internal/transport"
)

func TestWireClusterIDContext(t *testing.T) {
	t.Parallel()

	ctx := transport.WithWireClusterID(context.Background(), "spoke-a")
	if got := transport.WireClusterID(ctx); got != "spoke-a" {
		t.Fatalf("cluster = %q", got)
	}

	if got := transport.WireClusterID(context.Background()); got != "" {
		t.Fatalf("unset cluster = %q", got)
	}

	if ctx := transport.WithWireClusterID(context.Background(), ""); ctx != context.Background() {
		t.Fatal("empty cluster id should not wrap context")
	}
}
