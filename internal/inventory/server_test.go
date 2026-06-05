// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
)

func TestServerHandleInventory(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		Namespace:       "apps",
		Name:            "web",
		UID:             "uid-1",
		Version:         "v1",
		Kind:            "Deployment",
	})

	srv := &Server{Enabled: true, Store: store}
	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory?namespace=team-a", nil)
	rec := httptest.NewRecorder()
	srv.handleInventory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var summary Summary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.ItemCount != 1 || summary.Items[0].Name != "web" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestServerStartDisabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := (&Server{Enabled: false}).Start(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServerHandleWatch(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	srv := &Server{Enabled: true, Store: store}

	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory/watch?namespace=team-a", nil)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.handleWatch(rec, req)
		close(done)
	}()

	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		UID:             "uid-1",
		Namespace:       "apps",
		Name:            "web",
		Version:         "v1",
		Kind:            "Deployment",
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
	}

	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Fatal("expected watch event payload")
	}
}
