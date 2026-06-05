// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"strconv"

	"github.com/konih/kollect/internal/metrics"
)

// recordTargetSnapshotMetrics emits Phase 4 domain series from a target snapshot.
// Until KollectProfile.spec.metrics exists (ADR-0033), numeric extracted attributes
// are summed per attribute name; object_count tracks row cardinality for the target.
func recordTargetSnapshotMetrics(profile, gvk string, items []Item) {
	metrics.RecordCustomResourceSeries(profile, gvk, "object_count", float64(len(items)))

	sums := make(map[string]float64)
	for _, item := range items {
		for name, val := range item.Attributes {
			if f, ok := numericAttribute(val); ok {
				sums[name] += f
			}
		}
	}

	for name, sum := range sums {
		metrics.RecordCustomResourceSeries(profile, gvk, name, sum)
	}
}

func numericAttribute(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()

		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)

		return f, err == nil
	default:
		return 0, false
	}
}
