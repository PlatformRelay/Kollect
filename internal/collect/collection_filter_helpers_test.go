// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import "testing"

func TestUnionStrings(t *testing.T) {
	t.Parallel()

	if got := unionStrings(nil, []string{"a", "b"}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unionStrings(nil,b) = %#v", got)
	}
	if got := unionStrings([]string{"a", "b"}, nil); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unionStrings(a,nil) = %#v", got)
	}

	got := unionStrings([]string{"a", "", "b"}, []string{"b", "c", ""})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("unionStrings dedupe order = %#v, want [a b c]", got)
	}
}
