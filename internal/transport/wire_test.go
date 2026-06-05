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
}
