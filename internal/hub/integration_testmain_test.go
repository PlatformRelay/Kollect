//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"os"
	"testing"

	"github.com/konih/kollect/internal/sink"
)

func TestMain(m *testing.M) {
	sink.DisableBackendPoolForTest()
	os.Exit(m.Run())
}
