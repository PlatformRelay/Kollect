// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package version

import "testing"

func TestString(t *testing.T) {
	t.Cleanup(func() {
		Version, Commit, Date = "dev", "unknown", "unknown"
	})

	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "defaults",
			version: "dev",
			commit:  "unknown",
			date:    "unknown",
			want:    "dev (commit unknown, built unknown)",
		},
		{
			name:    "release build",
			version: "v0.18.0",
			commit:  "a1b2c3d",
			date:    "2026-08-07T12:00:00Z",
			want:    "v0.18.0 (commit a1b2c3d, built 2026-08-07T12:00:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version, Commit, Date = tt.version, tt.commit, tt.date

			if got := String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
