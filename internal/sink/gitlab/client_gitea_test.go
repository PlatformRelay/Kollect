// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// giteaFallbackServer routes GitLab v4 paths to 404 (forcing the Gitea fallback)
// and serves the Gitea v1 pulls API. It records the create payload it receives.
func giteaFallbackServer(t *testing.T, existing string, createStatus int, created *map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// GitLab API v4 — respond 404 so the client treats it as unsupported.
		case strings.Contains(r.URL.Path, "/merge_requests"):
			http.Error(w, "gitlab not found", http.StatusNotFound)
		// Gitea API v1 — list open pull requests.
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pulls"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(existing))
		// Gitea API v1 — create pull request.
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/pulls"):
			if r.Header.Get("Authorization") != "token gitea-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if created != nil {
				_ = json.NewDecoder(r.Body).Decode(created)
			}
			w.WriteHeader(createStatus)
			_, _ = w.Write([]byte(`{"number":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRESTClient_EnsureOpenMergeRequest_giteaFallbackCreate(t *testing.T) {
	t.Parallel()

	var created map[string]string
	srv := giteaFallbackServer(t, "[]", http.StatusCreated, &created)

	client := &RESTClient{
		BaseURL:    srv.URL,
		Token:      "gitea-token",
		HTTPClient: srv.Client(),
	}

	err := client.EnsureOpenMergeRequest(
		context.Background(),
		ProjectRef{Path: "owner/repo"},
		"kollect/default/inv",
		"main",
		"kollect inventory export",
	)
	if err != nil {
		t.Fatalf("EnsureOpenMergeRequest (gitea fallback): %v", err)
	}
	if created["head"] != "kollect/default/inv" {
		t.Fatalf("head = %q", created["head"])
	}
	if created["base"] != "main" {
		t.Fatalf("base = %q", created["base"])
	}
}

func TestRESTClient_EnsureOpenMergeRequest_giteaExisting(t *testing.T) {
	t.Parallel()

	existing := `[{"source_branch":"kollect/default/inv","target_branch":"main"}]`
	srv := giteaFallbackServer(t, existing, http.StatusTeapot, nil)

	client := &RESTClient{
		BaseURL:    srv.URL,
		Token:      "gitea-token",
		HTTPClient: srv.Client(),
	}

	// An existing open pull request with the matching target must short-circuit
	// before any create call (which the server would answer with 418).
	err := client.EnsureOpenMergeRequest(
		context.Background(),
		ProjectRef{Path: "owner/repo"},
		"kollect/default/inv",
		"main",
		"title",
	)
	if err != nil {
		t.Fatalf("EnsureOpenMergeRequest (gitea existing): %v", err)
	}
}

func TestRESTClient_EnsureOpenMergeRequest_giteaCreateError(t *testing.T) {
	t.Parallel()

	srv := giteaFallbackServer(t, "[]", http.StatusUnprocessableEntity, &map[string]string{})

	client := &RESTClient{
		BaseURL:    srv.URL,
		Token:      "gitea-token",
		HTTPClient: srv.Client(),
	}

	err := client.EnsureOpenMergeRequest(
		context.Background(),
		ProjectRef{Path: "owner/repo"},
		"kollect/default/inv",
		"main",
		"title",
	)
	if err == nil {
		t.Fatal("expected gitea create error on HTTP 422")
	}
	if !strings.Contains(err.Error(), "gitea create pull request") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRESTClient_setGiteaAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		token     string
		basicUser string
		wantAuth  string
		wantBasic bool
	}{
		{name: "basic", token: "tok", basicUser: "ci", wantBasic: true},
		{name: "token", token: "tok", wantAuth: "token tok"},
		{name: "anonymous", token: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &RESTClient{Token: tc.token, BasicUser: tc.basicUser}
			req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
			if err != nil {
				t.Fatal(err)
			}
			c.setGiteaAuth(req)
			if tc.wantBasic {
				user, pass, ok := req.BasicAuth()
				if !ok || user != tc.basicUser || pass != tc.token {
					t.Fatalf("basic auth = %q/%q ok=%v", user, pass, ok)
				}
				return
			}
			if got := req.Header.Get("Authorization"); got != tc.wantAuth {
				t.Fatalf("Authorization = %q, want %q", got, tc.wantAuth)
			}
		})
	}
}

func TestRESTClient_giteaBaseURL(t *testing.T) {
	t.Parallel()

	c := &RESTClient{BaseURL: "https://gitea.example.com/api/v4"}
	if got := c.giteaBaseURL(); got != "https://gitea.example.com/api/v1" {
		t.Fatalf("giteaBaseURL = %q", got)
	}
}

func TestIsGitLabAPIUnsupported(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"":                          false,
		"boom":                      false,
		"gitlab: HTTP 404: missing": true,
		"gitlab: HTTP 405: method":  true,
		"gitlab: HTTP 500: oops":    false,
	}
	for msg, want := range cases {
		var err error
		if msg != "" {
			err = errors.New(msg)
		}
		if got := isGitLabAPIUnsupported(err); got != want {
			t.Fatalf("isGitLabAPIUnsupported(%q) = %v, want %v", msg, got, want)
		}
	}
}

func TestTrimBody(t *testing.T) {
	t.Parallel()

	if got := trimBody([]byte("  hello  ")); got != "hello" {
		t.Fatalf("trimBody trim = %q", got)
	}
	long := strings.Repeat("x", 300)
	got := trimBody([]byte(long))
	if len(got) != 243 || !strings.HasSuffix(got, "...") {
		t.Fatalf("trimBody truncate len=%d suffix=%q", len(got), got[len(got)-3:])
	}
}

func TestProjectPath(t *testing.T) {
	t.Parallel()

	if got, err := projectPath(ProjectRef{ID: 42}); err != nil || got != "42" {
		t.Fatalf("projectPath id = %q, err=%v", got, err)
	}
	if got, err := projectPath(ProjectRef{Path: "group/sub/proj"}); err != nil || got != "group%2Fsub%2Fproj" {
		t.Fatalf("projectPath escape = %q, err=%v", got, err)
	}
	if _, err := projectPath(ProjectRef{}); err == nil {
		t.Fatal("expected error for empty project ref")
	}
}
