// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"testing"

	"github.com/konih/kollect/internal/transport"
)

func TestPublisherForHTTPRequiresURL(t *testing.T) {
	resetPublisherCache()
	t.Cleanup(resetPublisherCache)

	_, err := publisherFor(transport.Config{
		Type: transport.TypeHTTP,
		HTTP: transport.HTTPConfig{URL: ""},
	})
	if err == nil {
		t.Fatal("expected error for empty hub URL")
	}
}

func TestPublisherForHTTPRequiresCluster(t *testing.T) {
	resetPublisherCache()
	t.Cleanup(resetPublisherCache)
	t.Setenv("KOLLECT_SPOKE_CLUSTER", "")

	_, err := publisherFor(transport.Config{
		Type: transport.TypeHTTP,
		HTTP: transport.HTTPConfig{URL: "https://hub.example/hub/v1alpha1/reports"},
	})
	if err == nil {
		t.Fatal("expected error for missing spoke cluster")
	}
}

func TestPublisherForInProcessCachesPublisher(t *testing.T) {
	resetPublisherCache()
	t.Cleanup(resetPublisherCache)

	cfg := transport.Config{Type: transport.TypeInProcess}
	first, err := publisherFor(cfg)
	if err != nil {
		t.Fatal(err)
	}

	second, err := publisherFor(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatal("expected cached in-process publisher")
	}
}
