// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

const (
	pruneMetadataDir      = ".kollect-prune"
	maxPruneMetadataBytes = 16 << 20
	maxPruneOwners        = 1024
	maxPrunePaths         = 100000
)

// Records are committed with the exported files. A missing record is migration,
// not evidence that other files in the repository belong to this inventory.
type pruneRecord struct {
	Version int      `json:"version"`
	Owner   string   `json:"owner"`
	Paths   []string `json:"paths"`
}

type ownedPrunePlan struct {
	recordPath string
	data       []byte
	remove     []string
}

func pruneRecordPath(owner string) string {
	return fmt.Sprintf("%s/%x.json", pruneMetadataDir, sha256.Sum256([]byte(owner)))
}

// validatePrunePath is deliberately stricter than path cleaning: an ownership
// record must contain canonical file paths, never a normalized traversal.
func validatePrunePath(p string) error {
	if len(p) > 4096 || p == "" || p != strings.TrimSpace(p) || path.IsAbs(p) || path.Clean(p) != p || strings.ContainsAny(p, "\\\x00\r\n") {
		return invalidPrune("invalid prune path %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "." || part == ".." || strings.EqualFold(part, ".git") || strings.EqualFold(part, pruneMetadataDir) {
			return invalidPrune("reserved or unsafe prune path %q", p)
		}
	}
	return nil
}

// checkPruneFile checks every existing component without following symlinks.
// The clone is private and export is serialized by the repository lock.
func checkPruneFile(fs billy.Filesystem, p string) error {
	parts := strings.Split(p, "/")
	for i := range parts {
		component := strings.Join(parts[:i+1], "/")
		info, err := fs.Lstat(component)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("prune lstat %q: %w", component, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return invalidPrune("prune path %q is a symlink", component)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return invalidPrune("prune parent %q is not a directory", component)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return invalidPrune("prune path %q is not a regular file", component)
		}
	}
	return nil
}

func readPruneRecord(fs billy.Filesystem, name string, remaining int64) (pruneRecord, int64, error) {
	var record pruneRecord
	p := path.Join(pruneMetadataDir, name)
	if err := checkPruneFile(fs, p); err != nil {
		return record, 0, err
	}
	f, err := fs.Open(p)
	if err != nil {
		return record, 0, fmt.Errorf("open prune record: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(f, remaining+1))
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return record, 0, fmt.Errorf("read prune record: %w", err)
	}
	if int64(len(data)) > remaining {
		return record, 0, invalidPrune("prune metadata exceeds %d bytes", maxPruneMetadataBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, 0, invalidPrune("decode prune record %q: %w", name, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return record, 0, invalidPrune("prune record %q has trailing data", name)
	}
	if record.Version != 1 || record.Owner == "" || len(record.Owner) > 4096 || pruneRecordPath(record.Owner) != p || len(record.Paths) > maxPrunePaths {
		return record, 0, invalidPrune("invalid prune record %q", name)
	}
	return record, int64(len(data)), nil
}

// loadPruneRecords bounds total metadata, not just each record, and rejects
// conflicting claims. Another owner's file cannot be adopted or deleted.
func loadPruneRecords(fs billy.Filesystem) (map[string]string, map[string]pruneRecord, int64, error) {
	owners := make(map[string]string)
	records := make(map[string]pruneRecord)
	info, err := fs.Lstat(pruneMetadataDir)
	if os.IsNotExist(err) {
		return owners, records, 0, nil
	}
	if err != nil {
		return nil, nil, 0, fmt.Errorf("prune metadata stat: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, 0, invalidPrune("prune metadata must be a real directory")
	}
	entries, err := fs.ReadDir(pruneMetadataDir)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("prune metadata list: %w", err)
	}
	if len(entries) > maxPruneOwners {
		return nil, nil, 0, invalidPrune("prune metadata exceeds %d owners", maxPruneOwners)
	}
	var total int64
	for _, entry := range entries {
		record, size, err := readPruneRecord(fs, entry.Name(), maxPruneMetadataBytes-total)
		if err != nil {
			return nil, nil, 0, err
		}
		total += size
		for _, p := range record.Paths {
			if err := validatePrunePath(p); err != nil {
				return nil, nil, 0, err
			}
			if _, exists := owners[p]; exists {
				return nil, nil, 0, invalidPrune("duplicate ownership of %q", p)
			}
			owners[p] = record.Owner
		}
		records[record.Owner] = record
	}
	return owners, records, total, nil
}

// prepareOwnedPrune validates the complete operation before the engines write or
// delete anything. Suppressed multipart parts validate ownership but do not
// advance the committed record until the final union is available.
func prepareOwnedPrune(fs billy.Filesystem, cfg Config, written []string) (*ownedPrunePlan, error) {
	if cfg.PruneOwner == "" {
		return nil, nil
	}
	if len(cfg.PruneOwner) > 4096 {
		return nil, invalidPrune("prune owner exceeds 4096 bytes")
	}
	owners, records, total, err := loadPruneRecords(fs)
	if err != nil {
		return nil, err
	}
	current, err := validateOwnedPrunePaths(fs, cfg, written, owners)
	if err != nil {
		return nil, err
	}
	if !cfg.Prune {
		return nil, nil
	}
	plan := &ownedPrunePlan{recordPath: pruneRecordPath(cfg.PruneOwner)}
	for _, p := range records[cfg.PruneOwner].Paths {
		if pathErr := checkPruneFile(fs, p); pathErr != nil {
			return nil, pathErr
		}
		if _, ok := current[p]; !ok {
			plan.remove = append(plan.remove, p)
		}
	}
	paths := make([]string, 0, len(current))
	for p := range current {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	sort.Strings(plan.remove)
	record := pruneRecord{Version: 1, Owner: cfg.PruneOwner, Paths: paths}
	plan.data, err = json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode prune record: %w", err)
	}
	plan.data = append(plan.data, '\n')
	records[cfg.PruneOwner] = record
	if len(records) > maxPruneOwners {
		return nil, invalidPrune("prune metadata exceeds %d owners", maxPruneOwners)
	}
	// Bound the next snapshot using actual on-disk bytes, including whitespace
	// in other records. Re-encoding them would underestimate their stored size.
	if _, exists := records[cfg.PruneOwner]; exists {
		info, statErr := fs.Lstat(plan.recordPath)
		if statErr != nil && !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("prune record stat: %w", statErr)
		}
		if statErr == nil {
			total -= info.Size()
		}
	}
	total += int64(len(plan.data))
	if total > maxPruneMetadataBytes {
		return nil, invalidPrune("prune metadata exceeds %d bytes", maxPruneMetadataBytes)
	}

	return plan, nil
}

func (p *ownedPrunePlan) apply(fs billy.Filesystem) error {
	if p == nil {
		return nil
	}
	for _, name := range p.remove {
		if err := fs.Remove(name); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune remove %q: %w", name, err)
		}
	}
	if err := fs.MkdirAll(pruneMetadataDir, 0o750); err != nil {
		return fmt.Errorf("create prune metadata: %w", err)
	}
	if err := util.WriteFile(fs, p.recordPath, p.data, 0o600); err != nil {
		return fmt.Errorf("write prune metadata: %w", err)
	}
	return nil
}

func invalidPrune(format string, args ...any) error {
	return kollecterrors.Terminal(fmt.Errorf(format, args...))
}

func validateOwnedPrunePaths(fs billy.Filesystem, cfg Config, written []string, owners map[string]string) (map[string]struct{}, error) {
	keep := pruneKeepSet(cfg, written)
	if len(keep) > maxPrunePaths {
		return nil, invalidPrune("prune inventory exceeds %d paths", maxPrunePaths)
	}
	current := pathSet(keep)
	// Written paths must be present in an explicit union; otherwise the ownership
	// record would silently omit data just written by this export.
	for _, p := range written {
		if _, ok := current[p]; !ok {
			return nil, invalidPrune("written path %q missing from prune keep set", p)
		}
	}
	for p := range current {
		if err := validatePrunePath(p); err != nil {
			return nil, err
		}
		if owner, ok := owners[p]; ok && owner != cfg.PruneOwner {
			return nil, invalidPrune("prune path %q belongs to another inventory", p)
		}
		if pathErr := checkPruneFile(fs, p); pathErr != nil {
			return nil, pathErr
		}
	}
	return current, nil
}
