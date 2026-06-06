// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

// b9077924: hub export coalesce skips redundant exports within the configured interval.
func TestHubExportCoalesce_shouldSkipWithinInterval(t *testing.T) {
	t.Parallel()

	var c hubExportCoalesce
	now := time.Now()
	const interval = time.Minute

	c.record("cluster-a", "postgres", 3, "checksum-abc", now)
	if !c.shouldSkip("cluster-a", "postgres", 3, "checksum-abc", interval, now.Add(30*time.Second)) {
		t.Fatal("expected skip within interval for same generation and checksum")
	}
}

func TestHubExportCoalesce_shouldNotSkipAfterInterval(t *testing.T) {
	t.Parallel()

	var c hubExportCoalesce
	now := time.Now()
	const interval = time.Minute

	c.record("cluster-a", "postgres", 3, "checksum-abc", now)
	if c.shouldSkip("cluster-a", "postgres", 3, "checksum-abc", interval, now.Add(2*time.Minute)) {
		t.Fatal("expected export after interval elapsed")
	}
}

func TestHubExportCoalesce_shouldNotSkipOnGenerationChange(t *testing.T) {
	t.Parallel()

	var c hubExportCoalesce
	now := time.Now()

	c.record("cluster-a", "postgres", 3, "checksum-abc", now)
	if c.shouldSkip("cluster-a", "postgres", 4, "checksum-abc", time.Minute, now.Add(time.Second)) {
		t.Fatal("expected export when generation changes")
	}
}

func TestHubExportCoalesce_shouldNotSkipOnChecksumChange(t *testing.T) {
	t.Parallel()

	var c hubExportCoalesce
	now := time.Now()

	c.record("cluster-a", "postgres", 3, "checksum-abc", now)
	if c.shouldSkip("cluster-a", "postgres", 3, "checksum-def", time.Minute, now.Add(time.Second)) {
		t.Fatal("expected export when payload checksum changes")
	}
}

func TestHubExportCoalesce_zeroIntervalNeverSkips(t *testing.T) {
	t.Parallel()

	var c hubExportCoalesce
	now := time.Now()
	c.record("cluster-a", "postgres", 1, "x", now)

	if c.shouldSkip("cluster-a", "postgres", 1, "x", 0, now) {
		t.Fatal("zero interval must not skip exports")
	}
}

func TestChecksumForItems_stableFingerprint(t *testing.T) {
	t.Parallel()

	items := []collect.Item{{
		Namespace: "apps",
		Name:      "web",
		UID:       "uid-1",
		Version:   "v1",
		Kind:      "Deployment",
	}}
	report := SpokeReport{Generation: 7}

	sum1 := checksumForItems(items, report)
	sum2 := checksumForItems(items, report)
	if sum1 == "" || sum1 != sum2 {
		t.Fatalf("checksum = %q %q", sum1, sum2)
	}
}
