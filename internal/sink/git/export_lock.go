// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"sync"
)

var exportLocks sync.Map // map[string]*sync.Mutex

func repoExportLockKey(endpoint, branch string) string {
	return endpoint + "\x00" + branch
}

func withRepoExportLock(endpoint, branch string, fn func() error) error {
	key := repoExportLockKey(endpoint, branch)
	muIface, _ := exportLocks.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)

	mu.Lock()
	defer mu.Unlock()

	return fn()
}
