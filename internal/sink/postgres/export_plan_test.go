// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"math"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

func TestBuildUpsertRows_UsesScopeAndFallbackNamespaces(t *testing.T) {
	t.Parallel()

	exportedAt := time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)
	rows, err := buildUpsertRows("team-a", "apps", "prod-a", []collect.Item{
		{
			TargetName: "deployments",
			UID:        "uid-1",
			Namespace:  "",
			Name:       "api",
			Attributes: map[string]any{"replicas": 2},
		},
		{
			TargetName: "pods",
			UID:        "uid-2",
			Namespace:  "tenant-b",
			Name:       "worker",
			Attributes: map[string]any{"ready": true},
		},
	}, exportedAt)
	if err != nil {
		t.Fatalf("buildUpsertRows: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}
	if got := rows[0].values[5]; got != "team-a" {
		t.Fatalf("row[0] resource namespace = %v, want team-a fallback", got)
	}
	if got := rows[1].values[5]; got != "tenant-b" {
		t.Fatalf("row[1] resource namespace = %v, want tenant-b", got)
	}
	if got := rows[0].values[7]; got != exportedAt {
		t.Fatalf("row[0] exportedAt = %v, want %v", got, exportedAt)
	}
}

func TestBuildUpsertRows_ReportsMarshalErrors(t *testing.T) {
	t.Parallel()

	_, err := buildUpsertRows("team-a", "apps", "prod-a", []collect.Item{
		{
			TargetName: "deployments",
			UID:        "uid-1",
			Attributes: map[string]any{"ratio": math.NaN()},
		},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("expected marshal error for NaN payload")
	}
}

func TestBuildStaleDeletePlan(t *testing.T) {
	t.Parallel()

	plan := buildStaleDeletePlan(nil)
	if !plan.deleteAll {
		t.Fatal("deleteAll = false, want true for empty snapshot")
	}
	if len(plan.targetNames) != 0 || len(plan.sourceUIDs) != 0 {
		t.Fatalf("empty plan keys = %#v", plan)
	}

	plan = buildStaleDeletePlan([]collect.Item{
		{TargetName: "deployments", UID: "uid-1"},
		{TargetName: "pods", UID: "uid-2"},
	})
	if plan.deleteAll {
		t.Fatal("deleteAll = true, want false for non-empty snapshot")
	}
	if len(plan.targetNames) != 2 || len(plan.sourceUIDs) != 2 {
		t.Fatalf("plan keys lens = %d/%d, want 2/2", len(plan.targetNames), len(plan.sourceUIDs))
	}
	if plan.targetNames[0] != "deployments" || plan.sourceUIDs[1] != "uid-2" {
		t.Fatalf("unexpected plan keys: %#v", plan)
	}
}
