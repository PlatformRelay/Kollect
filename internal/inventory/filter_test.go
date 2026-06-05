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

func TestParseListFilter(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/v1alpha1/inventory?namespace=team-a&target=deploys&limit=10&offset=5", nil)
	filter := parseListFilter(req)
	if filter.Namespace != "team-a" || filter.Target != "deploys" || filter.Limit != 10 || filter.Offset != 5 {
		t.Fatalf("filter = %#v", filter)
	}
}
