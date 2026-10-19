// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import "errors"

// Sentinel errors for errors.Is classification of Export failure stages.
// Error() text matches the pre-existing fmt.Errorf prefixes byte-for-byte, so
// wrapping with these sentinels changes no observable error message.
var (
	ErrDecodePayloadFailed           = errors.New("postgres export: decode payload")
	ErrBeginTxFailed                 = errors.New("postgres export: begin tx")
	ErrCommitTxFailed                = errors.New("postgres export: commit tx")
	ErrDeleteAllFailed               = errors.New("postgres delete all")
	ErrDeleteStaleFailed             = errors.New("postgres delete stale")
	ErrUpsertFailed                  = errors.New("postgres upsert")
	ErrBulkUpsertFailed              = errors.New("postgres bulk upsert")
	ErrBulkUpsertCreateStagingFailed = errors.New("postgres bulk upsert: create staging")
	ErrBulkUpsertCopyFailed          = errors.New("postgres bulk upsert: copy")
	ErrBulkUpsertMergeFailed         = errors.New("postgres bulk upsert: merge")

	// ErrTableNotFound marks a provisioning.mode=existing verification that found the
	// destination table absent. Distinct from a failed verification query, which must not be
	// reported as "table absent" (ADR-0416 §5).
	ErrTableNotFound = errors.New("postgres table not found")

	// ErrCARootConflict marks a postgres sink that configures a spec.tls CA while
	// another root certificate source (DSN, service file, PGSSLROOTCERT,
	// ~/.postgresql/root.crt) is also set.
	ErrCARootConflict = errors.New("spec.tls: a CA is configured, so another postgres root certificate source conflicts with it")

	// ErrCAWeakSSLMode marks a postgres sink that configures a spec.tls CA with an
	// explicit sslmode that never checks a certificate.
	ErrCAWeakSSLMode = errors.New("spec.tls: a CA is configured, but the postgres sslmode never checks it")
)
