// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
//
// Adapted from Argo CD Image Updater (Apache-2.0): ext/git/client.go (transient error signals)

package git

import (
	"errors"
	"fmt"
	"strings"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	kollecterrors "github.com/konih/kollect/internal/errors"
)

// ClassifyExportError maps git transport and push failures to reconcile classes.
func ClassifyExportError(err error) error {
	if err == nil {
		return nil
	}

	if kollecterrors.IsTerminal(err) {
		return err
	}

	msg := strings.ToLower(err.Error())

	if isAuthFailure(msg, err) {
		return kollecterrors.Terminal(fmt.Errorf("git auth failed: %w", err))
	}

	if strings.Contains(msg, "protected branch") ||
		strings.Contains(msg, "pre-receive hook declined") ||
		strings.Contains(msg, "remote rejected") && strings.Contains(msg, "403") {
		return kollecterrors.Terminal(fmt.Errorf("git push rejected: %w", err))
	}

	if isTransientTransportError(err) {
		return kollecterrors.Transient(fmt.Errorf("git transport: %w", err))
	}

	return err
}

// isTransientTransportError reports whether a git network operation should be retried.
func isTransientTransportError(err error) bool {
	if err == nil {
		return false
	}

	if utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) {
		return true
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporary failure") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests")
}

func isAuthFailure(msg string, err error) bool {
	if strings.Contains(msg, "authentication required") ||
		strings.Contains(msg, "invalid credentials") ||
		strings.Contains(msg, "authorization failed") ||
		strings.Contains(msg, "access denied") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "403 forbidden") {
		return true
	}

	if errors.Is(err, gittransport.ErrAuthenticationRequired) {
		return true
	}

	return false
}

func isNonFastForwardError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "non-fast-forward") ||
		strings.Contains(msg, "non fast forward") ||
		strings.Contains(msg, "failed to push some refs") && strings.Contains(msg, "updates were rejected")
}
