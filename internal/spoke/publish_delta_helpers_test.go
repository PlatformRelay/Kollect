// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"testing"

	"github.com/konih/kollect/internal/collect"
)

func TestDeltaItemsAndSnapshotChanged(t *testing.T) {
	t.Parallel()

	prev := map[string]collect.Item{
		"uid-1": {UID: "uid-1", Name: "web", Version: "v1"},
		"uid-2": {UID: "uid-2", Name: "api", Version: "v1"},
	}
	current := []collect.Item{
		{UID: "uid-1", Name: "web", Version: "v1"},
		{UID: "uid-3", Name: "worker", Version: "v1"},
	}

	if !snapshotChanged(prev, current) {
		t.Fatal("expected changed snapshot when UIDs differ")
	}

	changed, removed := deltaItems(prev, current, true)
	if len(changed) != 1 || changed[0].UID != "uid-3" || len(removed) != 1 || removed[0] != "uid-2" {
		t.Fatalf("delta = changed %#v removed %#v", changed, removed)
	}

	first, removedFirst := deltaItems(nil, current, false)
	if len(first) != 2 || len(removedFirst) != 0 {
		t.Fatalf("first publish = %#v removed %#v", first, removedFirst)
	}

	prev["uid-1"] = collect.Item{UID: "uid-1", Name: "web", Version: "v2"}
	if !snapshotChanged(prev, current) {
		t.Fatal("expected change when item content differs")
	}
}
