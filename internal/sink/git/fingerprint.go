// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"strings"
	"sync"
)

// exportFingerprintKey identifies a git export path for checksum coalescing (PERF-10).
func exportFingerprintKey(endpoint, branch, objectPath string) string {
	return strings.TrimSpace(endpoint) + "\x00" + strings.TrimSpace(branch) + "\x00" + strings.TrimSpace(objectPath)
}

type exportFingerprintTracker struct {
	mu   sync.Mutex
	last map[string]string
}

var fingerprintTracker exportFingerprintTracker

func (t *exportFingerprintTracker) shouldSkip(key, checksum string) bool {
	checksum = strings.TrimSpace(checksum)
	if checksum == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.last == nil {
		return false
	}

	return t.last[key] == checksum
}

func (t *exportFingerprintTracker) record(key, checksum string) {
	checksum = strings.TrimSpace(checksum)
	if checksum == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.last == nil {
		t.last = make(map[string]string)
	}

	t.last[key] = checksum
}
