// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"
	"testing"
)

func assertInvalidResourceError(t *testing.T, err error, kind, name string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error")
	}

	want := fmt.Sprintf("%s %q is invalid:", kind, name)
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("err = %v, want prefix %q", err, want)
	}
}
