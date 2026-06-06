// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"testing"
)

func TestTestConnection_invalidURL(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), Config{Endpoint: "://bad"}, Auth{})
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
}
