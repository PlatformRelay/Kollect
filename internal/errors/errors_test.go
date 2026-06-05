// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package errors

import (
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	fb := apierrors.NewForbidden(schema.GroupResource{}, "x", fmt.Errorf("denied"))
	if !IsForbidden(ClassifyAPI(fb)) {
		t.Fatal("expected forbidden")
	}
}

func TestIsForbiddenAndFormat(t *testing.T) {
	t.Parallel()

	if IsForbidden(Transient(fmt.Errorf("retry"))) {
		t.Fatal("transient must not be forbidden")
	}

	formatted := Format(ErrTerminal, "sink %q invalid", "demo")
	var ce *ClassError
	if !errors.As(formatted, &ce) || !errors.Is(ce.Class, ErrTerminal) {
		t.Fatalf("Format() = %T %v", formatted, formatted)
	}
}

func TestNilClassErrorHelpers(t *testing.T) {
	t.Parallel()

	var nilCE *ClassError
	if nilCE.Error() != "" || nilCE.Unwrap() != nil || nilCE.Is(ErrTransient) {
		t.Fatal("nil ClassError helpers should be inert")
	}

	if Transient(nil) != nil || Terminal(nil) != nil || Forbidden(nil) != nil {
		t.Fatal("nil wrappers should return nil")
	}

	if Format(nil, "plain %s", "msg").Error() != "plain msg" {
		t.Fatal("Format with nil class should return plain message")
	}
}

func TestClassifyAPIBranches(t *testing.T) {
	t.Parallel()

	invalid := apierrors.NewInvalid(schema.GroupKind{}, "x", field.ErrorList{})
	if !IsTerminal(ClassifyAPI(invalid)) {
		t.Fatal("expected terminal for Invalid")
	}

	badReq := apierrors.NewBadRequest("bad")
	if !IsTerminal(ClassifyAPI(badReq)) {
		t.Fatal("expected terminal for BadRequest")
	}

	if !IsTransient(ClassifyAPI(fmt.Errorf("timeout"))) {
		t.Fatal("expected transient for generic error")
	}

	if ClassOf(nil) != ClassTransient {
		t.Fatal("nil error should classify as transient")
	}

	var ce ClassError
	if ClassOf(&ce) != ClassTransient {
		t.Fatal("ClassError with nil Class should default to transient")
	}
}
