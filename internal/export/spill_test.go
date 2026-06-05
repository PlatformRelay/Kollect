// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

import (
	"testing"

	"github.com/konih/kollect/internal/validation"
)

func TestAssessSpill(t *testing.T) {
	t.Parallel()

	maxBytes := validation.MaxExportBytesGlobal()

	cases := []struct {
		name        string
		size        int64
		wantWarn    bool
		wantSpill   bool
		wantExceeds bool
	}{
		{name: "under warn", size: SpillWarnBytes - 1},
		{name: "at warn", size: SpillWarnBytes, wantWarn: true},
		{name: "above spill", size: SpillMandatoryBytes + 1, wantWarn: true, wantSpill: true},
		{name: "at cap", size: maxBytes, wantWarn: true, wantSpill: true},
		{name: "over cap", size: maxBytes + 1, wantWarn: true, wantSpill: true, wantExceeds: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := AssessSpill(tc.size, maxBytes)
			if got.Warn != tc.wantWarn {
				t.Fatalf("Warn = %v, want %v", got.Warn, tc.wantWarn)
			}
			if got.RequiresSpill != tc.wantSpill {
				t.Fatalf("RequiresSpill = %v, want %v", got.RequiresSpill, tc.wantSpill)
			}
			if got.ExceedsCap != tc.wantExceeds {
				t.Fatalf("ExceedsCap = %v, want %v", got.ExceedsCap, tc.wantExceeds)
			}
		})
	}
}

func TestIsObjectStoreSinkType(t *testing.T) {
	t.Parallel()

	if !IsObjectStoreSinkType("s3") || !IsObjectStoreSinkType("gcs") {
		t.Fatal("expected s3/gcs to be object-store sinks")
	}
	if IsObjectStoreSinkType("git") || IsObjectStoreSinkType("postgres") {
		t.Fatal("expected non-object-store types to return false")
	}
}
