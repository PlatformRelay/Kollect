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

// G2: protected branch and hook rejections are terminal (no infinite retry).
func TestClassifyExportError_protectedBranchTerminal(t *testing.T) {
	t.Parallel()

	err := ClassifyExportError(errors.New("remote: protected branch hook declined"))
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("expected terminal for protected branch: %v", err)
	}
}

func TestClassifyExportError_preReceiveHookTerminal(t *testing.T) {
	t.Parallel()

	err := ClassifyExportError(errors.New("pre-receive hook declined"))
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("expected terminal for pre-receive hook: %v", err)
	}
}

func TestClassifyExportError_remote403Terminal(t *testing.T) {
	t.Parallel()

	err := ClassifyExportError(errors.New("remote rejected: 403 forbidden"))
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("expected terminal for remote 403: %v", err)
	}
}

func TestClassifyExportError_preservesExistingTerminal(t *testing.T) {
	t.Parallel()

	term := kollecterrors.Terminal(errors.New("already terminal"))
	if !kollecterrors.IsTerminal(ClassifyExportError(term)) {
		t.Fatal("expected terminal wrapper preserved")
	}
}
