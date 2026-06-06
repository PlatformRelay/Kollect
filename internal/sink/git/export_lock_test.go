// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestWithRepoExportLock_serializesConcurrentExports(t *testing.T) {
	t.Parallel()

	var concurrent int32
	var maxConcurrent int32

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_ = withRepoExportLock("https://git.example/repo", "main", func() error {
				cur := atomic.AddInt32(&concurrent, 1)
				for {
					peak := atomic.LoadInt32(&maxConcurrent)
					if cur > peak {
						if atomic.CompareAndSwapInt32(&maxConcurrent, peak, cur) {
							break
						}
						continue
					}
					break
				}

				atomic.AddInt32(&concurrent, -1)

				return nil
			})
		}()
	}

	wg.Wait()

	if maxConcurrent > 1 {
		t.Fatalf("max concurrent = %d, want 1", maxConcurrent)
	}
}
