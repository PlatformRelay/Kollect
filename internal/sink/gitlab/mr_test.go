// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/konih/kollect/internal/sink/git"
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

	if err := EnsureMergeRequest(ctx, cfg, MergeRequestConfig{}, "feature", "default", "team", ""); err != nil {
		t.Fatalf("direct mode: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v4/projects/") {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte("[]"))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"iid":1}`))
	}))
	t.Cleanup(srv.Close)

	mrCfg := MergeRequestConfig{
		Mode:         MergeRequestModeBranchMR,
		TargetBranch: "main",
	}
	gitEndpoint := srv.URL + "/platform/inventory.git"
	err := EnsureMergeRequest(ctx, Config{Endpoint: gitEndpoint}, mrCfg,
		"kollect/default/team", "default", "team", "tok")
	if err != nil {
		t.Fatalf("MR mode with token: %v", err)
	}

	err = EnsureMergeRequest(ctx, cfg, mrCfg, "kollect/default/team", "default", "team", "")
	if err == nil {
		t.Fatal("expected token error")
	}

	err = EnsureMergeRequest(ctx, cfg, MergeRequestConfig{Mode: MergeRequestModeBranchMR},
		"x", "ns", "n", "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTestConnection_invalidURL(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), Config{Endpoint: "://bad"}, git.Auth{})
	if err == nil {
		t.Fatal("expected error for invalid endpoint")
	}
}
