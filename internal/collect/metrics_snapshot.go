// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/konih/kollect/internal/metrics"
)

// recordTargetSnapshotMetrics emits Phase 4 domain series from a target snapshot.
// When metricPaths is empty, numeric extracted attributes are summed per attribute name.
// When KollectProfile.spec.metrics is set (ADR-0304), only configured series are emitted.
func recordTargetSnapshotMetrics(profile, gvk string, items []Item, metricPaths []metrics.MetricPathSpec) {
	metrics.RecordCustomResourceSeries(profile, gvk, "object_count", float64(len(items)))
	metrics.ResetCustomResourceLabeledSeries(profile, gvk)

	if len(metricPaths) == 0 {
		recordAutoSummedMetrics(profile, gvk, items)

		return
	}

	for _, spec := range metricPaths {
		if len(spec.Labels) == 0 {
			metrics.RecordCustomResourceSeries(profile, gvk, spec.Name, sumAttribute(items, spec.Path))

			continue
		}

		recordLabeledMetricSeries(profile, gvk, spec, items)
	}
}

func recordLabeledMetricSeries(profile, gvk string, spec metrics.MetricPathSpec, items []Item) {
	grouped := make(map[string]struct {
		labels map[string]string
		sum    float64
	})

	for _, item := range items {
		val, ok := numericAttribute(item.Attributes[spec.Path])
		if !ok {
			continue
		}

		labels := metricLabelValues(item, spec.Labels)
		key := metricLabelTupleKey(labels)
		entry := grouped[key]
		if entry.labels == nil {
			entry.labels = labels
		}

		entry.sum += val
		grouped[key] = entry
	}

	// EC-P2-09: cap distinct label tuples deterministically. The full tuple set
	// for this series is known here (unlike inside RecordCustomResourceLabeledSeries,
	// which only ever sees one tuple at a time), so a stable sort lets the same
	// tuples survive every snapshot refresh instead of an arbitrary random N from
	// Go map iteration order.
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if limit := metrics.MaxLabeledSeriesPerKeyGlobal(); len(keys) > limit {
		metrics.LabeledSeriesCardinalityCappedTotal.WithLabelValues(profile, gvk, spec.Name).
			Add(float64(len(keys) - limit))
		keys = keys[:limit]
	}

	for _, key := range keys {
		entry := grouped[key]
		metrics.RecordCustomResourceLabeledSeries(profile, gvk, spec.Name, entry.labels, entry.sum)
	}
}

func metricLabelValues(item Item, keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = attributeLabelValue(item.Attributes[key])
	}

	return out
}

func attributeLabelValue(v any) string {
	if v == nil {
		return ""
	}

	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprintf("%v", v)
	}
}

func metricLabelTupleKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte('\x00')
	}

	return b.String()
}

func recordAutoSummedMetrics(profile, gvk string, items []Item) {
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

func sumAttribute(items []Item, attribute string) float64 {
	var sum float64

	for _, item := range items {
		if f, ok := numericAttribute(item.Attributes[attribute]); ok {
			sum += f
		}
	}

	return sum
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
