// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"sync"
)

var exportLocks sync.Map // map[string]*sync.Mutex

// repoExportLockKey keys the lock on the mirror identity (clone URL + clone
// branch) rather than the push branch: every inventory of a branchMR sink
// shares one warm mirror worktree, so operations touching different push
// branches must still serialize against each other.
func repoExportLockKey(cloneURL, cloneBranch string) string {
	return cloneURL + "\x00" + cloneBranch
}

func withRepoExportLock(cloneURL, cloneBranch string, fn func() error) error {
	key := repoExportLockKey(cloneURL, cloneBranch)
	muIface, _ := exportLocks.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)

	mu.Lock()
	defer mu.Unlock()

	return fn()
}
