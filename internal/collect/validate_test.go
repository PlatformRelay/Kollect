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
		path       string
		wantErr    bool
		wantCELMsg bool
	}{
		{path: "object.metadata.name", wantErr: true, wantCELMsg: true},
		{path: "object['metadata']['name']", wantErr: true, wantCELMsg: true},
		{path: "$.metadata.name", wantErr: false},
		{path: "cel:object.metadata.name", wantErr: false},
		{path: "helm:release.chartVersion", wantErr: false},
		{path: "helm:release.manifest", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributePath(extractor, tt.path)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateAttributePath(%q) = %v, want nil", tt.path, err)
				}

				return
			}

			if err == nil {
				t.Fatalf("ValidateAttributePath(%q) = nil, want error", tt.path)
			}
			if tt.wantCELMsg && !strings.HasPrefix(err.Error(), "CEL expressions must use") {
				t.Fatalf("ValidateAttributePath(%q) = %v, want CEL prefix error", tt.path, err)
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
