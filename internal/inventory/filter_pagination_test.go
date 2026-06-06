// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"testing"

	"github.com/konih/kollect/internal/collect"
)

// EC-P2-10: pagination applies default limit and caps excessive requests.
func TestPaginateItems_maxLimitCap(t *testing.T) {
	t.Parallel()

	items := make([]collect.Item, 6000)
	for i := range items {
		items[i] = collect.Item{UID: "uid"}
	}

	page, meta := paginateItems(items, 10000, 0)
	if meta.Limit != maxPageLimit {
		t.Fatalf("limit = %d, want %d", meta.Limit, maxPageLimit)
	}
	if len(page) != maxPageLimit {
		t.Fatalf("page len = %d, want %d", len(page), maxPageLimit)
	}
}

func TestPaginateItems_defaultLimitWhenZeroOrNegative(t *testing.T) {
	t.Parallel()

	items := make([]collect.Item, 600)
	for i := range items {
		items[i] = collect.Item{UID: "uid"}
	}

	page, meta := paginateItems(items, 0, 0)
	if meta.Limit != defaultPageLimit {
		t.Fatalf("limit = %d, want default %d", meta.Limit, defaultPageLimit)
	}
	if len(page) != defaultPageLimit {
		t.Fatalf("page len = %d, want %d", len(page), defaultPageLimit)
	}

	pageNeg, metaNeg := paginateItems(items, -5, 0)
	if metaNeg.Limit != defaultPageLimit || len(pageNeg) != defaultPageLimit {
		t.Fatalf("negative limit: meta=%+v len=%d", metaNeg, len(pageNeg))
	}
}

func TestPaginateItems_offsetBeyondTotal(t *testing.T) {
	t.Parallel()

	items := []collect.Item{{UID: "a"}, {UID: "b"}}
	page, meta := paginateItems(items, 10, 100)
	if meta != nil && meta.Total != 2 {
		t.Fatalf("total = %d", meta.Total)
	}
	if len(page) != 0 {
		t.Fatalf("page len = %d, want 0", len(page))
	}
}
