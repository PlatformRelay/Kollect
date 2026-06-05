// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"net/http/httptest"
	"testing"

	"github.com/konih/kollect/internal/collect"
)

func TestFilterItemsByKindAndTarget(t *testing.T) {
	t.Parallel()

	items := []collect.Item{
		{TargetName: "deploys", Kind: "Deployment", Version: "v1", Name: "web"},
		{TargetName: "deploys", Kind: "Service", Version: "v1", Name: "web-svc"},
		{TargetName: "helm", Kind: "Deployment", Version: "v1", Name: "api"},
	}

	filtered := filterItems(items, ListFilter{Target: "deploys", Kind: "Deployment"})
	if len(filtered) != 1 || filtered[0].Name != "web" {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestPaginateItems(t *testing.T) {
	t.Parallel()

	items := make([]collect.Item, 5)
	for i := range items {
		items[i].Name = string(rune('a' + i))
	}

	page, meta := paginateItems(items, 2, 1)
	if len(page) != 2 || meta.Total != 5 || !meta.HasMore {
		t.Fatalf("page = %#v meta = %#v", page, meta)
	}
}

func TestItemMatchesFilterNamespaceBranches(t *testing.T) {
	t.Parallel()

	item := collect.Item{TargetNamespace: "team-a", Namespace: "apps", Name: "web"}
	if !itemMatchesFilter(item, ListFilter{Namespace: "team-a"}) {
		t.Fatal("expected target namespace match")
	}
	if !itemMatchesFilter(item, ListFilter{Namespace: "apps"}) {
		t.Fatal("expected resource namespace match")
	}
	if itemMatchesFilter(item, ListFilter{Namespace: "other"}) {
		t.Fatal("expected namespace miss")
	}
}

func TestPaginateItems_edgeCases(t *testing.T) {
	t.Parallel()

	if page, meta := paginateItems(nil, 10, 0); page != nil || meta != nil {
		t.Fatalf("nil items = %#v %#v", page, meta)
	}

	items := []collect.Item{{Name: "a"}, {Name: "b"}}
	page, meta := paginateItems(items, 0, 0)
	if len(page) != 2 || meta.Total != 2 || meta.HasMore {
		t.Fatalf("default limit page = %#v meta = %#v", page, meta)
	}

	page, meta = paginateItems(items, 1, 5)
	if len(page) != 0 || meta.Offset != 2 {
		t.Fatalf("offset past end = %#v meta = %#v", page, meta)
	}
}

func TestParseListFilter(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/v1alpha1/inventory?namespace=team-a&target=deploys&limit=10&offset=5", nil)
	filter := parseListFilter(req)
	if filter.Namespace != "team-a" || filter.Target != "deploys" || filter.Limit != 10 || filter.Offset != 5 {
		t.Fatalf("filter = %#v", filter)
	}
}
