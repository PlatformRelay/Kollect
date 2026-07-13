// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// newTestBackend builds a GitLab backend pointed at endpoint with the given auth.
func newTestBackend(t *testing.T, endpoint string, auth git.Auth) *Backend {
	t.Helper()

	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: endpoint,
	}, nil, auth)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}
	return b
}

func TestBackend_Capabilities(t *testing.T) {
	t.Parallel()

	b := newTestBackend(t, "https://gitlab.example.com/platform/inventory.git", git.Auth{Token: "tok"})

	got := b.Capabilities()
	if got != cap.SnapshotStore() {
		t.Fatalf("Capabilities() = %#v, want %#v", got, cap.SnapshotStore())
	}
	// GitLab is a whole-snapshot git store: no streaming, no server-side delete.
	if got.Stream {
		t.Fatal("Capabilities().Stream = true, want false (snapshot store)")
	}
	if got.SupportsDelete {
		t.Fatal("Capabilities().SupportsDelete = true, want false (snapshot store)")
	}
}

// TestBackend_Export_surfacesCloneFailure drives Export against a live TCP
// endpoint that immediately returns a non-git HTTP 500 to every request, so the
// go-git clone step fails and Export must surface that error rather than swallow
// it. This exercises the git-export branch and its error return without needing
// a real git-smart-HTTP remote.
func TestBackend_Export_surfacesCloneFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not a git server", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	b := newTestBackend(t, srv.URL+"/platform/inventory.git", git.Auth{Token: "tok"})

	err := b.Export(context.Background(), []byte(`{"items":[]}`), "inventory/team-a/rollup.json")
	if err == nil {
		t.Fatal("expected clone failure to surface from Export")
	}
}

// TestBackend_ExportFiles_surfacesCloneFailure is the ExportFiles analogue of the
// Export clone-failure test: a non-empty file set forces the git push branch,
// which must fail against a server that does not speak git.
func TestBackend_ExportFiles_surfacesCloneFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not a git server", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	b := newTestBackend(t, srv.URL+"/platform/inventory.git", git.Auth{Token: "tok"})

	files := []git.FileEntry{{Path: "inventory/team-a/rollup.json", Data: []byte(`{"items":[]}`)}}
	err := b.ExportFiles(context.Background(), files, false)
	if err == nil {
		t.Fatal("expected clone failure to surface from ExportFiles")
	}
}

// TestBackend_ExportFiles_empty asserts the no-files guard returns a specific
// error before any git work is attempted.
func TestBackend_ExportFiles_empty(t *testing.T) {
	t.Parallel()

	b := newTestBackend(t, "https://gitlab.example.com/platform/inventory.git", git.Auth{Token: "tok"})

	err := b.ExportFiles(context.Background(), nil, false)
	if err == nil {
		t.Fatal("expected error for empty file set")
	}
	if !strings.Contains(err.Error(), "no files to write") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRESTClient_setGitLabAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   string
		wantSet bool
		wantHdr string
	}{
		{name: "token present sets PRIVATE-TOKEN", token: "glpat-abc123", wantSet: true, wantHdr: "glpat-abc123"},
		{name: "empty token omits header", token: "", wantSet: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := &RESTClient{Token: tc.token}
			req, err := http.NewRequest(http.MethodGet, "https://gitlab.example.com/api/v4/projects", nil)
			if err != nil {
				t.Fatal(err)
			}
			c.setGitLabAuth(req)

			got := req.Header.Get("PRIVATE-TOKEN")
			if tc.wantSet {
				if got != tc.wantHdr {
					t.Fatalf("PRIVATE-TOKEN = %q, want %q", got, tc.wantHdr)
				}
				return
			}
			if got != "" {
				t.Fatalf("PRIVATE-TOKEN = %q, want unset", got)
			}
		})
	}
}

func TestAPIBaseURL_customHostAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("custom host and port", func(t *testing.T) {
		t.Parallel()

		got, err := APIBaseURL("https://gitlab.internal.corp:8443/group/sub/inventory.git")
		if err != nil {
			t.Fatalf("APIBaseURL: %v", err)
		}
		if got != "https://gitlab.internal.corp:8443/api/v4" {
			t.Fatalf("APIBaseURL = %q", got)
		}
	})

	t.Run("http scheme retained", func(t *testing.T) {
		t.Parallel()

		got, err := APIBaseURL("http://gitea.example.com/owner/repo.git")
		if err != nil {
			t.Fatalf("APIBaseURL: %v", err)
		}
		if got != "http://gitea.example.com/api/v4" {
			t.Fatalf("APIBaseURL = %q", got)
		}
	})

	t.Run("non-http scheme rejected", func(t *testing.T) {
		t.Parallel()

		if _, err := APIBaseURL("ssh://git@gitlab.example.com/group/project.git"); err == nil {
			t.Fatal("expected error for ssh scheme")
		}
	})

	t.Run("unparseable endpoint rejected", func(t *testing.T) {
		t.Parallel()

		if _, err := APIBaseURL("https://exa mple.com/x"); err == nil {
			t.Fatal("expected parse error for malformed endpoint")
		}
	})
}
