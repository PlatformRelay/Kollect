// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"sync"
	"testing"
)

// maxExportBytesGlobalTestMu serializes tests that mutate the process-global
// export cap. Production stays on atomic.Int64 for concurrent admission;
// tests must not overlap set→assert→restore windows (TEST-01).
var maxExportBytesGlobalTestMu sync.Mutex

// withMaxExportBytesGlobal installs bytes as the operator export cap for the
// duration of t, restoring the default on cleanup. Callers that also use
// t.Parallel must call t.Parallel before this helper so the lock is not held
// across the parallel barrier (which would deadlock other mutators).
func withMaxExportBytesGlobal(t *testing.T, bytes int64) {
	t.Helper()

	maxExportBytesGlobalTestMu.Lock()
	SetMaxExportBytesGlobal(bytes)
	t.Cleanup(func() {
		SetMaxExportBytesGlobal(defaultMaxExportBytesGlobal)
		maxExportBytesGlobalTestMu.Unlock()
	})
}

// lockMaxExportBytesGlobalForTest holds the mutation mutex for the whole test
// (e.g. concurrentAccess stress that mutates from many goroutines).
func lockMaxExportBytesGlobalForTest(t *testing.T) {
	t.Helper()

	maxExportBytesGlobalTestMu.Lock()
	t.Cleanup(func() {
		SetMaxExportBytesGlobal(defaultMaxExportBytesGlobal)
		maxExportBytesGlobalTestMu.Unlock()
	})
}
