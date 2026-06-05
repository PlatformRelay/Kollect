// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package export

// Spill thresholds per ADR-0103 and Q2 (warn at 1 MiB; mandatory above; hard cap ~1.5 MiB).
const (
	SpillWarnBytes      int64 = 1 << 20 // 1 MiB
	SpillMandatoryBytes int64 = 1 << 20 // spill required strictly above this size
)

// SpillAssessment captures export payload size policy outcomes.
type SpillAssessment struct {
	Size          int64
	Cap           int64
	Warn          bool
	RequiresSpill bool
	ExceedsCap    bool
}

// AssessSpill evaluates payload size against warn, spill-mandatory, and hard-cap thresholds.
func AssessSpill(size, maxBytes int64) SpillAssessment {
	a := SpillAssessment{Size: size, Cap: maxBytes}
	if maxBytes > 0 && size > maxBytes {
		a.ExceedsCap = true
	}
	if size >= SpillWarnBytes {
		a.Warn = true
	}
	if size > SpillMandatoryBytes {
		a.RequiresSpill = true
	}

	return a
}

// IsObjectStoreSinkType reports whether the sink type can receive mandatory spill exports.
func IsObjectStoreSinkType(sinkType string) bool {
	return sinkType == "s3" || sinkType == "gcs"
}
