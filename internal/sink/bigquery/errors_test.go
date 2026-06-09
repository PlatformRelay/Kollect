// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"testing"

	"google.golang.org/api/googleapi"

	kollecterrors "github.com/konih/kollect/internal/errors"
)

func TestClassifyError_terminalAndTransient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantClass string
	}{
		{
			name: "invalid query terminal",
			err: &googleapi.Error{
				Code:   400,
				Errors: []googleapi.ErrorItem{{Reason: "invalidQuery"}},
			},
			wantClass: kollecterrors.ClassTerminal,
		},
		{
			name: "access denied terminal",
			err: &googleapi.Error{
				Code:   403,
				Errors: []googleapi.ErrorItem{{Reason: "accessDenied"}},
			},
			wantClass: kollecterrors.ClassTerminal,
		},
		{
			name: "quota exceeded transient",
			err: &googleapi.Error{
				Code:   429,
				Errors: []googleapi.ErrorItem{{Reason: "quotaExceeded"}},
			},
			wantClass: kollecterrors.ClassTransient,
		},
		{
			name: "backend error transient",
			err: &googleapi.Error{
				Code:   500,
				Errors: []googleapi.ErrorItem{{Reason: "backendError"}},
			},
			wantClass: kollecterrors.ClassTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyError(tt.err)
			if class := kollecterrors.ClassOf(got); class != tt.wantClass {
				t.Fatalf("class = %s, want %s (err=%v)", class, tt.wantClass, got)
			}
		})
	}
}

func TestIsDuplicateCreate(t *testing.T) {
	t.Parallel()

	err := &googleapi.Error{
		Code:   409,
		Errors: []googleapi.ErrorItem{{Reason: "duplicate"}},
	}
	if !isDuplicateCreate(err) {
		t.Fatal("expected duplicate create classification")
	}
}
