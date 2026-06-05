// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"fmt"
	"testing"
)

func TestTrackReconcileRecordsMetrics(t *testing.T) {
	t.Parallel()

	finish := trackReconcile("test-controller")
	finish(nil)
	finish(fmt.Errorf("boom"))
}
