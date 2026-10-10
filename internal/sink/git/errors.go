// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
//
// Adapted from Argo CD Image Updater (Apache-2.0): ext/git/client.go (transient error signals)

package git

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
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
		strings.Contains(msg, "remote rejected") && hasAnyStatus(httpStatusCodes(err, msg), isAuthStatus) {
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
		hasAnyStatus(httpStatusCodes(err, msg), isRetryableStatus) ||
		strings.Contains(msg, "too many requests")
}

var (
	// statusPhraseRE matches a status code only after a phrase that introduces
	// one ("status code: 502", "returned error: 403", "HTTP 401", "code=401").
	statusPhraseRE = regexp.MustCompile(`(?:returned error:|status code:?|status:|code[=:]|http/?[0-9.]*|error:)\s*\(?([0-9]{3})(?:[^0-9]|$)`)
	// statusPunctRE matches a parenthesised or colon-terminated code: "(429)", "401:".
	statusPunctRE = regexp.MustCompile(`(?:^|[\s(])([0-9]{3})[):]`)
	// statusTextRE matches a code followed by its reason phrase: "403 forbidden".
	statusTextRE = regexp.MustCompile(`(?:^|\s)([0-9]{3}) (?:unauthorized|forbidden|too many requests|service unavailable|bad gateway|gateway timeout|internal server error)`)
)

// httpStatusCodes extracts the HTTP status codes an export error carries. It
// prefers go-git's typed *http.Err (its Response.StatusCode is authoritative)
// and otherwise reads only status phrases, never bare digit runs: a code-like
// substring in a URL, hostname or temp/mirror directory name (team401,
// git-503.example.com, a hex mirror dir) is not a status.
func httpStatusCodes(err error, msg string) []int {
	var codes []int

	var httpErr *githttp.Err
	if errors.As(err, &httpErr) && httpErr.Response != nil {
		codes = append(codes, httpErr.StatusCode())
	}

	// plumbing.UnexpectedError does not implement Unwrap.
	var unexpected *plumbing.UnexpectedError
	if errors.As(err, &unexpected) && unexpected.Err != nil {
		if errors.As(unexpected.Err, &httpErr) && httpErr.Response != nil {
			codes = append(codes, httpErr.StatusCode())
		}
	}

	for _, re := range []*regexp.Regexp{statusPhraseRE, statusPunctRE, statusTextRE} {
		for _, m := range re.FindAllStringSubmatch(msg, -1) {
			if n, convErr := strconv.Atoi(m[1]); convErr == nil {
				codes = append(codes, n)
			}
		}
	}

	return codes
}

func hasAnyStatus(codes []int, match func(int) bool) bool {
	for _, c := range codes {
		if match(c) {
			return true
		}
	}

	return false
}

func isAuthStatus(c int) bool { return c == 401 || c == 403 }

func isRetryableStatus(c int) bool { return c == 429 || c >= 500 && c <= 599 }

func isAuthFailure(msg string, err error) bool {
	if strings.Contains(msg, "authentication required") ||
		strings.Contains(msg, "invalid credentials") ||
		strings.Contains(msg, "authorization failed") ||
		strings.Contains(msg, "access denied") ||
		hasAnyStatus(httpStatusCodes(err, msg), isAuthStatus) {
		return true
	}

	if errors.Is(err, gittransport.ErrAuthenticationRequired) ||
		errors.Is(err, gittransport.ErrAuthorizationFailed) {
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
