// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// syncCollectedCountFields is the collected-count sync both target controllers share
// (TSP-1 / D3 — one contract, twice): the namespaced KollectTarget semantics applied to
// whichever status carries the fields. The count is written through even when it is zero so
// a measured zero stays distinguishable from "never computed" (the field stays nil until the
// first Ready observation), and collectedCountUpdatedAt marks when the number last
// *changed*, not when it was last checked — a steady count keeps its timestamp, and a
// Degraded target (which never reaches this helper) keeps its last known count. This is the
// whole point of PERF-FIX-05: the old prose-only count could not signal staleness.
//
// Returns the freshly derived number — callers restate it in the Ready message, so the
// prose and the stored count cannot disagree — and whether the fields moved and now need
// persisting. When they did not move, both fields are left exactly as they are.
func syncCollectedCountFields(
	countField **int64,
	updatedAtField **metav1.Time,
	collected int,
) (int64, bool) {
	next := int64(collected)
	if *countField != nil && **countField == next {
		return next, false
	}

	now := metav1.Now()
	*countField = &next
	*updatedAtField = &now

	return next, true
}
