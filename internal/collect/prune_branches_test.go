// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"reflect"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestPruneResource_nilObjectReturnsNil(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource}
	if got := PruneResource(nil, export, NewScrubber(nil)); got != nil {
		t.Fatalf("PruneResource(nil) = %v, want nil", got)
	}
}

// TestPruneResource_invalidPointersAreNoOps locks that malformed RFC 6901 pointers
// never mutate the object and never panic.
func TestPruneResource_invalidPointersAreNoOps(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:           kollectdevv1alpha1.ExportModeResource,
		Include:        kollectdevv1alpha1.ExportIncludeAll,
		DedupeIdentity: boolPtr(false),
		Prune: &kollectdevv1alpha1.PruneSpec{
			Defaults:     boolPtr(false),
			JSONPointers: []string{"", "   ", "/", "relative/path", "no-leading-slash"},
		},
	}

	got := PruneResource(sampleDeployment(), export, nil)
	want := sampleDeployment().Object

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invalid pointers mutated the object:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestPruneResource_sliceIndexPointers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pointer string
		check   func(t *testing.T, got map[string]any)
	}{
		{
			name:    "valid index nils element in place",
			pointer: "/status/conditions/0",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()

				conditions := got["status"].(map[string]any)["conditions"].([]any)
				if len(conditions) != 1 || conditions[0] != nil {
					t.Fatalf("conditions = %#v, want [nil] (element nilled, not removed)", conditions)
				}
			},
		},
		{
			name:    "out-of-range index is a no-op",
			pointer: "/status/conditions/9",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()

				conditions := got["status"].(map[string]any)["conditions"].([]any)
				if len(conditions) != 1 || conditions[0] == nil {
					t.Fatalf("conditions = %#v, want untouched element", conditions)
				}
			},
		},
		{
			name:    "non-numeric index is a no-op",
			pointer: "/status/conditions/first",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()

				conditions := got["status"].(map[string]any)["conditions"].([]any)
				if len(conditions) != 1 || conditions[0] == nil {
					t.Fatalf("conditions = %#v, want untouched element", conditions)
				}
			},
		},
		{
			name:    "negative index is a no-op",
			pointer: "/status/conditions/-1",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()

				conditions := got["status"].(map[string]any)["conditions"].([]any)
				if len(conditions) != 1 || conditions[0] == nil {
					t.Fatalf("conditions = %#v, want untouched element", conditions)
				}
			},
		},
		{
			name:    "recurse into slice element deletes leaf key",
			pointer: "/status/conditions/0/type",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()

				conditions := got["status"].(map[string]any)["conditions"].([]any)
				elem, ok := conditions[0].(map[string]any)
				if !ok {
					t.Fatalf("conditions[0] = %#v, want surviving map", conditions[0])
				}
				if _, present := elem["type"]; present {
					t.Fatalf("conditions[0].type still present: %#v", elem)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			export := &kollectdevv1alpha1.ExportSpec{
				Mode:    kollectdevv1alpha1.ExportModeResource,
				Include: kollectdevv1alpha1.ExportIncludeAll,
				Prune: &kollectdevv1alpha1.PruneSpec{
					Defaults:     boolPtr(false),
					JSONPointers: []string{tt.pointer},
				},
			}

			tt.check(t, PruneResource(sampleDeployment(), export, nil))
		})
	}
}

func TestRemoveByTokens_emptyTokensIsNoOp(t *testing.T) {
	t.Parallel()

	node := map[string]any{"keep": "me"}
	removeByTokens(node, nil)

	if node["keep"] != "me" {
		t.Fatalf("empty token list mutated node: %#v", node)
	}
}

// TestPruneResource_unsupportedJSONPathsAreNoOps locks the Phase-1 contract:
// wildcard and filter JSONPath expressions are deliberately skipped, untouched.
func TestPruneResource_unsupportedJSONPathsAreNoOps(t *testing.T) {
	t.Parallel()

	export := &kollectdevv1alpha1.ExportSpec{
		Mode:           kollectdevv1alpha1.ExportModeResource,
		Include:        kollectdevv1alpha1.ExportIncludeAll,
		DedupeIdentity: boolPtr(false),
		Prune: &kollectdevv1alpha1.PruneSpec{
			Defaults:  boolPtr(false),
			JSONPaths: []string{"$.metadata.labels[*]", "$.*", `$.status.conditions[?(@.type=="Available")]`},
		},
	}

	got := PruneResource(sampleDeployment(), export, nil)
	want := sampleDeployment().Object

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unsupported JSONPaths mutated the object:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestJSONPathTokens_rejectedShapes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"$.a[0",   // unterminated bracket
		"$.*",     // bare wildcard segment
		"$.a.*.b", // wildcard mid-path
	} {
		if tokens, ok := jsonPathTokens(path); ok {
			t.Fatalf("jsonPathTokens(%q) = %v ok=true, want rejection", path, tokens)
		}
	}
}
