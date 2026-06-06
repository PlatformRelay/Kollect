// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"errors"
	"fmt"
	"strings"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"

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

	return err
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
