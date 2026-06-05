// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

func TestIngestHandleReportsMergesAuthenticatedReport(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset sufficient for auth unit test
	client.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authenticationv1.TokenReview)
		review.Status = authenticationv1.TokenReviewStatus{Authenticated: true}

		return true, review, nil
	})
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

		return true, review, nil
	})

	store := collect.NewStore()
	srv := &IngestServer{
		Enabled: true,
		Auth: IngestAuthConfig{
			Mode:   IngestAuthModeKubernetes,
			Client: client,
		},
		Merger: NewMerger(store),
	}

	mux := http.NewServeMux()
	mux.Handle("POST "+ingestReportsPath, srv.Auth.Middleware(http.HandlerFunc(srv.handleReports)))

	report := SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-a",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
		Items: []collect.Item{
			{
				TargetNamespace: "team-a",
				TargetName:      "t",
				Namespace:       "apps",
				Name:            "demo",
				UID:             "uid-1",
				Version:         "v1",
				Kind:            "Deployment",
			},
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if store.TotalCount() != 1 {
		t.Fatalf("hub count = %d, want 1", store.TotalCount())
	}
}

func TestIngestHandleReportsRejectsUnregisteredCluster(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	srv := &IngestServer{
		Enabled:           true,
		Auth:              IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:            NewMerger(store),
		AllowedClusters:   []string{"spoke-a"},
		AllowlistEnforced: true,
	}

	report := SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "rogue",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "rogue")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestIngestServerStartDisabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := (&IngestServer{Enabled: false}).Start(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestIngestHandleReportsMissingClusterHeader(t *testing.T) {
	t.Parallel()

	srv := &IngestServer{
		Enabled: true,
		Auth:    IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:  NewMerger(collect.NewStore()),
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestIngestHandleReportsInvalidJSON(t *testing.T) {
	t.Parallel()

	srv := &IngestServer{
		Enabled: true,
		Auth:    IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:  NewMerger(collect.NewStore()),
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader([]byte(`not-json`)))
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestIngestHandleReportsNilMerger(t *testing.T) {
	t.Parallel()

	srv := &IngestServer{Enabled: true}
	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, nil)
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

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

func TestIngestReportsPath(t *testing.T) {
	t.Parallel()

	if got := IngestReportsPath(); got != ingestReportsPath {
		t.Fatalf("path = %q, want %q", got, ingestReportsPath)
	}
}

func TestIngestServerStartServesReports(t *testing.T) {
	store := collect.NewStore()
	srv := &IngestServer{
		Enabled: true,
		Port:    freeTCPPort(t),
		Auth:    IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:  NewMerger(store),
	}

	report := SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-a",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
		Items: []collect.Item{
			{
				TargetNamespace: "team-a",
				TargetName:      "t",
				Namespace:       "apps",
				Name:            "demo",
				UID:             "uid-1",
				Version:         "v1",
				Kind:            "Deployment",
			},
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d%s", srv.Port, ingestReportsPath)
	deadline := time.Now().Add(2 * time.Second)
	var ready bool
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusAccepted {
				ready = true

				break
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("ingest server did not accept report")
	}
	if store.TotalCount() != 1 {
		t.Fatalf("store count = %d, want 1", store.TotalCount())
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start after shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ingest server did not stop after cancel")
	}
}

func TestIngestHandleReportsMarksRemoteClusterConnected(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rc := &kollectdevv1alpha1.KollectRemoteCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-a", Namespace: "platform"},
		Spec:       kollectdevv1alpha1.KollectRemoteClusterSpec{ClusterName: "spoke-a"},
	}
	statusClient := crfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(rc).WithObjects(rc).Build()

	store := collect.NewStore()
	srv := &IngestServer{
		Enabled:      true,
		Auth:         IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:       NewMerger(store),
		StatusClient: statusClient,
	}

	report := SpokeReport{
		APIVersion: "kollect.dev/v1alpha1",
		Cluster:    "spoke-a",
		InventoryRef: InventoryRef{
			Namespace: "team-a",
			Name:      "inv",
		},
		Items: []collect.Item{{
			TargetNamespace: "team-a",
			TargetName:      "t",
			Namespace:       "apps",
			Name:            "demo",
			UID:             "uid-1",
			Version:         "v1",
			Kind:            "Deployment",
		}},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got kollectdevv1alpha1.KollectRemoteCluster
	if err := statusClient.Get(context.Background(), client.ObjectKeyFromObject(rc), &got); err != nil {
		t.Fatal(err)
	}
	for i := range got.Status.Conditions {
		if got.Status.Conditions[i].Type == kollectdevv1alpha1.ConditionConnected &&
			got.Status.Conditions[i].Status == metav1.ConditionTrue {
			return
		}
	}
	t.Fatalf("Connected condition not set: %+v", got.Status.Conditions)
}

func TestIngestHandleReportsExportFailure(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	srv := &IngestServer{
		Enabled: true,
		Auth:    IngestAuthConfig{Mode: IngestAuthModeDisabled},
		Merger:  NewMerger(store),
		Exporter: &Exporter{
			Config: ExportConfig{
				ExportNamespace: "platform",
				SinkRefs:        []string{"demo"},
			},
		},
	}

	report := SpokeReport{
		APIVersion:   "kollect.dev/v1alpha1",
		Cluster:      "spoke-a",
		InventoryRef: InventoryRef{Namespace: "team-a", Name: "inv"},
		Items: []collect.Item{{
			TargetNamespace: "team-a",
			TargetName:      "t",
			Namespace:       "apps",
			Name:            "demo",
			UID:             "uid-1",
			Version:         "v1",
			Kind:            "Deployment",
		}},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, ingestReportsPath, bytes.NewReader(body))
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, "spoke-a")
	rec := httptest.NewRecorder()
	srv.handleReports(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
