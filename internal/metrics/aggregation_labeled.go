// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package metrics

import (
	"sort"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const customResourceLabeledSeriesName = "kollect_custom_resource_labeled_series"

// DefaultMaxLabeledSeriesPerKey bounds the number of distinct label tuples a
// single (profile, gvk, series) may emit (EC-P2-09). KollectProfile.spec.metrics[].labels
// takes label values straight from collected attributes, which can be
// arbitrarily high-cardinality (UIDs, timestamps, free text); without a cap
// each distinct value becomes a permanent Prometheus series.
const DefaultMaxLabeledSeriesPerKey = 200

var maxLabeledSeriesPerKey = DefaultMaxLabeledSeriesPerKey

// SetMaxLabeledSeriesPerKeyGlobal configures the per (profile, gvk, series) cap
// on distinct label tuples. Values <= 0 are ignored.
func SetMaxLabeledSeriesPerKeyGlobal(n int) {
	if n <= 0 {
		return
	}

	maxLabeledSeriesPerKey = n
}

// MaxLabeledSeriesPerKeyGlobal returns the configured cap.
func MaxLabeledSeriesPerKeyGlobal() int {
	return maxLabeledSeriesPerKey
}

type labeledSeriesID struct {
	profile  string
	gvk      string
	series   string
	labelKey string
}

type labeledSeriesEntry struct {
	id     labeledSeriesID
	names  []string
	values []string
	value  float64
}

var (
	labeledSeriesMu sync.Mutex
	labeledSeries   = make(map[labeledSeriesID]labeledSeriesEntry)
	descCache       = make(map[string]*prometheus.Desc)
)

type customResourceLabeledCollector struct{}

func (customResourceLabeledCollector) Describe(ch chan<- *prometheus.Desc) {
	labeledSeriesMu.Lock()
	defer labeledSeriesMu.Unlock()

	seen := make(map[string]struct{})
	for _, entry := range labeledSeries {
		key := entry.descKey()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ch <- entry.desc()
	}
}

func (customResourceLabeledCollector) Collect(ch chan<- prometheus.Metric) {
	labeledSeriesMu.Lock()
	defer labeledSeriesMu.Unlock()

	for _, entry := range labeledSeries {
		ch <- prometheus.MustNewConstMetric(entry.desc(), prometheus.GaugeValue, entry.value, entry.values...)
	}
}

func (e labeledSeriesEntry) descKey() string {
	return strings.Join(append([]string{e.id.profile, e.id.gvk, e.id.series}, e.names...), "\x00")
}

func (e labeledSeriesEntry) desc() *prometheus.Desc {
	key := e.descKey()
	if d, ok := descCache[key]; ok {
		return d
	}

	labelNames := append([]string{"profile", "gvk", "series"}, e.names...)
	d := prometheus.NewDesc(
		customResourceLabeledSeriesName,
		"Domain metric series from collected custom resources with attribute label dimensions (ADR-0304).",
		labelNames,
		nil,
	)
	descCache[key] = d

	return d
}

// ResetCustomResourceLabeledSeries clears labeled entries for one profile/GVK before a snapshot refresh.
func ResetCustomResourceLabeledSeries(profile, gvk string) {
	labeledSeriesMu.Lock()
	defer labeledSeriesMu.Unlock()

	for id := range labeledSeries {
		if id.profile == profile && id.gvk == gvk {
			delete(labeledSeries, id)
		}
	}
}

// RecordCustomResourceLabeledSeries sets one labeled domain series value for a profile/GVK tuple.
func RecordCustomResourceLabeledSeries(profile, gvk, series string, labels map[string]string, value float64) {
	if len(labels) == 0 {
		RecordCustomResourceSeries(profile, gvk, series, value)

		return
	}

	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)

	values := make([]string, len(names))
	for i, name := range names {
		values[i] = labels[name]
	}

	id := labeledSeriesID{
		profile:  profile,
		gvk:      gvk,
		series:   series,
		labelKey: strings.Join(names, "\x00") + "\x00" + strings.Join(values, "\x00"),
	}

	labeledSeriesMu.Lock()
	defer labeledSeriesMu.Unlock()

	labeledSeries[id] = labeledSeriesEntry{
		id:     id,
		names:  names,
		values: values,
		value:  value,
	}
}

// CustomResourceLabeledSeriesValue returns one labeled series value (tests only).
func CustomResourceLabeledSeriesValue(profile, gvk, series string, labels map[string]string) (float64, bool) {
	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)

	values := make([]string, len(names))
	for i, name := range names {
		values[i] = labels[name]
	}

	id := labeledSeriesID{
		profile:  profile,
		gvk:      gvk,
		series:   series,
		labelKey: strings.Join(names, "\x00") + "\x00" + strings.Join(values, "\x00"),
	}

	labeledSeriesMu.Lock()
	defer labeledSeriesMu.Unlock()

	entry, ok := labeledSeries[id]

	return entry.value, ok
}
