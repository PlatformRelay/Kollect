// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"strings"
	"testing"
)

func TestValidateAttributePathCELPrefixRequired(t *testing.T) {
	t.Parallel()

	extractor, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		wantErr string
	}{
		{path: "object.metadata.name", wantErr: "cel:"},
		{path: "object['metadata']['name']", wantErr: "cel:"},
		{path: "$.metadata.name", wantErr: ""},
		{path: "cel:object.metadata.name", wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributePath(extractor, tt.path)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributePath(%q) = %v, want nil", tt.path, err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributePath(%q) = %v, want error containing %q", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestHasJSONPathFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "$.items[?(@.status.phase=='Running')].name", want: true},
		{path: "$.metadata.name", want: false},
		{path: "cel:object.status.phase", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			if got := HasJSONPathFilter(tt.path); got != tt.want {
				t.Fatalf("HasJSONPathFilter(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidateAttributePathEmpty(t *testing.T) {
	t.Parallel()

	extractor, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	if err := ValidateAttributePath(extractor, "  "); err == nil {
		t.Fatal("expected error for empty path")
	}
}
