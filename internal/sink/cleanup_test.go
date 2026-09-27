// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"errors"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// A cleanup backend error keeps its class: terminal stays terminal (operator
// action) and everything else stays transient so the finalizer retries.
func TestClassifyCleanupFailure_PreservesErrorClass(t *testing.T) {
	t.Parallel()

	terminal := classifyCleanupFailure("s3-prod", kollecterrors.Terminal(errors.New("bad credentials")))
	if !kollecterrors.IsTerminal(terminal) {
		t.Fatalf("terminal cleanup failure lost its class: %v", terminal)
	}

	transient := classifyCleanupFailure("s3-prod", errors.New("connection reset"))
	if !kollecterrors.IsTransient(transient) {
		t.Fatalf("unclassified cleanup failure must stay transient: %v", transient)
	}
}

// A cleanup attempt without a resolved registry or sink spec is a terminal
// configuration error, never a silent clean tombstone.
func TestRunCleanupExport_MissingRegistryOrSpecIsTerminal(t *testing.T) {
	t.Parallel()

	if _, err := RunCleanupExport(CleanupExportRequest{SinkName: "s3-prod"}); !kollecterrors.IsTerminal(err) {
		t.Fatalf("missing registry: err = %v, want terminal", err)
	}

	_, err := RunCleanupExport(CleanupExportRequest{
		Registry: &Registry{},
		SinkName: "s3-prod",
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{},
	})
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("missing sink spec: err = %v, want terminal", err)
	}
}
