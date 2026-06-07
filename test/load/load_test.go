// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

//go:build load

package load_test

import (
	"os"
	"strconv"
	"testing"
)

const defaultLoadTestMaxObjects = 2000
const maxLoadTestMaxObjects = 10000

// TestSyntheticObjectCap verifies the load-test harness respects the ADR-0603 object cap.
// Full collection/export load scenarios are added here as the reconcile path matures.
func TestSyntheticObjectCap(t *testing.T) {
	if os.Getenv("KOLECT_LOAD_TEST") != "1" {
		t.Skip("set KOLECT_LOAD_TEST=1 to run load tests")
	}

	max := defaultLoadTestMaxObjects
	if v := os.Getenv("KOLECT_LOAD_TEST_MAX"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > maxLoadTestMaxObjects {
			t.Fatalf("KOLECT_LOAD_TEST_MAX must be 1..%d: %q", maxLoadTestMaxObjects, v)
		}
		max = n
	}

	t.Logf("load test harness ready (cap=%d objects)", max)
}
