// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"errors"

	"go.mongodb.org/mongo-driver/mongo"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// Sentinel errors for errors.Is classification of Export failure stages.
// Error() text matches the pre-existing fmt.Errorf prefixes byte-for-byte, so
// wrapping with these sentinels changes no observable error message.
var (
	ErrUpsertFailed = errors.New("mongodb upsert")
)

// MongoDB write error codes that retry cannot fix without a spec/data change.
const (
	mongoCodeDocumentValidation = 121
	mongoCodeDuplicateKey       = 11000
)

func classifyError(err error) error {
	if err == nil {
		return nil
	}

	if kollecterrors.IsTerminal(err) {
		return err
	}

	if isTerminalWrite(err) {
		return kollecterrors.Terminal(err)
	}

	return kollecterrors.Transient(err)
}

func isTerminalWrite(err error) bool {
	if err == nil {
		return false
	}

	if mongo.IsDuplicateKeyError(err) {
		return true
	}

	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, writeErr := range we.WriteErrors {
			switch writeErr.Code {
			case mongoCodeDocumentValidation, mongoCodeDuplicateKey:
				return true
			}
		}
	}

	return false
}
