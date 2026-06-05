// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"
)

func TestValidateProfileRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{name: "valid", ref: "deployment-images", wantErr: false},
		{name: "empty", ref: "", wantErr: true},
		{name: "cross namespace ref", ref: "other-ns/profile", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := validateProfileRef(tt.ref, nil)
			if (len(errs) > 0) != tt.wantErr {
				t.Fatalf("validateProfileRef(%q) errs=%v wantErr=%v", tt.ref, errs, tt.wantErr)
			}
		})
	}
}
