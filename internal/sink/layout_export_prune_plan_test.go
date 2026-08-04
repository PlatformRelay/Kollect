// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"reflect"
	"testing"
)

func TestPrunePlan_NewAddUnion(t *testing.T) {
	t.Parallel()

	plan := NewPrunePlan()
	if got := plan.Union(); len(got) != 0 {
		t.Fatalf("empty Union = %v, want nil/empty", got)
	}

	plan.Add([]string{"b.yaml", "a.yaml"})
	plan.Add([]string{"a.yaml", "c.yaml"})

	want := []string{"a.yaml", "b.yaml", "c.yaml"}
	if got := plan.Union(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Union = %v, want %v", got, want)
	}
}

func TestPrunePlan_NilReceiver(t *testing.T) {
	t.Parallel()

	var plan *PrunePlan
	plan.Add([]string{"x.yaml"})
	if got := plan.Union(); got != nil {
		t.Fatalf("nil receiver Union = %v, want nil", got)
	}
}

func TestPrunePlan_AddInitializesNilPaths(t *testing.T) {
	t.Parallel()

	plan := &PrunePlan{}
	plan.Add([]string{"solo.yaml"})
	want := []string{"solo.yaml"}
	if got := plan.Union(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Union after Add on nil paths = %v, want %v", got, want)
	}
}
