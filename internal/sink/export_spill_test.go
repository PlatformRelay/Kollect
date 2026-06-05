// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"

	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink/cap"
)

func TestShouldExportForSpill(t *testing.T) {
	t.Parallel()

	under := export.SpillMandatoryBytes
	over := export.SpillMandatoryBytes + 1

	if !shouldExportForSpill(cap.SnapshotStore(), under) {
		t.Fatal("expected inline snapshot export under spill threshold")
	}
	if shouldExportForSpill(cap.SnapshotStore(), over) {
		t.Fatal("expected git snapshot to be skipped above spill threshold")
	}
	if !shouldExportForSpill(cap.ObjectStoreSnapshot(), over) {
		t.Fatal("expected object-store snapshot to export above spill threshold")
	}
}
