// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIBaseURL(t *testing.T) {
	t.Parallel()

	got, err := APIBaseURL("https://gitlab.example.com/platform/inventory.git")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://gitlab.example.com/api/v4" {
		t.Fatalf("got %q", got)
	}
}

func TestRESTClient_EnsureOpenMergeRequest_create(t *testing.T) {
	t.Parallel()

	var createBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/merge_requests"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/merge_requests"):
			if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"iid":1,"web_url":"https://gitlab.example.com/mr/1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := &RESTClient{
		BaseURL:    srv.URL,
		Token:      "test-token",
		HTTPClient: srv.Client(),
	}

	err := client.EnsureOpenMergeRequest(
		context.Background(),
		ProjectRef{Path: "platform/inventory"},
		"kollect/default/team-inventory",
		"main",
		"kollect inventory export: default/team-inventory",
	)
	if err != nil {
		t.Fatalf("EnsureOpenMergeRequest: %v", err)
	}
	if createBody["source_branch"] != "kollect/default/team-inventory" {
		t.Fatalf("source_branch = %q", createBody["source_branch"])
	}
	if createBody["target_branch"] != "main" {
		t.Fatalf("target_branch = %q", createBody["target_branch"])
	}
}

func TestRESTClient_EnsureOpenMergeRequest_existing(t *testing.T) {
	t.Parallel()

	createCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/merge_requests"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"iid":7,"source_branch":"kollect/default/team-inventory","target_branch":"main"}]`))
		case r.Method == http.MethodPost:
			createCalls++
			http.Error(w, "should not create", http.StatusTeapot)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := &RESTClient{
		BaseURL:    srv.URL,
		Token:      "test-token",
		HTTPClient: srv.Client(),
	}

	err := client.EnsureOpenMergeRequest(
		context.Background(),
		ProjectRef{Path: "platform/inventory"},
		"kollect/default/team-inventory",
		"main",
		"title",
	)
	if err != nil {
		t.Fatalf("EnsureOpenMergeRequest: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("createCalls = %d, want 0", createCalls)
	}
}

func TestRESTClient_EnsureOpenMergeRequest_missingToken(t *testing.T) {
	t.Parallel()

	client := &RESTClient{BaseURL: "https://gitlab.example.com/api/v4"}
	err := client.EnsureOpenMergeRequest(
		context.Background(),
		ProjectRef{Path: "group/project"},
		"feature",
		"main",
		"title",
	)
	if err == nil {
		t.Fatal("expected token error")
	}
}
