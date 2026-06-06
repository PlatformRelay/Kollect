// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"fmt"
	"sync"
	"time"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
)

type hubExportCoalesce struct {
	mu     sync.Mutex
	states map[string]*hubExportState
}

type hubExportState struct {
	lastExport   time.Time
	lastChecksum string
	generation   int64
}

func (c *hubExportCoalesce) key(cluster, sink string) string {
	return cluster + "\x00" + sink
}

func (c *hubExportCoalesce) shouldSkip(
	cluster, sink string,
	generation int64,
	checksum string,
	interval time.Duration,
	now time.Time,
) bool {
	if interval <= 0 {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.states == nil {
		c.states = make(map[string]*hubExportState)
	}

	state := c.states[c.key(cluster, sink)]
	if state == nil || state.generation != generation || state.lastChecksum != checksum {
		return false
	}

	if state.lastExport.IsZero() {
		return false
	}

	return now.Sub(state.lastExport) < interval
}

func (c *hubExportCoalesce) record(cluster, sink string, generation int64, checksum string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.states == nil {
		c.states = make(map[string]*hubExportState)
	}

	k := c.key(cluster, sink)
	state := c.states[k]
	if state == nil {
		state = &hubExportState{}
		c.states[k] = state
	}

	state.lastExport = now
	state.lastChecksum = checksum
	state.generation = generation
}

func checksumForItems(items []collect.Item, report SpokeReport) string {
	sum, err := export.ItemsFingerprint(items)
	if err != nil {
		return fmt.Sprintf("gen-%d", report.Generation)
	}

	return sum
}
