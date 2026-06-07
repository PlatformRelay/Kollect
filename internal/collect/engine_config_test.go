// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import "testing"

func TestNormalizeEngineConfig_defaults(t *testing.T) {
	t.Parallel()

	cfg := normalizeEngineConfig(EngineConfig{})
	if cfg.DispatchWorkers != defaultDispatchWorkers {
		t.Fatalf("DispatchWorkers = %d, want %d", cfg.DispatchWorkers, defaultDispatchWorkers)
	}
	if cfg.DispatchQueueSize != defaultDispatchQueueSize {
		t.Fatalf("DispatchQueueSize = %d, want %d", cfg.DispatchQueueSize, defaultDispatchQueueSize)
	}
}

func TestNormalizeEngineConfig_custom(t *testing.T) {
	t.Parallel()

	cfg := normalizeEngineConfig(EngineConfig{DispatchWorkers: 8, DispatchQueueSize: 1024})
	if cfg.DispatchWorkers != 8 || cfg.DispatchQueueSize != 1024 {
		t.Fatalf("got workers=%d queue=%d", cfg.DispatchWorkers, cfg.DispatchQueueSize)
	}
}
