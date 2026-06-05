// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"

	"github.com/konih/kollect/internal/sink/cap"
)

func TestCapabilityWrappers(t *testing.T) {
	t.Parallel()

	if SnapshotStoreCapabilities().Stream || SnapshotStoreCapabilities().ObjectStore {
		t.Fatal("snapshot store should not be stream or object store")
	}
	if !ObjectStoreSnapshotCapabilities().ObjectStore {
		t.Fatal("object store snapshot capabilities")
	}
	if !StreamEmitterCapabilities().Stream {
		t.Fatal("stream emitter capabilities")
	}
	if !RelationalStoreCapabilities().SupportsDelete {
		t.Fatal("relational store capabilities")
	}
}

func TestExportPayloadDelegation(t *testing.T) {
	t.Parallel()

	payload := []byte(`[{"name":"demo"}]`)
	exported, skip := ExportPayload(cap.SnapshotStore(), payload)
	if skip || string(exported) != string(payload) {
		t.Fatalf("export = %q skip=%v", exported, skip)
	}

	exported, skip = ExportPayload(cap.StreamEmitter(), nil)
	if !skip || exported != nil {
		t.Fatalf("stream skip = %q skip=%v", exported, skip)
	}
}
