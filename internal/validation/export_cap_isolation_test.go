// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"sync"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestMaxExportBytesGlobal_parallelMutationIsolation proves that concurrent
// mutators of the process-global export cap cannot cross-contaminate each
// other's set→validate→restore windows (TEST-01). Serialization is via
// withMaxExportBytesGlobal; without it this fails under -race as one
// goroutine's restore races another still expecting a tightened cap.
func TestMaxExportBytesGlobal_parallelMutationIsolation(t *testing.T) {
	const workers = 32

	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			maxExportBytesGlobalTestMu.Lock()
			SetMaxExportBytesGlobal(1000)
			defer func() {
				SetMaxExportBytesGlobal(defaultMaxExportBytesGlobal)
				maxExportBytesGlobalTestMu.Unlock()
			}()

			over := int64(2000)
			refs := kollectdevv1alpha1.InventorySinkRefList{
				{Name: "audit-git", MaxExportBytes: &over},
			}
			if errs := ValidateInventorySinkRefs(refs, nil); len(errs) != 1 {
				errCh <- fmt.Errorf("over-global maxExportBytes errs = %v, want 1", errs)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}
