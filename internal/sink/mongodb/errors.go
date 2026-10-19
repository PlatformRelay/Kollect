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

	// ErrCAFileConflict marks a mongodb sink that configures a spec.tls CA while its
	// URI names a CA file of its own (tlsCAFile, sslCertificateAuthorityFile).
	ErrCAFileConflict = errors.New("spec.tls: a CA is configured, so the mongodb URI's own CA file conflicts with it")

	// ErrCATLSOff marks a mongodb sink that configures a spec.tls CA on a URI that
	// does not use TLS, so the CA would never be checked.
	ErrCATLSOff = errors.New("spec.tls: a CA is configured, but the mongodb URI does not use TLS")

	// ErrCAInsecure marks a mongodb sink that configures a spec.tls CA on a URI that
	// turns certificate verification off.
	ErrCAInsecure = errors.New("spec.tls: a CA is configured, but the mongodb URI turns certificate verification off")

	// ErrCAURIInvalid marks a mongodb URI whose TLS options cannot be read when a
	// spec.tls CA is configured.
	ErrCAURIInvalid = errors.New("spec.tls: a CA is configured, but the mongodb URI's options cannot be read")
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
