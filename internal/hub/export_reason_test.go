// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"fmt"
	"testing"

	kollecterrors "github.com/konih/kollect/internal/errors"
)

func TestExportSinkErrorReason(t *testing.T) {
	t.Parallel()

	if got := exportSinkErrorReason(nil); got != "unknown" {
		t.Fatalf("nil = %q", got)
	}
	if got := exportSinkErrorReason(kollecterrors.Terminal(fmt.Errorf("bad"))); got != "terminal" {
		t.Fatalf("terminal = %q", got)
	}
	if got := exportSinkErrorReason(kollecterrors.Forbidden(fmt.Errorf("denied"))); got != "forbidden" {
		t.Fatalf("forbidden = %q", got)
	}
	if got := exportSinkErrorReason(fmt.Errorf("timeout")); got != "transient" {
		t.Fatalf("transient = %q", got)
	}
}
