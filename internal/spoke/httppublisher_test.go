// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPPublisherRequiresURL(t *testing.T) {
	t.Parallel()

	var pub *HTTPPublisher
	if err := pub.Publish(context.Background(), "", nil); err == nil {
		t.Fatal("expected error for nil publisher")
	}

	if err := (&HTTPPublisher{}).Publish(context.Background(), "", nil); err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestHTTPPublisherPublishSuccess(t *testing.T) {
	t.Setenv(envSpokeToken, "test-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	pub := NewHTTPPublisher(srv.URL, "spoke-a")
	if err := pub.Publish(context.Background(), "", []byte(`{}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func TestHTTPPublisherPublishErrorStatus(t *testing.T) {
	t.Setenv(envSpokeToken, "test-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	pub := &HTTPPublisher{URL: srv.URL, Cluster: "spoke-a", Client: srv.Client()}
	err := pub.Publish(context.Background(), "", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestHTTPPublisherPublishErrorStatusEmptyBody(t *testing.T) {
	t.Setenv(envSpokeToken, "test-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "")
	}))
	defer srv.Close()

	pub := &HTTPPublisher{URL: srv.URL, Cluster: "spoke-a", Client: srv.Client()}
	if err := pub.Publish(context.Background(), "", []byte(`{}`)); err == nil {
		t.Fatal("expected error for 500 without body")
	}
}
