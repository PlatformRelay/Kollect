// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

import (
	"reflect"
	"testing"
)

func TestCacheOptionsForWatchNamespaces_emptyWatchesAll(t *testing.T) {
	t.Parallel()

	opts := CacheOptionsForWatchNamespaces(nil)
	if opts.DefaultNamespaces != nil {
		t.Fatalf("DefaultNamespaces = %#v, want nil", opts.DefaultNamespaces)
	}

	opts = CacheOptionsForWatchNamespaces([]string{"", "  "})
	if opts.DefaultNamespaces != nil {
		t.Fatalf("DefaultNamespaces = %#v, want nil for blank entries", opts.DefaultNamespaces)
	}
}

func TestCacheOptionsForWatchNamespaces_scopedNamespaces(t *testing.T) {
	t.Parallel()

	opts := CacheOptionsForWatchNamespaces([]string{" team-a ", "team-b", "team-a"})
	want := []string{"team-a", "team-b"}

	if len(opts.DefaultNamespaces) != len(want) {
		t.Fatalf("DefaultNamespaces len = %d, want %d", len(opts.DefaultNamespaces), len(want))
	}

	for _, ns := range want {
		if _, ok := opts.DefaultNamespaces[ns]; !ok {
			t.Fatalf("missing namespace %q in DefaultNamespaces", ns)
		}
	}
}

func TestParseWatchNamespaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "single", raw: "team-a", want: []string{"team-a"}},
		{name: "comma separated", raw: "team-a,team-b", want: []string{"team-a", "team-b"}},
		{name: "dedupe and trim", raw: " team-a ,team-b,team-a ", want: []string{"team-a", "team-b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseWatchNamespaces(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseWatchNamespaces(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
