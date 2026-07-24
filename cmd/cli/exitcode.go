// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import "github.com/platformrelay/kollect/internal/pipeline"

// Process exit codes (ADR-0801).
const (
	// ExitSuccess: all targets collected and written.
	ExitSuccess = 0
	// ExitPartialFailure: at least one target was skipped (forbidden/transient/gvk-not-found)
	// or at least one per-object extraction failure was recorded, but at least one item was
	// still collected/exported (ADR-0801 / REL-02).
	ExitPartialFailure = 1
	// ExitFatalError: config invalid, cluster unreachable, or every target failed.
	ExitFatalError = 2
)

// mapContextResultsToExit aggregates per-context results into one process exit code:
// worst-of across all contexts (ExitFatalError > ExitPartialFailure > ExitSuccess). A
// single-context run (the default, no --context flag) reduces to the outcome of that one
// context.
func mapContextResultsToExit(results []pipeline.ContextResult) int {
	worst := ExitSuccess

	for _, r := range results {
		code := exitCodeForContext(r)
		if code > worst {
			worst = code
		}
	}

	return worst
}

func exitCodeForContext(r pipeline.ContextResult) int {
	degraded := len(r.Errs) > 0 || r.Skipped > 0

	switch {
	case r.Fatal != nil:
		return ExitFatalError
	case degraded && r.Exported == 0:
		return ExitFatalError
	case degraded:
		return ExitPartialFailure
	default:
		return ExitSuccess
	}
}
