// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/konih/kollect/internal/collect"
)

type upsertRow struct {
	values []any
}

func buildUpsertRows(
	invNS, invName, cluster string,
	items []collect.Item,
	exportedAt time.Time,
) ([]upsertRow, error) {
	rows := make([]upsertRow, len(items))
	for i, item := range items {
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("postgres export: marshal item: %w", err)
		}

		rows[i] = upsertRow{
			values: []any{
				invNS,
				invName,
				item.TargetName,
				item.UID,
				cluster,
				resourceNamespace(invNS, item),
				string(itemJSON),
				exportedAt,
			},
		}
	}

	return rows, nil
}

func resourceNamespace(invNS string, item collect.Item) string {
	if item.Namespace == "" {
		return invNS
	}

	return item.Namespace
}

type staleDeletePlan struct {
	deleteAll   bool
	targetNames []string
	sourceUIDs  []string
}

func buildStaleDeletePlan(items []collect.Item) staleDeletePlan {
	if len(items) == 0 {
		return staleDeletePlan{deleteAll: true}
	}

	targetNames := make([]string, len(items))
	sourceUIDs := make([]string, len(items))
	for i, item := range items {
		targetNames[i] = item.TargetName
		sourceUIDs[i] = item.UID
	}

	return staleDeletePlan{
		targetNames: targetNames,
		sourceUIDs:  sourceUIDs,
	}
}
