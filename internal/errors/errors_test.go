// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package errors

import (
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestClassOfTypedErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		err   error
		class string
	}{
		{"transient", Transient(fmt.Errorf("timeout")), ClassTransient},
		{"terminal", Terminal(fmt.Errorf("bad sink")), ClassTerminal},
		{"forbidden", Forbidden(fmt.Errorf("sar denied")), ClassForbidden},
		{
			"not found",
			apierrors.NewNotFound(schema.GroupResource{Group: "kollect.dev", Resource: "sinks"}, "x"),
			ClassTerminal,
		},
		{"forbidden api", apierrors.NewForbidden(schema.GroupResource{}, "x", fmt.Errorf("no")), ClassForbidden},
		{"plain", fmt.Errorf("plain"), ClassTransient},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ClassOf(tc.err); got != tc.class {
				t.Fatalf("ClassOf() = %q, want %q", got, tc.class)
			}
		})
	}
}

func TestIsHelpers(t *testing.T) {
	t.Parallel()

	term := Terminal(fmt.Errorf("x"))
	if !IsTerminal(term) || IsTransient(term) {
		t.Fatal("expected terminal")
	}

	trans := Transient(fmt.Errorf("x"))
	if !IsTransient(trans) || IsTerminal(trans) {
		t.Fatal("expected transient")
	}
}

func TestUnwrap(t *testing.T) {
	t.Parallel()

	root := fmt.Errorf("root cause")
	wrapped := Transient(root)

	if !errors.Is(wrapped, ErrTransient) {
		t.Fatal("expected ErrTransient in chain")
	}

	if !errors.Is(errors.Unwrap(wrapped), root) {
		t.Fatal("expected root cause on Unwrap")
	}
}

func TestClassifyAPI(t *testing.T) {
	t.Parallel()

	nf := apierrors.NewNotFound(schema.GroupResource{}, "missing")
	if !IsTerminal(ClassifyAPI(nf)) {
		t.Fatal("expected terminal for NotFound")
	}
}
