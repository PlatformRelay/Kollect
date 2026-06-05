// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func freeTCPPort(t *testing.T) int32 {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	port := int32(ln.Addr().(*net.TCPAddr).Port) //nolint:gosec // ephemeral listener port fits int32
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	return port
}

func TestServerHandleInventoryFilters(t *testing.T) {
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
	store.Upsert(collect.Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		Namespace:       "apps",
		Name:            "cache",
		UID:             "uid-2",
		Version:         "v1",
		Kind:            "Deployment",
	})

	srv := &Server{Enabled: true, Store: store}
	req := httptest.NewRequest(
		http.MethodGet,
		"/v1alpha1/inventory?namespace=team-a&kind=Deployment&limit=1&offset=0",
		nil,
	)
	rec := httptest.NewRecorder()
	srv.handleInventory(rec, req)

	var summary Summary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.ItemCount != 1 || summary.Pagination == nil || summary.Pagination.Total != 2 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestServerBuildSummaryWithExportStatus(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	now := metav1.Now()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: []string{"git"}},
		Status: kollectdevv1alpha1.KollectInventoryStatus{
			LastExportTime: &now,
			Conditions: []metav1.Condition{{
				Type:   kollectdevv1alpha1.ConditionSynced,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(inv).Build()

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

	srv := &Server{
		Store:  store,
		Status: &ClientStatusReader{Client: cl},
	}
	summary := srv.buildSummary(context.Background(), ListFilter{
		Namespace: "team-a",
		Inventory: "platform",
		Kind:      "Deployment",
	})
	if summary.ItemCount != 1 || len(summary.ExportStatus) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Checksum == "" || summary.SchemaVersion == "" {
		t.Fatalf("checksum/schema missing: %#v", summary)
	}
}

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
	if summary.SchemaVersion != collect.ExportSchemaVersion {
		t.Fatalf("schemaVersion = %q", summary.SchemaVersion)
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

func TestServerHandleInventoryPathValues(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory/team-a/my-inv", nil)
	req.SetPathValue("namespace", "team-a")
	req.SetPathValue("name", "my-inv")
	rec := httptest.NewRecorder()
	srv.handleInventory(rec, req)

	var summary Summary
	if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}

	if summary.Namespace != "team-a" || summary.Inventory != "my-inv" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestServerHandleWatchNilStore(t *testing.T) {
	t.Parallel()

	srv := &Server{Enabled: true}
	req := httptest.NewRequest(http.MethodGet, "/v1alpha1/inventory/watch", nil)
	rec := httptest.NewRecorder()
	srv.handleWatch(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestServerStartServesInventory(t *testing.T) {
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

	port := freeTCPPort(t)
	srv := &Server{Enabled: true, Port: port, Store: store, Auth: &AuthConfig{Mode: AuthModeDisabled}}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/v1alpha1/inventory?namespace=team-a", port)
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get(url) //nolint:gosec,noctx // test probe against local ephemeral listener
		if err == nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET inventory: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var summary Summary
	if decodeErr := json.NewDecoder(resp.Body).Decode(&summary); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if summary.ItemCount != 1 {
		t.Fatalf("itemCount = %d", summary.ItemCount)
	}

	depURL := fmt.Sprintf("http://127.0.0.1:%d/inventory?namespace=team-a", port)
	depResp, err := http.Get(depURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatal(err)
	}
	_ = depResp.Body.Close()
	if depResp.StatusCode != http.StatusOK {
		t.Fatalf("deprecated route status = %d", depResp.StatusCode)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start after shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after cancel")
	}
}
