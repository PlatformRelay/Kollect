// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"errors"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

func TestClassifyError_documentValidationIsTerminal(t *testing.T) {
	t.Parallel()

	cause := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{
			Code:    mongoCodeDocumentValidation,
			Message: "Document failed validation",
		}},
	}
	wrapped := fmt.Errorf("%w: %w", ErrUpsertFailed, cause)

	got := classifyError(wrapped)
	if !errors.Is(got, ErrUpsertFailed) {
		t.Fatalf("classifyError() = %v, want ErrUpsertFailed", got)
	}
	if !kollecterrors.IsTerminal(got) {
		t.Fatalf("classifyError() = %v, want terminal", got)
	}
}

func TestClassifyError_duplicateKeyIsTerminal(t *testing.T) {
	t.Parallel()

	cause := mongo.WriteException{
		WriteErrors: []mongo.WriteError{{
			Code:    mongoCodeDuplicateKey,
			Message: "E11000 duplicate key error",
		}},
	}

	got := classifyError(fmt.Errorf("%w: %w", ErrUpsertFailed, cause))
	if !kollecterrors.IsTerminal(got) {
		t.Fatalf("classifyError() = %v, want terminal duplicate-key", got)
	}
}

func TestClassifyError_plainNetworkIsTransient(t *testing.T) {
	t.Parallel()

	got := classifyError(fmt.Errorf("%w: %w", ErrUpsertFailed, errors.New("connection reset")))
	if !kollecterrors.IsTransient(got) {
		t.Fatalf("classifyError() = %v, want transient", got)
	}
	if kollecterrors.IsTerminal(got) {
		t.Fatalf("classifyError() must not mark plain network failure terminal: %v", got)
	}
}

func TestClassifyError_nil(t *testing.T) {
	t.Parallel()

	if got := classifyError(nil); got != nil {
		t.Fatalf("classifyError(nil) = %v, want nil", got)
	}
}
