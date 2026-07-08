// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"errors"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"

	kollecterrors "github.com/konih/kollect/internal/errors"
)

// Sentinel errors for errors.Is classification of Export failure stages.
// Error() text matches the pre-existing fmt.Errorf prefixes byte-for-byte, so
// wrapping with these sentinels changes no observable error message.
var (
	ErrDecodePayloadFailed  = errors.New("bigquery export: decode payload")
	ErrMergeUpsertFailed    = errors.New("bigquery merge upsert")
	ErrDeleteStaleFailed    = errors.New("bigquery delete stale")
	ErrEmulatorInsertFailed = errors.New("bigquery emulator insert")
)

func classifyError(err error) error {
	if err == nil {
		return nil
	}

	if isTerminal(err) {
		return kollecterrors.Terminal(err)
	}

	return kollecterrors.Transient(err)
}

func isDuplicateCreate(err error) bool {
	if err == nil {
		return false
	}

	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return false
	}

	if gerr.Code != 409 {
		return false
	}

	reasons := reasonsFromGoogleAPI(gerr)
	for _, reason := range reasons {
		if reason == "duplicate" {
			return true
		}
	}

	return false
}

func isTerminal(err error) bool {
	if err == nil {
		return false
	}

	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		switch gerr.Code {
		case 400, 401, 403, 404:
			return true
		}
	}

	for _, reason := range collectReasons(err) {
		switch reason {
		case "invalid", "invalidquery", "accessdenied", "notfound":
			return true
		case "ratelimitexceeded", "quotaexceeded", "backenderror", "internalerror":
			return false
		}
	}

	return false
}

func collectReasons(err error) []string {
	reasons := make([]string, 0, 2)

	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		reasons = append(reasons, reasonsFromGoogleAPI(gerr)...)
	}

	var bqerr *bigquery.Error
	if errors.As(err, &bqerr) {
		reason := strings.ToLower(strings.TrimSpace(bqerr.Reason))
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}

	return reasons
}

func reasonsFromGoogleAPI(err *googleapi.Error) []string {
	if err == nil {
		return nil
	}

	if len(err.Errors) == 0 {
		return nil
	}

	reasons := make([]string, 0, len(err.Errors))
	for _, item := range err.Errors {
		reason := strings.ToLower(strings.TrimSpace(item.Reason))
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}

	return reasons
}
