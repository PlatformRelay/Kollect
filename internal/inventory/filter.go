// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/konih/kollect/internal/collect"
)

const defaultPageLimit = 500

func parseListFilter(r *http.Request) ListFilter {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(q.Get("offset")))
	if offset < 0 {
		offset = 0
	}

	namespace := strings.TrimSpace(q.Get("namespace"))
	inventoryName := strings.TrimSpace(q.Get("inventory"))

	if ns := r.PathValue("namespace"); ns != "" {
		namespace = strings.TrimSpace(ns)
	}
	if name := r.PathValue("name"); name != "" {
		inventoryName = strings.TrimSpace(name)
	}

	return ListFilter{
		Namespace: namespace,
		Inventory: inventoryName,
		Target:    strings.TrimSpace(q.Get("target")),
		Group:     strings.TrimSpace(q.Get("group")),
		Version:   strings.TrimSpace(q.Get("version")),
		Kind:      strings.TrimSpace(q.Get("kind")),
		Name:      strings.TrimSpace(q.Get("name")),
		Limit:     limit,
		Offset:    offset,
	}
}

func filterItems(items []collect.Item, f ListFilter) []collect.Item {
	if len(items) == 0 {
		return nil
	}

	out := make([]collect.Item, 0, len(items))
	for _, item := range items {
		if !itemMatchesFilter(item, f) {
			continue
		}

		out = append(out, item)
	}

	return out
}

func itemMatchesFilter(item collect.Item, f ListFilter) bool {
	if f.Target != "" && item.TargetName != f.Target {
		return false
	}
	if f.Group != "" && item.Group != f.Group {
		return false
	}
	if f.Version != "" && item.Version != f.Version {
		return false
	}
	if f.Kind != "" && item.Kind != f.Kind {
		return false
	}
	if f.Name != "" && item.Name != f.Name {
		return false
	}
	if f.Namespace != "" && item.Namespace != f.Namespace && item.TargetNamespace != f.Namespace {
		return false
	}

	return true
}

func paginateItems(items []collect.Item, limit, offset int) ([]collect.Item, *Pagination) {
	total := len(items)
	if total == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = total
	}

	if offset > total {
		offset = total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	page := items[offset:end]

	return page, &Pagination{
		Limit:   limit,
		Offset:  offset,
		Total:   total,
		HasMore: end < total,
	}
}
