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
)
