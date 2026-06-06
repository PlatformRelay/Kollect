// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"testing"
)

func TestValidateUniqueNonEmptyStrings(t *testing.T) {
	t.Parallel()

	if err := validateUniqueNonEmptyStrings([]string{"a", "b"}, "snapshotSinkRefs"); err != nil {
		t.Fatalf("unique values: %v", err)
	}

	if err := validateUniqueNonEmptyStrings([]string{""}, "snapshotSinkRefs"); err == nil {
		t.Fatal("expected empty string error")
	}

	if err := validateUniqueNonEmptyStrings([]string{"dup", "dup"}, "snapshotSinkRefs"); err == nil {
		t.Fatal("expected duplicate error")
	}
}
