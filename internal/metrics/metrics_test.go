// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	Register()

	InventoryItemsTotal.Set(0)
	if v := testutil.ToFloat64(InventoryItemsTotal); v != 0 {
		t.Fatalf("inventory items gauge: got %v", v)
	}

	SinkConnectionTestTotal.WithLabelValues("git", ResultSuccess).Inc()
	if v := testutil.ToFloat64(SinkConnectionTestTotal.WithLabelValues("git", ResultSuccess)); v < 1 {
		t.Fatalf("connection test counter: got %v", v)
	}
}
