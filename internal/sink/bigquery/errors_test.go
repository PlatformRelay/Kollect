// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"errors"
	"testing"

	"cloud.google.com/go/bigquery"
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

func TestClassifyError_nilAndPlainError(t *testing.T) {
	t.Parallel()

	if got := classifyError(nil); got != nil {
		t.Fatalf("classifyError(nil) = %v, want nil", got)
	}

	// A plain, unclassifiable error is treated as transient (retryable) so a
	// flaky backend does not get stuck Degraded on an error we can't reason about.
	got := classifyError(errors.New("dial tcp: connection refused"))
	if class := kollecterrors.ClassOf(got); class != kollecterrors.ClassTransient {
		t.Fatalf("class = %s, want %s", class, kollecterrors.ClassTransient)
	}
}

func TestClassifyError_bigQueryReasonClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reason    string
		wantClass string
	}{
		{name: "notFound terminal", reason: "notFound", wantClass: kollecterrors.ClassTerminal},
		{name: "invalid terminal", reason: "invalid", wantClass: kollecterrors.ClassTerminal},
		{name: "rateLimitExceeded transient", reason: "rateLimitExceeded", wantClass: kollecterrors.ClassTransient},
		{name: "unknown reason transient", reason: "somethingElse", wantClass: kollecterrors.ClassTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// *bigquery.Error (not *googleapi.Error) exercises the bigquery
			// reason branch of collectReasons/isTerminal.
			err := &bigquery.Error{Reason: tt.reason, Message: "boom"}
			if class := kollecterrors.ClassOf(classifyError(err)); class != tt.wantClass {
				t.Fatalf("class = %s, want %s (reason=%s)", class, tt.wantClass, tt.reason)
			}
		})
	}
}

func TestIsDuplicateCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "409 duplicate is duplicate",
			err:  &googleapi.Error{Code: 409, Errors: []googleapi.ErrorItem{{Reason: "duplicate"}}},
			want: true,
		},
		{
			name: "409 without duplicate reason is not",
			err:  &googleapi.Error{Code: 409, Errors: []googleapi.ErrorItem{{Reason: "rateLimitExceeded"}}},
			want: false,
		},
		{
			name: "non-409 googleapi is not",
			err:  &googleapi.Error{Code: 400, Errors: []googleapi.ErrorItem{{Reason: "duplicate"}}},
			want: false,
		},
		{name: "nil is not", err: nil, want: false},
		{name: "plain error is not", err: errors.New("nope"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isDuplicateCreate(tt.err); got != tt.want {
				t.Fatalf("isDuplicateCreate(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestReasonsFromGoogleAPI_nilAndEmpty(t *testing.T) {
	t.Parallel()

	if got := reasonsFromGoogleAPI(nil); got != nil {
		t.Fatalf("reasonsFromGoogleAPI(nil) = %v, want nil", got)
	}

	if got := reasonsFromGoogleAPI(&googleapi.Error{Code: 500}); got != nil {
		t.Fatalf("reasonsFromGoogleAPI(no items) = %v, want nil", got)
	}

	// Blank reasons are skipped, real ones lower-cased and trimmed.
	got := reasonsFromGoogleAPI(&googleapi.Error{Errors: []googleapi.ErrorItem{
		{Reason: "  "},
		{Reason: "BackendError"},
	}})
	if len(got) != 1 || got[0] != "backenderror" {
		t.Fatalf("reasonsFromGoogleAPI = %v, want [backenderror]", got)
	}
}
