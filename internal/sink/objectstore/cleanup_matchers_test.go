// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package objectstore

import "testing"

func matchKeys(ms []KeyMatcher, key string) bool {
	for _, m := range ms {
		if len(key) >= len(m.Prefix) && key[:len(m.Prefix)] == m.Prefix && m.Rest.MatchString(key[len(m.Prefix):]) {
			return true
		}
	}

	return false
}

func TestCleanupMatchers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		match  []string
		reject []string
	}{
		{
			name:  "default json path matches itself and part siblings",
			path:  "inventory/team-a/apps.json",
			match: []string{"inventory/team-a/apps.json", "inventory/team-a/apps.part-0001-of-0003.json", "inventory/team-a/apps.part-0003-of-0003.json"},
			reject: []string{
				// sibling inventory sharing the base prefix must never be touched
				"inventory/team-a/apps-v2.json",
				"inventory/team-a/apps.json.bak",
				"inventory/team-a/apps.json.part-0001-of-0003.json", // suffix lands before ext, not after
				"inventory/team-a/appsx.json",
				"inventory/team-b/apps.json",
				"inventory/team-a/apps.part-1-of-3.json", // suffix is zero-padded 4 digits
				"inventory/team-a/apps.jsonn",
			},
		},
		{
			name:  "extensionless path",
			path:  "reports/team-a/daily",
			match: []string{"reports/team-a/daily", "reports/team-a/daily.part-0002-of-0004"},
			reject: []string{
				"reports/team-a/daily.json",
				"reports/team-a/daily-v2",
			},
		},
		{
			name:   "custom template path with name infix",
			path:   "snapshots/prod/team-a/report.json",
			match:  []string{"snapshots/prod/team-a/report.json", "snapshots/prod/team-a/report.part-0001-of-0002.json"},
			reject: []string{"snapshots/prod/team-a/report-2.json", "snapshots/prod/team-a/report.Json"},
		},
		{
			name: "parquet hive partition sweeps generations and part variants",
			path: "inventory/cluster=prod/ns=team-a/name=apps/generation=7.parquet",
			match: []string{
				"inventory/cluster=prod/ns=team-a/name=apps/generation=7.parquet",
				"inventory/cluster=prod/ns=team-a/name=apps/generation=3.parquet",
				"inventory/cluster=prod/ns=team-a/name=apps/generation=9.part-0001-of-0002.parquet",
			},
			reject: []string{
				"inventory/cluster=prod/ns=team-a/name=apps-v2/generation=7.parquet",
				"inventory/cluster=prod/ns=team-b/name=apps/generation=7.parquet",
				"inventory/cluster=prod/ns=team-a/name=apps/sub/generation=7.parquet",
				"inventory/cluster=prod/ns=team-a/name=apps/generation=notanumber.parquet",
			},
		},
		{
			name:  "empty path yields no matchers",
			path:  "  ",
			match: nil,
			reject: []string{
				"anything",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms := CleanupMatchers(tc.path)
			for _, key := range tc.match {
				if !matchKeys(ms, key) {
					t.Errorf("CleanupMatchers(%q): key %q should match", tc.path, key)
				}
			}
			for _, key := range tc.reject {
				if matchKeys(ms, key) {
					t.Errorf("CleanupMatchers(%q): key %q should NOT match", tc.path, key)
				}
			}
		})
	}
}
