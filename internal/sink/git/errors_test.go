// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"fmt"
	"testing"

	kollecterrors "github.com/konih/kollect/internal/errors"
)

func TestClassifyExportError_authTerminal(t *testing.T) {
	t.Parallel()

	err := ClassifyExportError(fmt.Errorf("git push: authentication required"))
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("expected terminal, got %v", err)
	}
}

func TestClassifyExportError_transientPush(t *testing.T) {
	t.Parallel()

	err := ClassifyExportError(errors.New("connection reset"))
	if kollecterrors.IsTerminal(err) {
		t.Fatalf("expected transient/default, got terminal: %v", err)
	}
}
