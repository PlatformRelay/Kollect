// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package envtestassets resolves the envtest control-plane binaries that match the running host.
//
// setup-envtest lays its downloads out as <bin/k8s>/<k8s-version>-<os>-<arch>, and a checkout can
// easily end up holding several of them at once — a bin/ shared between machines, a stale download,
// or a directory left behind by an earlier --bin-dir. The kubebuilder scaffold picks the first
// directory it finds, which on an Apple-silicon host hands envtest linux-amd64 binaries and fails
// every suite in BeforeSuite with "exec format error". Selecting by host OS/arch instead keeps a
// bare `go test ./internal/...` usable off linux without changing what CI resolves.
package envtestassets

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EnvVar is the environment variable `make test` and hack/coverage.sh export, and the one
// controller-runtime consults before an envtest.Environment's BinaryAssetsDirectory.
const EnvVar = "KUBEBUILDER_ASSETS"

// Dir returns the envtest asset directory built for goos/goarch, or "" when there is none.
//
// basePaths are searched in order and the first one holding a match wins; a base path that cannot
// be read is skipped. Only directories whose name ends in -<goos>-<goarch> are considered, which
// also rules out non-asset directories. Among the matches the highest version wins, compared field
// by numeric field so that 1.36.10 outranks 1.36.2 — plain lexical order gets that backwards.
//
// The result is absolute so it survives a caller changing directory. An empty result means callers
// should leave BinaryAssetsDirectory unset and let controller-runtime fall back to EnvVar or to its
// own default location.
func Dir(basePaths []string, goos, goarch string) string {
	suffix := "-" + goos + "-" + goarch

	for _, basePath := range basePaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}

		best, bestVersion := "", ""

		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || !strings.HasSuffix(name, suffix) {
				continue
			}

			version := strings.TrimSuffix(name, suffix)
			if best == "" || newer(version, bestVersion) {
				best, bestVersion = name, version
			}
		}

		if best != "" {
			return abs(filepath.Join(basePath, best))
		}
	}

	return ""
}

// Resolve returns EnvVar when it is set and Dir(basePaths, goos, goarch) otherwise.
//
// EnvVar wins because that is the order controller-runtime itself resolves binaries in, so a
// harness that exports it — `make test`, hack/coverage.sh, CI — stays authoritative over whatever
// else happens to sit in bin/k8s.
func Resolve(basePaths []string, goos, goarch string) string {
	if assets := os.Getenv(EnvVar); assets != "" {
		return abs(assets)
	}

	return Dir(basePaths, goos, goarch)
}

// newer reports whether version a ranks above version b. Dot-separated fields are compared in
// order, numerically where both parse and lexically where either does not, and a version that
// extends another as a prefix ranks above it.
func newer(a, b string) bool {
	fieldsA := strings.Split(a, ".")
	fieldsB := strings.Split(b, ".")

	for i := 0; i < len(fieldsA) && i < len(fieldsB); i++ {
		if fieldsA[i] == fieldsB[i] {
			continue
		}

		numA, errA := strconv.Atoi(fieldsA[i])
		numB, errB := strconv.Atoi(fieldsB[i])

		if errA != nil || errB != nil {
			return fieldsA[i] > fieldsB[i]
		}

		return numA > numB
	}

	return len(fieldsA) > len(fieldsB)
}

// abs resolves path against the working directory, falling back to path when that is unavailable.
func abs(path string) string {
	if resolved, err := filepath.Abs(path); err == nil {
		return resolved
	}

	return path
}
