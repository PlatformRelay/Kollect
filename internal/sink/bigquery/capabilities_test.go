// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"testing"

	"github.com/konih/kollect/internal/sink/cap"
)

func TestBackendCapabilities(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	caps := b.Capabilities()
	if caps.Stream {
		t.Fatal("bigquery sink must not be a stream emitter")
	}
	if !caps.SupportsDelete {
		t.Fatal("bigquery sink must support delete reconciliation")
	}

	want := cap.RelationalStore()
	if caps != want {
		t.Fatalf("capabilities = %+v, want %+v", caps, want)
	}
}
