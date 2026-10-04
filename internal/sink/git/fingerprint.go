// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"crypto/sha256"
	"fmt"
	"sort"
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
	t.mu.Lock()
	defer t.mu.Unlock()
	if checksum == "" {
		delete(t.last, key)
		return
	}
	if t.last == nil {
		t.last = make(map[string]string)
	}
	t.last[key] = checksum
}

// ownedExportFingerprint uses a stable owner scope and records only the latest
// operation, so returning to an earlier snapshot cannot reuse a historical hit.
func ownedExportFingerprint(endpoint, branch, objectPath, checksum string, cfg Config, files []FileEntry) (string, string) {
	key := exportFingerprintKey(endpoint, branch, objectPath)
	if cfg.PruneOwner == "" {
		return key, checksum
	}
	key = exportFingerprintKey(endpoint, branch, "") + "\x00owner\x00" + cfg.PruneOwner
	checksum = strings.TrimSpace(checksum)
	if checksum == "" {
		return key, ""
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	keep := append([]string(nil), pruneKeepSet(cfg, paths)...)
	sort.Strings(paths)
	sort.Strings(keep)
	digest := sha256.Sum256([]byte(fmt.Sprintf("%q/%t/%q/%q", checksum, cfg.Prune, paths, keep)))
	return key, fmt.Sprintf("%x", digest)
}
