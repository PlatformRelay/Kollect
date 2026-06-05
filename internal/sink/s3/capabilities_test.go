// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"testing"

	"github.com/konih/kollect/internal/sink/cap"
)

func TestBackendCapabilities(t *testing.T) {
	t.Parallel()

	b := &Backend{}
	caps := b.Capabilities()
	if caps.Stream {
		t.Fatal("s3 sink must not be a stream emitter")
	}

	if caps.SupportsDelete {
		t.Fatal("s3 snapshot store must not use relational delete reconciliation")
	}

	if caps != cap.SnapshotStore() {
		t.Fatalf("capabilities = %+v", caps)
	}
}
