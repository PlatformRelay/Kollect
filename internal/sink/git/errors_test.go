// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"testing"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
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
	if !kollecterrors.IsTransient(err) {
		t.Fatalf("expected transient, got %v", err)
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

func TestClassifyExportError_nilReturnsNil(t *testing.T) {
	t.Parallel()

	if err := ClassifyExportError(nil); err != nil {
		t.Fatalf("ClassifyExportError(nil) = %v, want nil", err)
	}
}

// TestClassifyExportError_unclassifiedPassesThrough ensures an error that matches
// none of the auth/rejection/transient signals is returned verbatim (not wrapped as a
// terminal error). Unknown git failures must stay retryable rather than being
// force-classified terminal — the default classification for a bare error is transient.
func TestClassifyExportError_unclassifiedPassesThrough(t *testing.T) {
	t.Parallel()

	orig := errors.New("something entirely unexpected happened")
	got := ClassifyExportError(orig)
	if !errors.Is(got, orig) {
		t.Fatalf("expected original error passed through, got %v", got)
	}
	if kollecterrors.IsTerminal(got) {
		t.Fatalf("unclassified git error must not be force-classified terminal: %v", got)
	}
}

// TestClassifyExportError_authRequiredSentinelTerminal covers the errors.Is branch
// in isAuthFailure: a wrapped go-git ErrAuthenticationRequired (whose message does
// not contain the auth keyword strings) must still classify as terminal auth failure.
func TestClassifyExportError_authRequiredSentinelTerminal(t *testing.T) {
	t.Parallel()

	err := ClassifyExportError(fmt.Errorf("push failed: %w", gittransport.ErrAuthenticationRequired))
	if !kollecterrors.IsTerminal(err) {
		t.Fatalf("expected terminal for wrapped ErrAuthenticationRequired, got %v", err)
	}
}

func TestIsTransientTransportError_nilAndEOF(t *testing.T) {
	t.Parallel()

	if isTransientTransportError(nil) {
		t.Fatal("nil error must not be transient")
	}

	// io.EOF is recognized by utilnet.IsProbableEOF as a probable connection
	// termination and must be treated as transient/retryable.
	if !isTransientTransportError(io.EOF) {
		t.Fatal("io.EOF should be classified transient")
	}
}

func TestIsTransientTransportError_connectionResetErrno(t *testing.T) {
	t.Parallel()

	// syscall.ECONNRESET is detected by utilnet.IsConnectionReset even when wrapped,
	// exercising the errno branch rather than the string-match fallback.
	err := fmt.Errorf("dial tcp: %w", syscall.ECONNRESET)
	if !isTransientTransportError(err) {
		t.Fatal("ECONNRESET should be classified transient")
	}
}

func TestIsNonFastForwardError(t *testing.T) {
	t.Parallel()

	if !isNonFastForwardError(errors.New("! [rejected] main -> main (non-fast-forward)")) {
		t.Fatal("expected non-fast-forward")
	}

	if isNonFastForwardError(errors.New("connection reset")) {
		t.Fatal("expected false for transient")
	}

	if isNonFastForwardError(nil) {
		t.Fatal("expected false for nil error")
	}

	if !isNonFastForwardError(errors.New("failed to push some refs: updates were rejected")) {
		t.Fatal("expected non-fast-forward for combined rejected-refs message")
	}
}

// Digits inside a path (a random temp dir) are not an HTTP status: a missing
// local remote must not classify as an auth failure.
func TestClassifyExportError_statusCodeNeedsTokenBoundary(t *testing.T) {
	t.Parallel()

	pathErr := errors.New("git clone: fatal: '/tmp/x3401029481/missing.git' does not appear to be a git repository")
	if kollecterrors.IsTerminal(ClassifyExportError(pathErr)) {
		t.Fatalf("digits in a path classified as terminal: %v", ClassifyExportError(pathErr))
	}

	for _, msg := range []string{"unexpected client error: 401", "HTTP 403 Forbidden"} {
		if !kollecterrors.IsTerminal(ClassifyExportError(errors.New(msg))) {
			t.Fatalf("%q must stay terminal", msg)
		}
	}

	if !kollecterrors.IsTransient(ClassifyExportError(errors.New("server said 503"))) {
		t.Fatal("standalone 503 must stay transient")
	}

	if strings.Contains(ClassifyExportError(errors.New("path /tmp/a15039b")).Error(), "git transport") {
		t.Fatal("503 inside a longer number must not be transient")
	}
}
