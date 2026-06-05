// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"strings"
	"testing"
)

func TestResolveProjectRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "https with git suffix",
			input: "https://gitlab.example.com/platform/kollect-inventory.git",
			want:  "platform/kollect-inventory",
		},
		{
			name:  "http without suffix",
			input: "http://gitlab.example.com/group/project",
			want:  "group/project",
		},
		{
			name:    "ssh scheme rejected",
			input:   "ssh://git@gitlab.example.com/group/project.git",
			wantErr: true,
		},
		{
			name:    "missing project path",
			input:   "https://gitlab.example.com/",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ref, err := ResolveProjectRef(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveProjectRef: %v", err)
			}
			if ref.Path != tc.want {
				t.Fatalf("Path = %q, want %q", ref.Path, tc.want)
			}
		})
	}
}

func TestValidateMergeRequestConfig(t *testing.T) {
	t.Parallel()

	if err := ValidateMergeRequestConfig(MergeRequestConfig{}); err != nil {
		t.Fatalf("direct default: %v", err)
	}
	if err := ValidateMergeRequestConfig(MergeRequestConfig{Mode: MergeRequestModeDirectPush}); err != nil {
		t.Fatalf("direct explicit: %v", err)
	}
	if err := ValidateMergeRequestConfig(MergeRequestConfig{Mode: MergeRequestModeBranchMR}); err == nil {
		t.Fatal("expected error for missing targetBranch")
	}
	if err := ValidateMergeRequestConfig(MergeRequestConfig{
		Mode:         MergeRequestModeBranchMR,
		TargetBranch: "main",
	}); err != nil {
		t.Fatalf("valid MR config: %v", err)
	}
	if err := ValidateMergeRequestConfig(MergeRequestConfig{Mode: "invalid"}); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestBranchNameForExport(t *testing.T) {
	t.Parallel()

	got := BranchNameForExport("", "team-a", "inventory")
	want := "kollect/team-a/inventory"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	custom := BranchNameForExport("exports", "ns", "inv")
	if custom != "exports/ns/inv" {
		t.Fatalf("custom prefix = %q", custom)
	}
}

func TestMergeRequestTitle(t *testing.T) {
	t.Parallel()

	defaultTitle := MergeRequestTitle("", "default", "team-inventory")
	if !strings.Contains(defaultTitle, "default/team-inventory") {
		t.Fatalf("default title = %q", defaultTitle)
	}

	tpl := MergeRequestTitle("sync {namespace}/{name}", "platform", "rollup")
	if tpl != "sync platform/rollup" {
		t.Fatalf("template title = %q", tpl)
	}
}

func TestEnsureMergeRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{Endpoint: "https://gitlab.example.com/platform/inventory.git"}

	if err := EnsureMergeRequest(ctx, cfg, MergeRequestConfig{}, "feature"); err != nil {
		t.Fatalf("direct mode: %v", err)
	}

	err := EnsureMergeRequest(ctx, cfg, MergeRequestConfig{
		Mode:         MergeRequestModeBranchMR,
		TargetBranch: "main",
	}, "kollect/default/team")
	if err == nil {
		t.Fatal("expected not-implemented error")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := EnsureMergeRequest(ctx, cfg, MergeRequestConfig{Mode: MergeRequestModeBranchMR}, "x"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTestConnection_invalidURL(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), Config{Endpoint: "://bad"})
	if err == nil {
		t.Fatal("expected error for invalid endpoint")
	}
}
